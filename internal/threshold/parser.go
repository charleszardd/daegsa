package threshold

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ParseThreshold parses and strictly validates a single metric threshold expression (§6, §10).
//
// Examples of valid expressions:
//   - http_error_rate: "<= 1%"
//   - p95: "<= 500ms"
//   - p99: "< 1s"
//   - p50: "<= 250µs" (or "250us")
//   - completed_rps: ">= 90" (or ">= 90 req/s")
//   - dropped_requests: "== 0"
//   - max_in_flight: "<= 100"
func ParseThreshold(metricName string, expr string) (*Threshold, error) {
	trimmedMetric := strings.TrimSpace(metricName)
	if trimmedMetric == "" {
		return nil, fmt.Errorf("%w: threshold metric name cannot be empty", ErrInvalidThreshold)
	}

	var stepName string
	canonicalMetric := trimmedMetric

	if strings.HasPrefix(trimmedMetric, "step.") {
		parts := strings.Split(trimmedMetric, ".")
		if len(parts) != 3 || parts[0] != "step" {
			return nil, fmt.Errorf("%w: invalid step threshold metric syntax %q (expected step.<step_name>.<metric>)", ErrInvalidThreshold, trimmedMetric)
		}
		stepName = strings.TrimSpace(parts[1])
		canonicalMetric = strings.TrimSpace(parts[2])
		if stepName == "" || canonicalMetric == "" {
			return nil, fmt.Errorf("%w: step name and metric cannot be empty in %q", ErrInvalidThreshold, trimmedMetric)
		}
	}

	category, exists := CanonicalMetricCategories[canonicalMetric]
	if !exists {
		return nil, fmt.Errorf("%w: unknown threshold metric %q", ErrInvalidThreshold, canonicalMetric)
	}

	trimmedExpr := strings.TrimSpace(expr)
	if trimmedExpr == "" {
		return nil, fmt.Errorf("%w: threshold expression for %q cannot be empty", ErrInvalidThreshold, trimmedMetric)
	}

	// Extract comparison operator (checked in order of decreasing prefix length)
	var op string
	switch {
	case strings.HasPrefix(trimmedExpr, OpLTE):
		op = OpLTE
	case strings.HasPrefix(trimmedExpr, OpGTE):
		op = OpGTE
	case strings.HasPrefix(trimmedExpr, OpEQ):
		op = OpEQ
	case strings.HasPrefix(trimmedExpr, OpNE):
		op = OpNE
	case strings.HasPrefix(trimmedExpr, OpLT):
		op = OpLT
	case strings.HasPrefix(trimmedExpr, OpGT):
		op = OpGT
	default:
		return nil, fmt.Errorf("%w: threshold %q expression %q must begin with an operator (<, <=, >, >=, ==, !=)",
			ErrInvalidThreshold, trimmedMetric, expr)
	}

	targetStr := strings.TrimSpace(trimmedExpr[len(op):])
	if targetStr == "" {
		return nil, fmt.Errorf("%w: threshold %q expression %q is missing target value",
			ErrInvalidThreshold, trimmedMetric, expr)
	}

	targetValue, unit, err := parseTargetValueAndUnit(canonicalMetric, category, targetStr)
	if err != nil {
		return nil, err
	}

	return &Threshold{
		MetricName:    trimmedMetric,
		StepName:      stepName,
		Category:      category,
		Operator:      op,
		TargetRaw:     targetStr,
		TargetValue:   targetValue,
		Unit:          unit,
		RawExpression: trimmedExpr,
	}, nil
}

// ParseThresholds parses a map of metric threshold expressions into a sorted slice of Threshold structs.
func ParseThresholds(thresholdMap map[string]string) ([]*Threshold, error) {
	if len(thresholdMap) == 0 {
		return make([]*Threshold, 0), nil
	}

	keys := make([]string, 0, len(thresholdMap))
	for k := range thresholdMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	thresholds := make([]*Threshold, 0, len(keys))
	for _, key := range keys {
		t, err := ParseThreshold(key, thresholdMap[key])
		if err != nil {
			return nil, err
		}
		thresholds = append(thresholds, t)
	}

	return thresholds, nil
}

func parseTargetValueAndUnit(metricName string, category MetricCategory, targetStr string) (float64, string, error) {
	switch category {
	case MetricCategoryRate:
		// Requires '%' suffix
		if !strings.HasSuffix(targetStr, "%") {
			return 0, "", fmt.Errorf("%w: rate metric %q requires '%%' unit, got %q",
				ErrInvalidThreshold, metricName, targetStr)
		}
		numStr := strings.TrimSpace(strings.TrimSuffix(targetStr, "%"))
		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, "", fmt.Errorf("%w: invalid numeric value in rate threshold for %q (%q): %w",
				ErrInvalidThreshold, metricName, targetStr, err)
		}
		if val < 0.0 || val > 100.0 {
			return 0, "", fmt.Errorf("%w: rate metric %q target must be between 0%% and 100%%, got %g%%",
				ErrInvalidThreshold, metricName, val)
		}
		return val, "%", nil

	case MetricCategoryLatency:
		// Rejects '%' or throughput units
		if strings.HasSuffix(targetStr, "%") || strings.HasSuffix(targetStr, "req/s") || strings.HasSuffix(targetStr, "rps") {
			return 0, "", fmt.Errorf("%w: latency metric %q requires a duration unit (ms, s, µs, us), got %q",
				ErrInvalidThreshold, metricName, targetStr)
		}

		var numStr string
		var unit string
		var multiplierMS float64

		switch {
		case strings.HasSuffix(targetStr, "ms"):
			numStr = strings.TrimSpace(strings.TrimSuffix(targetStr, "ms"))
			unit = "ms"
			multiplierMS = 1.0
		case strings.HasSuffix(targetStr, "µs"):
			numStr = strings.TrimSpace(strings.TrimSuffix(targetStr, "µs"))
			unit = "µs"
			multiplierMS = 0.001
		case strings.HasSuffix(targetStr, "us"):
			numStr = strings.TrimSpace(strings.TrimSuffix(targetStr, "us"))
			unit = "µs"
			multiplierMS = 0.001
		case strings.HasSuffix(targetStr, "s"):
			numStr = strings.TrimSpace(strings.TrimSuffix(targetStr, "s"))
			unit = "s"
			multiplierMS = 1000.0
		default:
			return 0, "", fmt.Errorf("%w: latency metric %q requires a duration unit (ms, s, µs, us), got %q",
				ErrInvalidThreshold, metricName, targetStr)
		}

		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, "", fmt.Errorf("%w: invalid numeric value in latency threshold for %q (%q): %w",
				ErrInvalidThreshold, metricName, targetStr, err)
		}
		if val < 0 {
			return 0, "", fmt.Errorf("%w: latency metric %q target cannot be negative, got %g",
				ErrInvalidThreshold, metricName, val)
		}
		return val * multiplierMS, unit, nil

	case MetricCategoryThroughput:
		// Rejects duration or rate units
		if strings.HasSuffix(targetStr, "%") || strings.HasSuffix(targetStr, "ms") || strings.HasSuffix(targetStr, "µs") || strings.HasSuffix(targetStr, "us") {
			return 0, "", fmt.Errorf("%w: throughput metric %q cannot use duration/rate units, got %q",
				ErrInvalidThreshold, metricName, targetStr)
		}

		var numStr string
		var unit string
		switch {
		case strings.HasSuffix(targetStr, "req/s"):
			numStr = strings.TrimSpace(strings.TrimSuffix(targetStr, "req/s"))
			unit = "req/s"
		case strings.HasSuffix(targetStr, "rps"):
			numStr = strings.TrimSpace(strings.TrimSuffix(targetStr, "rps"))
			unit = "req/s"
		default:
			numStr = targetStr
			unit = "req/s"
		}

		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, "", fmt.Errorf("%w: invalid numeric value in throughput threshold for %q (%q): %w",
				ErrInvalidThreshold, metricName, targetStr, err)
		}
		if val < 0 {
			return 0, "", fmt.Errorf("%w: throughput metric %q target cannot be negative, got %g",
				ErrInvalidThreshold, metricName, val)
		}
		return val, unit, nil

	case MetricCategoryCount, MetricCategoryConcurrency:
		// Must be a non-negative integer
		if strings.ContainsAny(targetStr, ".%abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZµ") {
			return 0, "", fmt.Errorf("%w: metric %q requires non-negative integer target, got %q",
				ErrInvalidThreshold, metricName, targetStr)
		}

		valInt, err := strconv.ParseInt(targetStr, 10, 64)
		if err != nil {
			return 0, "", fmt.Errorf("%w: invalid integer value for %q (%q): %w",
				ErrInvalidThreshold, metricName, targetStr, err)
		}
		if valInt < 0 {
			return 0, "", fmt.Errorf("%w: metric %q target cannot be negative, got %d",
				ErrInvalidThreshold, metricName, valInt)
		}
		return float64(valInt), "", nil

	default:
		return 0, "", fmt.Errorf("%w: unhandled metric category %q for metric %q",
			ErrInvalidThreshold, category, metricName)
	}
}
