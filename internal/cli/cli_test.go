package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/safety"
	"github.com/charleszardd/daegsa/internal/testtarget"
)

func TestCLI_Version(t *testing.T) {
	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"version"})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for version, got %d", code)
	}
}

func TestCLI_Help(t *testing.T) {
	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"--help"})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for --help, got %d", code)
	}
}

func TestCLI_Validate_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "valid.yaml")
	yamlContent := `
schema_version: 1
name: cli-validate-test
request:
  url: http://127.0.0.1:8080/items
  method: GET
load:
  model: open
  rate: 10
  duration: 10s
  max_in_flight: 50
safety:
  allowed_hosts:
    - 127.0.0.1
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"validate", "--config", configFile})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for valid config validation, got %d", code)
	}
}

func TestCLI_Validate_InvalidSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")
	yamlContent := `
schema_version: 1
name: cli-invalid
request:
  url: [not-a-valid-url]
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"validate", "--config", configFile})
	if code != core.ExitCodeValidationFailure {
		t.Errorf("expected exit code 2 (VALIDATION_FAILURE), got %d (%s)", code, code)
	}
}

func TestCLI_Validate_SafetyRefusal(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "disallowed_host.yaml")
	yamlContent := `
schema_version: 1
name: cli-disallowed-host
request:
  url: http://evil.com/api
  method: GET
load:
  model: open
  rate: 10
  duration: 10s
  max_in_flight: 50
safety:
  allowed_hosts:
    - api.example.com
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"validate", "--config", configFile})
	if code != core.ExitCodeSafetyRefusal {
		t.Errorf("expected exit code 4 (SAFETY_REFUSAL), got %d (%s)", code, code)
	}
}

func TestCLI_Run_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "dryrun.yaml")
	yamlContent := `
schema_version: 1
name: cli-dryrun-test
request:
  url: http://127.0.0.1:8080/data
  method: GET
load:
  model: open
  rate: 10
  duration: 10s
  max_in_flight: 50
safety:
  allowed_hosts:
    - 127.0.0.1
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"run", "--config", configFile, "--dry-run"})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for dry-run, got %d", code)
	}
}

func TestCLI_Run_DestructiveUnauthorized(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// POST request without --allow-destructive should fail with exit code 4
	code := ExecuteContext(ctx, []string{"run", "--url", server.URL() + "/items", "--method", "POST"})
	if code != core.ExitCodeSafetyRefusal {
		t.Errorf("expected exit code 4 (SAFETY_REFUSAL) for unauthorized POST, got %d (%s)", code, code)
	}
}

func TestCLI_Run_DestructiveAuthorized(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// POST request with --allow-destructive should succeed with exit code 0
	code := ExecuteContext(ctx, []string{"run", "--url", server.URL() + "/items", "--method", "POST", "--allow-destructive"})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for authorized POST, got %d", code)
	}
}

func TestCLI_Run_ExecuteClosedModel_Success(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// Execute closed model run with 5 users for 200ms
	code := ExecuteContext(ctx, []string{
		"run",
		"--url", server.URL(),
		"--model", "closed",
		"--users", "5",
		"--duration", "200ms",
	})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for closed model load run, got %d", code)
	}
}

func TestCLI_Run_ExecuteClosedModel_OutputJSON(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	tmpDir := t.TempDir()
	outJSON := filepath.Join(tmpDir, "report.json")

	ctx := context.Background()
	// Execute closed model run and export report to JSON
	code := ExecuteContext(ctx, []string{
		"run",
		"--url", server.URL(),
		"--model", "closed",
		"--users", "4",
		"--duration", "150ms",
		"--output-json", outJSON,
	})
	if code != core.ExitCodeSuccess {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	data, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatalf("failed to read exported JSON report: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse exported JSON report: %v", err)
	}

	if int(parsed["report_schema_version"].(float64)) != 1 {
		t.Errorf("expected report_schema_version 1, got %v", parsed["report_schema_version"])
	}
	reqCounts := parsed["request_counts"].(map[string]interface{})
	if reqCounts["completed"].(float64) <= 0 {
		t.Errorf("expected completed requests > 0, got %v", reqCounts["completed"])
	}
}

func TestCLI_Run_ExecuteClosedModel_FromConfigFile(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "closed_test.yaml")
	yamlContent := `
schema_version: 1
name: cli-closed-yaml-test
request:
  url: ` + server.URL() + `
  method: GET
load:
  model: closed
  users: 3
  duration: 150ms
  think_time: 10ms
safety:
  allowed_hosts:
    - 127.0.0.1
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"run", "--config", configFile})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for closed config execution, got %d", code)
	}
}

func TestCLI_Run_ExecuteClosedModel_UnexpectedStatus_ReturnsExitCode1(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// Target returns 500 -> ExitCodeThresholdFailure (1)
	code := ExecuteContext(ctx, []string{
		"run",
		"--url", server.URL() + "/?status=500",
		"--model", "closed",
		"--users", "2",
		"--duration", "100ms",
	})
	if code != core.ExitCodeThresholdFailure {
		t.Errorf("expected exit code 1 (FAIL_THRESHOLDS) for status 500, got %d (%s)", code, code)
	}
}

func TestCLI_DetermineExitCode_Mapping(t *testing.T) {
	// 0: Success
	if code := DetermineExitCode(nil); code != core.ExitCodeSuccess {
		t.Errorf("expected ExitCodeSuccess for nil error, got %d", code)
	}

	// 1: Threshold / Execution outcome failure
	errThreshold := &CLIExitError{Code: core.ExitCodeThresholdFailure, Err: errors.New("threshold failure")}
	if code := DetermineExitCode(errThreshold); code != core.ExitCodeThresholdFailure {
		t.Errorf("expected ExitCodeThresholdFailure, got %d", code)
	}

	// 2: Validation failure
	if code := DetermineExitCode(config.ErrConfigValidation); code != core.ExitCodeValidationFailure {
		t.Errorf("expected ExitCodeValidationFailure, got %d", code)
	}
	if code := DetermineExitCode(config.ErrInvalidEnvSyntax); code != core.ExitCodeValidationFailure {
		t.Errorf("expected ExitCodeValidationFailure for env syntax, got %d", code)
	}

	// 4: Safety refusal
	if code := DetermineExitCode(safety.ErrSafetyRefusal); code != core.ExitCodeSafetyRefusal {
		t.Errorf("expected ExitCodeSafetyRefusal, got %d", code)
	}
	if code := DetermineExitCode(safety.ErrHostNotAllowed); code != core.ExitCodeSafetyRefusal {
		t.Errorf("expected ExitCodeSafetyRefusal for ErrHostNotAllowed, got %d", code)
	}

	// 3: Runtime failure
	errRuntime := errors.New("unhandled network error")
	if code := DetermineExitCode(errRuntime); code != core.ExitCodeRuntimeFailure {
		t.Errorf("expected ExitCodeRuntimeFailure for generic error, got %d", code)
	}
}

func TestCLI_Validate_MissingConfigFile(t *testing.T) {
	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"validate", "--config", "non_existent_file_daegsa.yaml"})
	if code != core.ExitCodeValidationFailure {
		t.Errorf("expected exit code 2 (VALIDATION_FAILURE) for missing file, got %d (%s)", code, code)
	}
}

func TestCLI_Run_NonInteractive_DestructiveRefusal(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// POST request with --non-interactive but no --allow-destructive must refuse with exit code 4
	code := ExecuteContext(ctx, []string{"run", "--url", server.URL() + "/items", "--method", "DELETE", "--non-interactive"})
	if code != core.ExitCodeSafetyRefusal {
		t.Errorf("expected exit code 4 (SAFETY_REFUSAL), got %d (%s)", code, code)
	}
}

func TestCLI_Run_ExecuteOpenModel_Success(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// Execute open model run with 100 req/s for 200ms
	code := ExecuteContext(ctx, []string{
		"run",
		"--url", server.URL(),
		"--model", "open",
		"--rate", "100",
		"--time-unit", "1s",
		"--max-in-flight", "50",
		"--duration", "200ms",
	})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for open model load run, got %d", code)
	}
}

func TestCLI_Run_ExecuteOpenModel_OutputJSON(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	tmpDir := t.TempDir()
	outJSON := filepath.Join(tmpDir, "open_report.json")

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{
		"run",
		"--url", server.URL(),
		"--model", "open",
		"--rate", "100",
		"--max-in-flight", "50",
		"--duration", "200ms",
		"--output-json", outJSON,
	})
	if code != core.ExitCodeSuccess {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	data, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatalf("failed to read exported JSON report: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse exported JSON report: %v", err)
	}

	if int(parsed["report_schema_version"].(float64)) != 1 {
		t.Errorf("expected report_schema_version 1, got %v", parsed["report_schema_version"])
	}
	if parsed["workload_model"] != "open" {
		t.Errorf("expected workload_model 'open', got %v", parsed["workload_model"])
	}
	reqCounts := parsed["request_counts"].(map[string]interface{})
	if reqCounts["completed"].(float64) <= 0 {
		t.Errorf("expected completed requests > 0, got %v", reqCounts["completed"])
	}
}

func TestCLI_Run_ExecuteOpenModel_FromConfigFile(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "open_test.yaml")
	yamlContent := `
schema_version: 1
name: cli-open-yaml-test
request:
  url: ` + server.URL() + `
  method: GET
load:
  model: open
  rate: 50
  time_unit: 1s
  max_in_flight: 20
  duration: 200ms
safety:
  allowed_hosts:
    - 127.0.0.1
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"run", "--config", configFile})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for open config execution, got %d", code)
	}
}

func TestCLI_Run_ExecuteOpenModel_UnexpectedStatus_ReturnsExitCode1(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// Target returns 500 -> ExitCodeThresholdFailure (1)
	code := ExecuteContext(ctx, []string{
		"run",
		"--url", server.URL() + "/?status=500",
		"--model", "open",
		"--rate", "50",
		"--duration", "100ms",
		"--max-in-flight", "20",
	})
	if code != core.ExitCodeThresholdFailure {
		t.Errorf("expected exit code 1 (FAIL_THRESHOLDS) for status 500 in open model, got %d (%s)", code, code)
	}
}

func TestCLI_Run_PassingThresholds_ExitCode0(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "threshold_pass.yaml")
	yamlContent := `
schema_version: 1
name: cli-threshold-pass
request:
  url: ` + server.URL() + `
  method: GET
load:
  model: closed
  users: 2
  duration: 150ms
thresholds:
  http_error_rate: "<= 1%"
  p95: "<= 500ms"
  completed_requests: ">= 5"
safety:
  allowed_hosts:
    - 127.0.0.1
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"run", "--config", configFile})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for passing thresholds, got %d (%s)", code, code)
	}
}

func TestCLI_Run_FailingThresholds_ExitCode1(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "threshold_fail.yaml")
	// Target has 50ms delay, threshold requires p95 <= 5ms -> FAIL
	yamlContent := `
schema_version: 1
name: cli-threshold-fail
request:
  url: ` + server.URL() + `/?delay=50ms
  method: GET
load:
  model: closed
  users: 2
  duration: 150ms
thresholds:
  p95: "<= 5ms"
safety:
  allowed_hosts:
    - 127.0.0.1
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"run", "--config", configFile})
	if code != core.ExitCodeThresholdFailure {
		t.Errorf("expected exit code 1 (FAIL_THRESHOLDS) for failing threshold, got %d (%s)", code, code)
	}
}

func TestCLI_Run_InvalidThresholdSyntax_ExitCode2(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "bad_threshold.yaml")
	// Missing operator in threshold expression
	yamlContent := `
schema_version: 1
name: cli-bad-threshold
request:
  url: http://127.0.0.1:8080/items
  method: GET
load:
  model: open
  rate: 10
  duration: 5s
  max_in_flight: 50
thresholds:
  p95: "500ms"
safety:
  allowed_hosts:
    - 127.0.0.1
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"validate", "--config", configFile})
	if code != core.ExitCodeValidationFailure {
		t.Errorf("expected exit code 2 (VALIDATION_FAILURE) for malformed threshold, got %d (%s)", code, code)
	}
}

func TestCLI_Run_Cancellation_ExitCode3_Incomplete(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately before or during run
	cancel()

	code := ExecuteContext(ctx, []string{
		"run",
		"--url", server.URL(),
		"--model", "closed",
		"--users", "2",
		"--duration", "1s",
	})

	if code != core.ExitCodeRuntimeFailure {
		t.Errorf("expected exit code 3 (RUNTIME_FAILURE) for canceled run, got %d (%s)", code, code)
	}
}

func TestCLI_FormatSingleLineSummary(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		code     core.ExitCode
		contains string
	}{
		{
			name:     "threshold failure summary",
			err:      errors.New("p95 (650.00ms) failed target <= 500ms"),
			code:     core.ExitCodeThresholdFailure,
			contains: "daegsa: threshold failure: p95 (650.00ms) failed target <= 500ms",
		},
		{
			name:     "validation failure summary",
			err:      errors.New("invalid threshold 'p95': missing operator"),
			code:     core.ExitCodeValidationFailure,
			contains: "daegsa: validation failure: invalid threshold 'p95': missing operator",
		},
		{
			name:     "runtime failure summary",
			err:      errors.New("test execution incomplete (aborted)"),
			code:     core.ExitCodeRuntimeFailure,
			contains: "daegsa: runtime failure: test execution incomplete (aborted)",
		},
		{
			name:     "safety refusal summary",
			err:      errors.New("target host 'evil.com' not in allowed_hosts"),
			code:     core.ExitCodeSafetyRefusal,
			contains: "daegsa: safety refusal: target host 'evil.com' not in allowed_hosts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSingleLineSummary(tt.err, tt.code)
			if got != tt.contains {
				t.Errorf("FormatSingleLineSummary() = %q, want %q", got, tt.contains)
			}
		})
	}
}

func TestCLI_Run_JSONExportWithThresholds(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "json_thresholds.yaml")
	outJSON := filepath.Join(tmpDir, "report_with_thresholds.json")

	yamlContent := `
schema_version: 1
name: cli-json-thresholds
request:
  url: ` + server.URL() + `
  method: GET
load:
  model: open
  rate: 20
  duration: 150ms
  max_in_flight: 10
thresholds:
  http_error_rate: "<= 1%"
  p95: "<= 500ms"
safety:
  allowed_hosts:
    - 127.0.0.1
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx := context.Background()
	code := ExecuteContext(ctx, []string{"run", "--config", configFile, "--output-json", outJSON})
	if code != core.ExitCodeSuccess {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	data, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatalf("failed to read JSON report: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON report: %v", err)
	}

	thresholds, ok := parsed["thresholds"].([]interface{})
	if !ok {
		t.Fatalf("expected thresholds array in report, got %T", parsed["thresholds"])
	}
	if len(thresholds) != 2 {
		t.Fatalf("expected 2 threshold results, got %d", len(thresholds))
	}

	for i, item := range thresholds {
		thMap := item.(map[string]interface{})
		if thMap["passed"] != true {
			t.Errorf("threshold[%d] passed = %v, want true", i, thMap["passed"])
		}
		if thMap["expression"] == "" || thMap["target"] == "" || thMap["observed"] == "" {
			t.Errorf("threshold[%d] missing fields: %+v", i, thMap)
		}
	}
}

func TestCLI_Run_AuthenticatedExamples(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	t.Setenv("TARGET_URL", server.URL())
	t.Setenv("API_TOKEN", "secret-bearer-token-123")
	t.Setenv("TOKEN_1", "tok-1-alpha")
	t.Setenv("TOKEN_2", "tok-2-beta")
	t.Setenv("TOKEN_3", "tok-3-gamma")

	examples := []string{
		filepath.Join("..", "..", "examples", "authenticated-api.yaml"),
		filepath.Join("..", "..", "examples", "token-pool-load.yaml"),
		filepath.Join("..", "..", "examples", "cookie-session-closed.yaml"),
	}

	ctx := context.Background()

	for _, ex := range examples {
		t.Run(filepath.Base(ex), func(t *testing.T) {
			tmpDir := t.TempDir()
			outJSON := filepath.Join(tmpDir, "report.json")

			// 1. Test validate command
			valCode := ExecuteContext(ctx, []string{"validate", "--config", ex})
			if valCode != core.ExitCodeSuccess {
				t.Fatalf("daegsa validate %s failed with exit code %d", ex, valCode)
			}

			// 2. Test dry-run
			dryCode := ExecuteContext(ctx, []string{"run", "--config", ex, "--dry-run"})
			if dryCode != core.ExitCodeSuccess {
				t.Fatalf("daegsa run --dry-run %s failed with exit code %d", ex, dryCode)
			}

			// 3. Test actual run with short duration override
			runCode := ExecuteContext(ctx, []string{
				"run",
				"--config", ex,
				"--duration", "500ms",
				"--output-json", outJSON,
			})
			if runCode != core.ExitCodeSuccess {
				t.Fatalf("daegsa run %s failed with exit code %d", ex, runCode)
			}

			// 4. Verify JSON report was created and has auth metadata
			data, err := os.ReadFile(outJSON)
			if err != nil {
				t.Fatalf("failed to read JSON report %s: %v", outJSON, err)
			}

			// Zero secret leakage in report
			prohibited := []string{"secret-bearer-token-123", "tok-1-alpha", "tok-2-beta", "tok-3-gamma"}
			for _, sec := range prohibited {
				if strings.Contains(string(data), sec) {
					t.Fatalf("CRITICAL: Secret %q leaked in exported JSON report: %s", sec, string(data))
				}
			}

			var parsed map[string]interface{}
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("invalid JSON report: %v", err)
			}
			if authObj, ok := parsed["auth"].(map[string]interface{}); ok {
				if authObj["auth_mode"] == "" {
					t.Errorf("expected non-empty auth_mode in report")
				}
			}
		})
	}
}

func TestCLI_AuthenticationSecretsNeverReachOutput(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	const (
		bearerSecret   = "SECRET_TOKEN_ALPHA_999"
		passwordSecret = "SECRET_PASS_BETA_888"
		apiKeySecret   = "SECRET_APIKEY_GAMMA_777"
		poolSecretOne  = "SECRET_POOL_DELTA_666"
		poolSecretTwo  = "SECRET_POOL_EPSILON_555"
		querySecret    = "SECRET_QUERY_ZETA_444"
	)
	secrets := []string{bearerSecret, passwordSecret, apiKeySecret, poolSecretOne, poolSecretTwo, querySecret}

	t.Setenv("CLI_BEARER_SECRET", bearerSecret)
	t.Setenv("CLI_PASSWORD_SECRET", passwordSecret)
	t.Setenv("CLI_API_KEY_SECRET", apiKeySecret)
	t.Setenv("CLI_POOL_SECRET_ONE", poolSecretOne)
	t.Setenv("CLI_POOL_SECRET_TWO", poolSecretTwo)
	t.Setenv("CLI_QUERY_SECRET", querySecret)

	authCases := []struct {
		name     string
		path     string
		authYAML string
	}{
		{name: "bearer", path: "/auth/bearer", authYAML: "  type: bearer\n  token: ${CLI_BEARER_SECRET}\n"},
		{name: "custom header", path: "/auth/header?header_name=X-API-Key", authYAML: "  type: custom_header\n  header_name: X-API-Key\n  token: ${CLI_API_KEY_SECRET}\n"},
		{name: "basic", path: "/auth/basic", authYAML: "  type: basic\n  username: load-user\n  password: ${CLI_PASSWORD_SECRET}\n"},
		{name: "token pool", path: "/auth/token-pool", authYAML: "  type: token_pool\n  token_pool:\n    - ${CLI_POOL_SECRET_ONE}\n    - ${CLI_POOL_SECRET_TWO}\n"},
	}

	for _, testCase := range authCases {
		t.Run(testCase.name, func(t *testing.T) {
			temporaryDirectory := t.TempDir()
			configPath := filepath.Join(temporaryDirectory, "auth.yaml")
			reportPath := filepath.Join(temporaryDirectory, "report.json")
			targetURL := server.URL() + testCase.path
			querySeparator := "?"
			if strings.Contains(targetURL, "?") {
				querySeparator = "&"
			}

			manifest := fmt.Sprintf(`schema_version: 1
name: cli-secret-leakage
request:
  url: %s%sapi_key=${CLI_QUERY_SECRET}
  method: GET
  headers:
    X-Trace-Secret: ${CLI_API_KEY_SECRET}
load:
  model: closed
  users: 2
  duration: 75ms
auth:
%ssafety:
  allowed_hosts:
    - 127.0.0.1
`, targetURL, querySeparator, testCase.authYAML)
			if err := os.WriteFile(configPath, []byte(manifest), 0600); err != nil {
				t.Fatalf("failed to write authentication manifest: %v", err)
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			validateCode := executeContext(context.Background(), []string{"validate", "--config", configPath}, &stdout, &stderr)
			if validateCode != core.ExitCodeSuccess {
				t.Fatalf("validate exit code = %d, want %d; stderr=%s", validateCode, core.ExitCodeSuccess, stderr.String())
			}
			assertSecretsAbsent(t, "validate output", stdout.String()+stderr.String(), secrets)

			stdout.Reset()
			stderr.Reset()
			dryRunCode := executeContext(context.Background(), []string{"run", "--config", configPath, "--dry-run"}, &stdout, &stderr)
			if dryRunCode != core.ExitCodeSuccess {
				t.Fatalf("dry-run exit code = %d, want %d; stderr=%s", dryRunCode, core.ExitCodeSuccess, stderr.String())
			}
			assertSecretsAbsent(t, "dry-run output", stdout.String()+stderr.String(), secrets)

			stdout.Reset()
			stderr.Reset()
			runCode := executeContext(context.Background(), []string{"run", "--config", configPath, "--output-json", reportPath}, &stdout, &stderr)
			if runCode != core.ExitCodeSuccess {
				t.Fatalf("run exit code = %d, want %d; stderr=%s", runCode, core.ExitCodeSuccess, stderr.String())
			}
			assertSecretsAbsent(t, "terminal output", stdout.String()+stderr.String(), secrets)

			reportBytes, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatalf("failed to read JSON report: %v", err)
			}
			assertSecretsAbsent(t, "JSON report", string(reportBytes), secrets)
			var reportDocument struct {
				ConfigFingerprint string `json:"config_fingerprint"`
			}
			if err := json.Unmarshal(reportBytes, &reportDocument); err != nil {
				t.Fatalf("failed to parse JSON report: %v", err)
			}
			if len(reportDocument.ConfigFingerprint) != 64 {
				t.Errorf("configuration fingerprint length = %d, want 64", len(reportDocument.ConfigFingerprint))
			}
			assertSecretsAbsent(t, "configuration fingerprint", reportDocument.ConfigFingerprint, secrets)
		})
	}

	t.Run("failure stderr", func(t *testing.T) {
		temporaryDirectory := t.TempDir()
		configPath := filepath.Join(temporaryDirectory, "threshold-failure.yaml")
		reportPath := filepath.Join(temporaryDirectory, "failure-report.json")
		manifest := fmt.Sprintf(`schema_version: 1
name: cli-secret-failure
request:
  url: %s/auth/bearer?token=${CLI_QUERY_SECRET}
  method: GET
load:
  model: closed
  users: 1
  duration: 50ms
auth:
  type: bearer
  token: ${CLI_BEARER_SECRET}
thresholds:
  completed_requests: ">= 999999"
safety:
  allowed_hosts:
    - 127.0.0.1
`, server.URL())
		if err := os.WriteFile(configPath, []byte(manifest), 0600); err != nil {
			t.Fatalf("failed to write failure manifest: %v", err)
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exitCode := executeContext(context.Background(), []string{"run", "--config", configPath, "--output-json", reportPath}, &stdout, &stderr)
		if exitCode != core.ExitCodeThresholdFailure {
			t.Fatalf("failure exit code = %d, want %d; stderr=%s", exitCode, core.ExitCodeThresholdFailure, stderr.String())
		}
		if stderr.Len() == 0 {
			t.Fatal("expected a CI failure summary on stderr")
		}
		assertSecretsAbsent(t, "failure stdout and stderr", stdout.String()+stderr.String(), secrets)
		reportBytes, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatalf("failed to read failure JSON report: %v", err)
		}
		assertSecretsAbsent(t, "failure JSON report", string(reportBytes), secrets)
	})
}

func assertSecretsAbsent(t *testing.T, outputName, output string, secrets []string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(output, secret) {
			t.Errorf("%s contains credential sentinel %q", outputName, secret)
		}
	}
}

func TestCLI_InvalidURLFailureRedactsSensitiveQuery(t *testing.T) {
	const secret = "SECRET_QUERY_ZETA_444"
	temporaryDirectory := t.TempDir()
	configPath := filepath.Join(temporaryDirectory, "invalid-url.yaml")
	manifest := `schema_version: 1
name: invalid-url-secret-redaction
request:
  url: http://127.0.0.1/%zz?client_secret=` + secret + `
  method: GET
load:
  model: closed
  users: 1
  duration: 1s
safety:
  allowed_hosts:
    - 127.0.0.1
`
	if err := os.WriteFile(configPath, []byte(manifest), 0600); err != nil {
		t.Fatalf("failed to write malformed URL manifest: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := executeContext(context.Background(), []string{"validate", "--config", configPath}, &stdout, &stderr)
	if exitCode != core.ExitCodeValidationFailure {
		t.Fatalf("exit code = %d, want %d", exitCode, core.ExitCodeValidationFailure)
	}
	combinedOutput := stdout.String() + stderr.String()
	if strings.Contains(combinedOutput, secret) {
		t.Fatalf("invalid URL failure output leaked secret: %s", combinedOutput)
	}
	if !strings.Contains(combinedOutput, config.RedactedPlaceholder) {
		t.Errorf("invalid URL failure output = %q, want redaction placeholder", combinedOutput)
	}
}

func TestCLI_InvalidURLFailureRedactsUsernameOnlyUserInfo(t *testing.T) {
	const secretUsername = "SECRET_USER_ALPHA_111"
	temporaryDirectory := t.TempDir()
	configPath := filepath.Join(temporaryDirectory, "invalid-userinfo-url.yaml")
	manifest := `schema_version: 1
name: invalid-userinfo-secret-redaction
request:
  url: http://` + secretUsername + `@127.0.0.1/%zz
  method: GET
load:
  model: closed
  users: 1
  duration: 1s
safety:
  allowed_hosts:
    - 127.0.0.1
`
	if err := os.WriteFile(configPath, []byte(manifest), 0600); err != nil {
		t.Fatalf("failed to write malformed userinfo URL manifest: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := executeContext(context.Background(), []string{"validate", "--config", configPath}, &stdout, &stderr)
	if exitCode != core.ExitCodeValidationFailure {
		t.Fatalf("exit code = %d, want %d", exitCode, core.ExitCodeValidationFailure)
	}
	combinedOutput := stdout.String() + stderr.String()
	if strings.Contains(combinedOutput, secretUsername) {
		t.Fatalf("invalid username-only userinfo output leaked secret: %s", combinedOutput)
	}
	if !strings.Contains(combinedOutput, config.RedactedPlaceholder) {
		t.Errorf("invalid username-only userinfo output = %q, want redaction placeholder", combinedOutput)
	}
}
