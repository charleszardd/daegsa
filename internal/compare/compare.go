package compare

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/report"
)

const MaxReportFileBytes int64 = 10 * 1024 * 1024

var ErrInvalidReport = errors.New("invalid comparison report")

type Delta struct {
	Name                string
	Baseline            float64
	Candidate           float64
	Absolute            float64
	Percentage          float64
	PercentageAvailable bool
}

type ThresholdChange struct{ Expression, Change string }

type Result struct {
	Warnings         []string
	Deltas           []Delta
	ThresholdChanges []ThresholdChange
}

func LoadReport(path string) (*report.Report, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: open %q: %v", ErrInvalidReport, path, err)
	}
	defer file.Close()
	limited := io.LimitReader(file, MaxReportFileBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("%w: read %q: %v", ErrInvalidReport, path, err)
	}
	if int64(len(data)) > MaxReportFileBytes {
		return nil, fmt.Errorf("%w: %q exceeds %d bytes", ErrInvalidReport, path, MaxReportFileBytes)
	}
	var loaded report.Report
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&loaded); err != nil {
		return nil, fmt.Errorf("%w: malformed JSON in %q: %v", ErrInvalidReport, path, err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, fmt.Errorf("%w: trailing JSON data in %q", ErrInvalidReport, path)
	}
	if loaded.ReportSchemaVersion != 1 && loaded.ReportSchemaVersion != 2 {
		return nil, fmt.Errorf("%w: unsupported report_schema_version %d", ErrInvalidReport, loaded.ReportSchemaVersion)
	}
	if loaded.Incomplete {
		return nil, fmt.Errorf("%w: incomplete reports cannot be compared", ErrInvalidReport)
	}
	if loaded.WorkloadModel == "" {
		return nil, fmt.Errorf("%w: workload_model is required", ErrInvalidReport)
	}
	return &loaded, nil
}

func Compare(baseline, candidate *report.Report) (*Result, error) {
	if baseline == nil || candidate == nil {
		return nil, fmt.Errorf("%w: both reports are required", ErrInvalidReport)
	}
	result := &Result{}
	if baseline.ConfigFingerprint != candidate.ConfigFingerprint {
		result.Warnings = append(result.Warnings, "configuration fingerprints differ")
	}
	if baseline.WorkloadModel != candidate.WorkloadModel {
		result.Warnings = append(result.Warnings, "workload models differ")
	}
	if !compatibleSegments(baseline, candidate) {
		result.Warnings = append(result.Warnings, "compiled profile segments differ")
	}
	b := comparisonSummary(baseline)
	c := comparisonSummary(candidate)
	values := []struct {
		name                string
		baseline, candidate float64
	}{
		{"p50_ms", b.Latency.AllCompleted.P50MS, c.Latency.AllCompleted.P50MS},
		{"p90_ms", b.Latency.AllCompleted.P90MS, c.Latency.AllCompleted.P90MS},
		{"p95_ms", b.Latency.AllCompleted.P95MS, c.Latency.AllCompleted.P95MS},
		{"p99_ms", b.Latency.AllCompleted.P99MS, c.Latency.AllCompleted.P99MS},
		{"max_ms", b.Latency.AllCompleted.MaxMS, c.Latency.AllCompleted.MaxMS},
		{"achieved_start_rate", b.AchievedStartRPS, c.AchievedStartRPS},
		{"completed_throughput", b.CompletedThroughput, c.CompletedThroughput},
		{"error_rate", b.ErrorRate, c.ErrorRate},
		{"rate_limited_rate", b.RateLimitedRate, c.RateLimitedRate},
		{"dropped", float64(b.RequestCounts.Dropped), float64(c.RequestCounts.Dropped)},
		{"observed_429", float64(b.RateLimits.Observed429Count), float64(c.RateLimits.Observed429Count)},
	}
	for _, value := range values {
		result.Deltas = append(result.Deltas, newDelta(value.name, value.baseline, value.candidate))
	}
	if compatibleSegments(baseline, candidate) && len(baseline.Segments) == len(candidate.Segments) {
		for index := range baseline.Segments {
			name := "segment[" + baseline.Segments[index].Segment.Name + "].p95_ms"
			result.Deltas = append(result.Deltas, newDelta(name, baseline.Segments[index].Metrics.Latency.AllCompleted.P95MS, candidate.Segments[index].Metrics.Latency.AllCompleted.P95MS))
		}
	}
	result.ThresholdChanges = compareThresholds(baseline, candidate)
	return result, nil
}

func newDelta(name string, baseline, candidate float64) Delta {
	delta := Delta{Name: name, Baseline: baseline, Candidate: candidate, Absolute: candidate - baseline}
	if baseline != 0 {
		delta.PercentageAvailable = true
		delta.Percentage = delta.Absolute / math.Abs(baseline) * 100
	}
	return delta
}

func comparisonSummary(value *report.Report) report.MetricsSummary {
	if value.MeasuredSummary != nil {
		return *value.MeasuredSummary
	}
	durationSeconds := float64(value.DurationMS) / 1000
	summary := report.MetricsSummary{RequestCounts: value.RequestCounts, Outcomes: value.Outcomes, StatusCodes: value.StatusCodes, Latency: value.Latency, RateLimits: value.RateLimits}
	if durationSeconds > 0 {
		summary.AchievedStartRPS = float64(value.RequestCounts.Started) / durationSeconds
		summary.CompletedThroughput = float64(value.RequestCounts.Completed) / durationSeconds
	}
	if value.RequestCounts.Completed > 0 {
		success := value.Outcomes["success"]
		summary.ErrorRate = float64(value.RequestCounts.Completed-success) / float64(value.RequestCounts.Completed) * 100
		summary.RateLimitedRate = float64(value.RateLimits.Observed429Count) / float64(value.RequestCounts.Completed) * 100
	}
	return summary
}

func compatibleSegments(baseline, candidate *report.Report) bool {
	if len(baseline.CompiledSegments) == 0 && len(candidate.CompiledSegments) == 0 {
		return true
	}
	if len(baseline.CompiledSegments) != len(candidate.CompiledSegments) {
		return false
	}
	for index := range baseline.CompiledSegments {
		left, right := baseline.CompiledSegments[index], candidate.CompiledSegments[index]
		if left.Name != right.Name || left.Stage != right.Stage || left.TargetRPS != right.TargetRPS || left.DurationMS != right.DurationMS {
			return false
		}
	}
	return true
}

func compareThresholds(baseline, candidate *report.Report) []ThresholdChange {
	left := make(map[string]report.ThresholdResult, len(baseline.Thresholds))
	right := make(map[string]report.ThresholdResult, len(candidate.Thresholds))
	for _, threshold := range baseline.Thresholds {
		left[threshold.Expression] = threshold
	}
	for _, threshold := range candidate.Thresholds {
		right[threshold.Expression] = threshold
	}
	changes := make([]ThresholdChange, 0)
	for expression, prior := range left {
		current, exists := right[expression]
		if !exists {
			changes = append(changes, ThresholdChange{expression, "removed"})
			continue
		}
		if prior.Passed && !current.Passed {
			changes = append(changes, ThresholdChange{expression, "pass-to-fail"})
		}
		if !prior.Passed && current.Passed {
			changes = append(changes, ThresholdChange{expression, "fail-to-pass"})
		}
	}
	for expression := range right {
		if _, exists := left[expression]; !exists {
			changes = append(changes, ThresholdChange{expression, "added"})
		}
	}
	return changes
}

func (result *Result) String() string {
	var builder strings.Builder
	builder.WriteString("DAEGSA report comparison (factual single-run deltas)\n")
	for _, warning := range result.Warnings {
		fmt.Fprintf(&builder, "WARNING: %s\n", warning)
	}
	for _, delta := range result.Deltas {
		if delta.PercentageAvailable {
			fmt.Fprintf(&builder, "%s: %.2f -> %.2f (%+.2f, %+.2f%%)\n", delta.Name, delta.Baseline, delta.Candidate, delta.Absolute, delta.Percentage)
		} else {
			fmt.Fprintf(&builder, "%s: %.2f -> %.2f (%+.2f, percentage unavailable)\n", delta.Name, delta.Baseline, delta.Candidate, delta.Absolute)
		}
	}
	for _, change := range result.ThresholdChanges {
		fmt.Fprintf(&builder, "threshold %q: %s\n", change.Expression, change.Change)
	}
	return builder.String()
}

func ValidationError(err error) error { return fmt.Errorf("%w: %w", config.ErrConfigValidation, err) }
