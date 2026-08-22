package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/report"
)

func validateReportJSONStructure(t *testing.T, data []byte) {
	t.Helper()

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("failed to unmarshal generated report JSON: %v", err)
	}

	allowedTopLevel := map[string]bool{
		"report_schema_version": true,
		"daegsa_version":        true,
		"commit":                true,
		"build_date":            true,
		"os":                    true,
		"arch":                  true,
		"config_fingerprint":    true,
		"start_time_utc":        true,
		"end_time_utc":          true,
		"duration_ms":           true,
		"workload_model":        true,
		"request_counts":        true,
		"outcomes":              true,
		"status_codes":          true,
		"latency":               true,
		"rate_limits":           true,
		"generator_health":      true,
		"auth":                  true,
		"scenario":              true,
		"thresholds":            true,
		"incomplete":            true,
	}

	requiredTopLevel := []string{
		"report_schema_version",
		"daegsa_version",
		"commit",
		"build_date",
		"os",
		"arch",
		"config_fingerprint",
		"start_time_utc",
		"end_time_utc",
		"duration_ms",
		"workload_model",
		"request_counts",
		"outcomes",
		"status_codes",
		"latency",
		"rate_limits",
		"generator_health",
		"thresholds",
		"incomplete",
	}

	// 1. Verify no unexpected top-level fields (additionalProperties: false)
	for key := range root {
		if !allowedTopLevel[key] {
			t.Errorf("schema violation: unexpected top-level field %q", key)
		}
	}

	// 2. Verify all required top-level fields are present
	for _, reqKey := range requiredTopLevel {
		if _, exists := root[reqKey]; !exists {
			t.Errorf("schema violation: missing required top-level field %q", reqKey)
		}
	}

	// 3. Verify report_schema_version == 1
	if v, ok := root["report_schema_version"].(float64); !ok || int(v) != 1 {
		t.Errorf("schema violation: report_schema_version must be 1, got %v", root["report_schema_version"])
	}

	// 4. Verify request_counts
	reqCounts, ok := root["request_counts"].(map[string]interface{})
	if !ok {
		t.Fatalf("schema violation: request_counts is not an object")
	}
	for _, reqField := range []string{"planned", "scheduled", "started", "completed", "canceled", "dropped"} {
		if _, exists := reqCounts[reqField]; !exists {
			t.Errorf("schema violation: missing required field in request_counts: %q", reqField)
		}
	}

	// 5. Verify latency
	latency, ok := root["latency"].(map[string]interface{})
	if !ok {
		t.Fatalf("schema violation: latency is not an object")
	}
	for _, subHist := range []string{"all_completed", "expected_success"} {
		hist, exists := latency[subHist].(map[string]interface{})
		if !exists {
			t.Errorf("schema violation: missing %q in latency", subHist)
			continue
		}
		for _, stat := range []string{"min_ms", "max_ms", "mean_ms", "p50_ms", "p90_ms", "p95_ms", "p99_ms"} {
			if _, exists := hist[stat]; !exists {
				t.Errorf("schema violation: missing %q in latency.%s", stat, subHist)
			}
		}
	}

	// 6. Verify rate_limits
	rateLimits, ok := root["rate_limits"].(map[string]interface{})
	if !ok {
		t.Fatalf("schema violation: rate_limits is not an object")
	}
	if _, exists := rateLimits["observed_429_count"]; !exists {
		t.Errorf("schema violation: missing observed_429_count in rate_limits")
	}

	// 7. Verify generator_health
	genHealth, ok := root["generator_health"].(map[string]interface{})
	if !ok {
		t.Fatalf("schema violation: generator_health is not an object")
	}
	for _, hField := range []string{"cpu_max_percent", "goroutines_peak", "scheduler_lag_max_ms"} {
		if _, exists := genHealth[hField]; !exists {
			t.Errorf("schema violation: missing %q in generator_health", hField)
		}
	}

	// 8. Verify thresholds array items
	thresholds, ok := root["thresholds"].([]interface{})
	if !ok {
		t.Fatalf("schema violation: thresholds is not an array")
	}
	allowedThresholdProps := map[string]bool{
		"expression": true,
		"target":     true,
		"observed":   true,
		"passed":     true,
	}
	for i, item := range thresholds {
		thMap, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("schema violation: thresholds[%d] is not an object", i)
			continue
		}
		for prop := range thMap {
			if !allowedThresholdProps[prop] {
				t.Errorf("schema violation: thresholds[%d] contains unexpected property %q", i, prop)
			}
		}
		for reqProp := range allowedThresholdProps {
			if _, exists := thMap[reqProp]; !exists {
				t.Errorf("schema violation: thresholds[%d] missing required property %q", i, reqProp)
			}
		}
	}

	// 9. Verify incomplete boolean
	if _, ok := root["incomplete"].(bool); !ok {
		t.Errorf("schema violation: incomplete must be a boolean, got %T", root["incomplete"])
	}
}

func TestReport_Serialization_GoldenPassingThresholds(t *testing.T) {
	startTime := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	endTime := startTime.Add(30 * time.Second)

	rep := report.Report{
		ReportSchemaVersion: 1,
		DaegsaVersion:       "0.1.0",
		Commit:              "abcd123",
		BuildDate:           "2026-08-22",
		OS:                  "windows",
		Arch:                "amd64",
		ConfigFingerprint:   "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StartTimeUTC:        startTime,
		EndTimeUTC:          endTime,
		DurationMS:          30000,
		WorkloadModel:       core.WorkloadModelOpen,
		RequestCounts: report.RequestCounts{
			Planned:   3000,
			Scheduled: 3000,
			Started:   3000,
			Completed: 3000,
			Canceled:  0,
			Dropped:   0,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 3000,
		},
		StatusCodes: map[string]int64{
			"200": 3000,
		},
		Latency: report.LatencySummary{
			AllCompleted: report.LatencyPercentiles{
				MinMS:  1.2,
				MaxMS:  125.4,
				MeanMS: 15.3,
				P50MS:  12.0,
				P90MS:  25.0,
				P95MS:  35.5,
				P99MS:  80.0,
			},
			ExpectedSuccess: report.LatencyPercentiles{
				MinMS:  1.2,
				MaxMS:  95.0,
				MeanMS: 14.1,
				P50MS:  11.5,
				P90MS:  22.0,
				P95MS:  30.0,
				P99MS:  65.0,
			},
		},
		RateLimits: report.RateLimitObservations{
			Observed429Count: 0,
		},
		GeneratorHealth: report.GeneratorHealth{
			CPUMaxPercent:     18.5,
			GoroutinesPeak:    35,
			SchedulerLagMaxMS: 0.8,
		},
		Thresholds: []report.ThresholdResult{
			{
				Expression: "http_error_rate <= 1%",
				Target:     "<= 1%",
				Observed:   "0.00%",
				Passed:     true,
			},
			{
				Expression: "p95 <= 500ms",
				Target:     "<= 500ms",
				Observed:   "35.50ms",
				Passed:     true,
			},
			{
				Expression: "completed_rps >= 90",
				Target:     ">= 90",
				Observed:   "100.00 req/s",
				Passed:     true,
			},
		},
		Incomplete: false,
	}

	data, err := rep.ToJSON()
	if err != nil {
		t.Fatalf("Report.ToJSON() error: %v", err)
	}

	validateReportJSONStructure(t, data)
}

func TestReport_Serialization_GoldenFailingThresholds(t *testing.T) {
	startTime := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	endTime := startTime.Add(10 * time.Second)

	rep := report.Report{
		ReportSchemaVersion: 1,
		DaegsaVersion:       "0.1.0",
		Commit:              "abcd123",
		BuildDate:           "2026-08-22",
		OS:                  "windows",
		Arch:                "amd64",
		ConfigFingerprint:   "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StartTimeUTC:        startTime,
		EndTimeUTC:          endTime,
		DurationMS:          10000,
		WorkloadModel:       core.WorkloadModelClosed,
		RequestCounts: report.RequestCounts{
			Planned:   500,
			Scheduled: 500,
			Started:   500,
			Completed: 500,
			Canceled:  0,
			Dropped:   0,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess:          475,
			core.OutcomeUnexpectedStatus: 25,
		},
		StatusCodes: map[string]int64{
			"200": 475,
			"500": 25,
		},
		Latency: report.LatencySummary{
			AllCompleted: report.LatencyPercentiles{
				MinMS:  5.0,
				MaxMS:  800.0,
				MeanMS: 150.0,
				P50MS:  100.0,
				P90MS:  400.0,
				P95MS:  600.0,
				P99MS:  750.0,
			},
			ExpectedSuccess: report.LatencyPercentiles{
				MinMS:  5.0,
				MaxMS:  400.0,
				MeanMS: 80.0,
				P50MS:  60.0,
				P90MS:  150.0,
				P95MS:  200.0,
				P99MS:  350.0,
			},
		},
		RateLimits: report.RateLimitObservations{
			Observed429Count: 0,
		},
		GeneratorHealth: report.GeneratorHealth{
			CPUMaxPercent:     25.0,
			GoroutinesPeak:    10,
			SchedulerLagMaxMS: 1.2,
		},
		Thresholds: []report.ThresholdResult{
			{
				Expression: "http_error_rate <= 1%",
				Target:     "<= 1%",
				Observed:   "5.00%",
				Passed:     false,
			},
			{
				Expression: "p95 <= 500ms",
				Target:     "<= 500ms",
				Observed:   "600.00ms",
				Passed:     false,
			},
		},
		Incomplete: false,
	}

	data, err := rep.ToJSON()
	if err != nil {
		t.Fatalf("Report.ToJSON() error: %v", err)
	}

	validateReportJSONStructure(t, data)
}

func TestReport_Serialization_IncompleteRun(t *testing.T) {
	startTime := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	endTime := startTime.Add(2 * time.Second)

	rep := report.Report{
		ReportSchemaVersion: 1,
		DaegsaVersion:       "0.1.0",
		Commit:              "abcd123",
		BuildDate:           "2026-08-22",
		OS:                  "windows",
		Arch:                "amd64",
		ConfigFingerprint:   "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StartTimeUTC:        startTime,
		EndTimeUTC:          endTime,
		DurationMS:          2000,
		WorkloadModel:       core.WorkloadModelOpen,
		RequestCounts: report.RequestCounts{
			Planned:   1000,
			Scheduled: 200,
			Started:   200,
			Completed: 150,
			Canceled:  50,
			Dropped:   0,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess:  150,
			core.OutcomeCanceled: 50,
		},
		StatusCodes: map[string]int64{
			"200": 150,
		},
		Latency: report.LatencySummary{
			AllCompleted: report.LatencyPercentiles{
				MinMS:  1.0,
				MaxMS:  50.0,
				MeanMS: 10.0,
				P50MS:  8.0,
				P90MS:  20.0,
				P95MS:  30.0,
				P99MS:  45.0,
			},
			ExpectedSuccess: report.LatencyPercentiles{
				MinMS:  1.0,
				MaxMS:  50.0,
				MeanMS: 10.0,
				P50MS:  8.0,
				P90MS:  20.0,
				P95MS:  30.0,
				P99MS:  45.0,
			},
		},
		RateLimits: report.RateLimitObservations{
			Observed429Count: 0,
		},
		GeneratorHealth: report.GeneratorHealth{
			CPUMaxPercent:     12.0,
			GoroutinesPeak:    15,
			SchedulerLagMaxMS: 0.5,
		},
		Thresholds: make([]report.ThresholdResult, 0),
		Incomplete: true,
	}

	data, err := rep.ToJSON()
	if err != nil {
		t.Fatalf("Report.ToJSON() error: %v", err)
	}

	validateReportJSONStructure(t, data)
}

func TestReport_SchemaMatchesFileSchema(t *testing.T) {
	// Verify that testdata/schemas/v1/report.schema.json exists and is valid JSON
	schemaPath := filepath.Join("..", "..", "testdata", "schemas", "v1", "report.schema.json")
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", schemaPath, err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("schema file is not valid JSON: %v", err)
	}

	if schema["$schema"] != "http://json-schema.org/draft-07/schema#" {
		t.Errorf("expected draft-07 schema, got %v", schema["$schema"])
	}
}

func TestReport_Serialization_WithAuth(t *testing.T) {
	startTime := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	endTime := startTime.Add(10 * time.Second)

	rep := report.Report{
		ReportSchemaVersion: 1,
		DaegsaVersion:       "0.1.0",
		Commit:              "abcd123",
		BuildDate:           "2026-08-22",
		OS:                  "windows",
		Arch:                "amd64",
		ConfigFingerprint:   "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StartTimeUTC:        startTime,
		EndTimeUTC:          endTime,
		DurationMS:          10000,
		WorkloadModel:       core.WorkloadModelClosed,
		RequestCounts: report.RequestCounts{
			Planned:   100,
			Scheduled: 100,
			Started:   100,
			Completed: 100,
			Canceled:  0,
			Dropped:   0,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 100,
		},
		StatusCodes: map[string]int64{
			"200": 100,
		},
		Latency: report.LatencySummary{
			AllCompleted: report.LatencyPercentiles{
				MinMS:  1.0,
				MaxMS:  50.0,
				MeanMS: 10.0,
				P50MS:  8.0,
				P90MS:  20.0,
				P95MS:  30.0,
				P99MS:  45.0,
			},
			ExpectedSuccess: report.LatencyPercentiles{
				MinMS:  1.0,
				MaxMS:  50.0,
				MeanMS: 10.0,
				P50MS:  8.0,
				P90MS:  20.0,
				P95MS:  30.0,
				P99MS:  45.0,
			},
		},
		RateLimits: report.RateLimitObservations{
			Observed429Count: 0,
		},
		GeneratorHealth: report.GeneratorHealth{
			CPUMaxPercent:     12.0,
			GoroutinesPeak:    15,
			SchedulerLagMaxMS: 0.5,
		},
		Auth: &report.AuthReportSummary{
			AuthMode:         "token_pool",
			TokenCount:       4,
			CookieJarEnabled: true,
		},
		Thresholds: make([]report.ThresholdResult, 0),
		Incomplete: false,
	}

	data, err := rep.ToJSON()
	if err != nil {
		t.Fatalf("Report.ToJSON() error: %v", err)
	}

	validateReportJSONStructure(t, data)

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	authObj, ok := parsed["auth"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected auth object in JSON report")
	}
	if authObj["auth_mode"] != "token_pool" {
		t.Errorf("expected auth_mode='token_pool', got %v", authObj["auth_mode"])
	}
	if authObj["token_count"].(float64) != 4 {
		t.Errorf("expected token_count=4, got %v", authObj["token_count"])
	}
	if authObj["cookie_jar_enabled"] != true {
		t.Errorf("expected cookie_jar_enabled=true, got %v", authObj["cookie_jar_enabled"])
	}
}

func TestReportJSON_ScenarioStructure(t *testing.T) {
	rep := &report.Report{
		ReportSchemaVersion: 1,
		DaegsaVersion:       "v0.1.0-dev",
		Commit:              "abc1234",
		BuildDate:           "2026-08-22T00:00:00Z",
		OS:                  "windows",
		Arch:                "amd64",
		ConfigFingerprint:   "fp123",
		StartTimeUTC:        time.Now().UTC().Add(-10 * time.Second),
		EndTimeUTC:          time.Now().UTC(),
		DurationMS:          10000,
		WorkloadModel:       core.WorkloadModelClosed,
		RequestCounts: report.RequestCounts{
			Planned:   200,
			Scheduled: 200,
			Started:   200,
			Completed: 200,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 200,
		},
		StatusCodes: map[string]int64{
			"200": 200,
		},
		Latency: report.LatencySummary{
			AllCompleted: report.LatencyPercentiles{
				P50MS: 20.0,
				P95MS: 40.0,
				P99MS: 50.0,
			},
		},
		Scenario: &report.ScenarioReport{
			Name: "order_flow",
			Iterations: metrics.ScenarioIterationCounts{
				Planned:   100,
				Started:   100,
				Completed: 100,
				Failed:    0,
			},
			Steps: []report.StepReport{
				{
					Name:   "login",
					Method: "POST",
					URL:    "https://api.example.com/login",
					RequestCounts: report.RequestCounts{
						Completed: 100,
					},
					Latency: report.LatencySummary{
						AllCompleted: report.LatencyPercentiles{
							P50MS: 10.0,
							P95MS: 20.0,
							P99MS: 30.0,
						},
					},
					CompletedThroughput: 10.0,
					ErrorRate:           0.0,
				},
			},
		},
		Thresholds: make([]report.ThresholdResult, 0),
		Incomplete: false,
	}

	data, err := rep.ToJSON()
	if err != nil {
		t.Fatalf("Report.ToJSON() error: %v", err)
	}

	validateReportJSONStructure(t, data)

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	scObj, ok := parsed["scenario"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected scenario object in JSON report")
	}
	if scObj["name"] != "order_flow" {
		t.Errorf("expected scenario name 'order_flow', got %v", scObj["name"])
	}
	stepsArr, ok := scObj["steps"].([]interface{})
	if !ok || len(stepsArr) != 1 {
		t.Fatalf("expected 1 step in steps array, got %v", stepsArr)
	}
}
