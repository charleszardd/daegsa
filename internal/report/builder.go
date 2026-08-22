package report

import (
	"runtime"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
)

// Build metadata defaults for report generation (§13, §15).
var (
	DefaultDaegsaVersion = "v0.1.0-dev"
	DefaultCommit        = "unknown"
	DefaultBuildDate     = "unknown"
)

// BuildReport constructs a Report conforming to schema version 1 from test execution artifacts (§13).
func BuildReport(
	p *plan.Plan,
	agg *metrics.AggregatedMetrics,
	health *metrics.GeneratorHealth,
	startTime time.Time,
	endTime time.Time,
	incomplete bool,
) *Report {
	startUTC := startTime.UTC()
	endUTC := endTime.UTC()
	durationMS := endUTC.Sub(startUTC).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}

	rep := &Report{
		ReportSchemaVersion: ExpectedReportSchemaVersion,
		DaegsaVersion:       DefaultDaegsaVersion,
		Commit:              DefaultCommit,
		BuildDate:           DefaultBuildDate,
		OS:                  runtime.GOOS,
		Arch:                runtime.GOARCH,
		StartTimeUTC:        startUTC,
		EndTimeUTC:          endUTC,
		DurationMS:          durationMS,
		Thresholds:          make([]ThresholdResult, 0),
		Incomplete:          incomplete,
	}

	if p != nil {
		rep.ConfigFingerprint = p.Fingerprint
		rep.WorkloadModel = p.Model
	}

	if agg != nil {
		rep.RequestCounts = agg.RequestCounts
		rep.Outcomes = agg.Outcomes
		rep.StatusCodes = agg.StatusCodes
		rep.Latency = agg.Latency
		rep.RateLimits = agg.RateLimits
	} else {
		rep.Outcomes = make(map[core.Outcome]int64)
		rep.StatusCodes = make(map[string]int64)
	}

	if health != nil {
		rep.GeneratorHealth = *health
	}

	return rep
}
