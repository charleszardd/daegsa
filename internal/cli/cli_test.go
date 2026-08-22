package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestCLI_Run_ExecuteAgainstTestTarget(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// Execute GET request against live test target
	code := ExecuteContext(ctx, []string{"run", "--url", server.URL() + "?status=200"})
	if code != core.ExitCodeSuccess {
		t.Errorf("expected exit code 0 for successful execution, got %d", code)
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

func TestCLI_Run_UnexpectedStatus_ReturnsExitCode1(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// GET request returning 500 when expecting 200 -> ExitCodeThresholdFailure (1)
	code := ExecuteContext(ctx, []string{"run", "--url", server.URL() + "?status=500"})
	if code != core.ExitCodeThresholdFailure {
		t.Errorf("expected exit code 1 (FAIL_THRESHOLDS) for status 500, got %d (%s)", code, code)
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
