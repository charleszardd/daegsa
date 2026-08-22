package cli

import (
	"errors"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/safety"
)

// ExitCoder is an interface implemented by errors that carry an explicit ExitCode.
type ExitCoder interface {
	ExitCode() core.ExitCode
}

// CLIExitError attaches a specific process ExitCode to an error (§10).
type CLIExitError struct {
	Code core.ExitCode
	Err  error
}

func (e *CLIExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code.String()
}

func (e *CLIExitError) Unwrap() error {
	return e.Err
}

func (e *CLIExitError) ExitCode() core.ExitCode {
	return e.Code
}

// DetermineExitCode maps errors deterministically to canonical DAEGSA process exit codes (§10).
//
// Exit Codes:
//   0: PASS (All tests/validation passed)
//   1: FAIL_THRESHOLDS (Executed, but thresholds or expected status failed)
//   2: VALIDATION_FAILURE (CLI usage, syntax, missing env, schema, or invariant failure)
//   3: RUNTIME_FAILURE (Tool or runtime crash, dial init failure)
//   4: SAFETY_REFUSAL (Host allowlist, destructive method, or hard ceiling refusal)
func DetermineExitCode(err error) core.ExitCode {
	if err == nil {
		return core.ExitCodeSuccess
	}

	// Check if error explicitly declares an ExitCode
	var exitCoder ExitCoder
	if errors.As(err, &exitCoder) {
		return exitCoder.ExitCode()
	}

	// Safety Refusals -> ExitCode 4
	if errors.Is(err, safety.ErrSafetyRefusal) ||
		errors.Is(err, safety.ErrHostNotAllowed) ||
		errors.Is(err, safety.ErrDestructiveMethodUnauthorized) ||
		errors.Is(err, safety.ErrSafetyCeilingExceeded) ||
		errors.Is(err, safety.ErrCrossOriginRedirectBlocked) ||
		errors.Is(err, safety.ErrDNSPreflightFailed) {
		return core.ExitCodeSafetyRefusal
	}

	// Validation Failures -> ExitCode 2
	if errors.Is(err, config.ErrConfigValidation) ||
		errors.Is(err, config.ErrInvalidSchemaVersion) ||
		errors.Is(err, config.ErrDuplicateYAMLKey) ||
		errors.Is(err, config.ErrMissingEnvVar) ||
		errors.Is(err, config.ErrInvalidEnvSyntax) {
		return core.ExitCodeValidationFailure
	}

	// Default fallback for unclassified runtime errors -> ExitCode 3
	return core.ExitCodeRuntimeFailure
}
