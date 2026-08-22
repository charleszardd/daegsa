package core

import "fmt"

// ExitCode represents the process exit code returned by DAEGSA CLI commands (§10).
type ExitCode int

const (
	// ExitCodeSuccess indicates the test completed successfully and all thresholds passed.
	ExitCodeSuccess ExitCode = 0

	// ExitCodeThresholdFailure indicates the test completed but one or more thresholds failed.
	ExitCodeThresholdFailure ExitCode = 1

	// ExitCodeValidationFailure indicates CLI usage or configuration validation failed before traffic started.
	ExitCodeValidationFailure ExitCode = 2

	// ExitCodeRuntimeFailure indicates an unrecoverable runtime error or tool failure prevented a valid test result.
	ExitCodeRuntimeFailure ExitCode = 3

	// ExitCodeSafetyRefusal indicates the safety preflight policy refused execution.
	ExitCodeSafetyRefusal ExitCode = 4
)

// String returns a short string identifier for the exit code.
func (c ExitCode) String() string {
	switch c {
	case ExitCodeSuccess:
		return "PASS"
	case ExitCodeThresholdFailure:
		return "FAIL_THRESHOLDS"
	case ExitCodeValidationFailure:
		return "VALIDATION_FAILURE"
	case ExitCodeRuntimeFailure:
		return "RUNTIME_FAILURE"
	case ExitCodeSafetyRefusal:
		return "SAFETY_REFUSAL"
	default:
		return fmt.Sprintf("EXIT_CODE_%d", int(c))
	}
}

// Description returns the detailed canonical specification meaning of the exit code (§10).
func (c ExitCode) Description() string {
	switch c {
	case ExitCodeSuccess:
		return "Test completed and all thresholds passed"
	case ExitCodeThresholdFailure:
		return "Test completed but one or more thresholds failed"
	case ExitCodeValidationFailure:
		return "CLI usage or configuration validation failed"
	case ExitCodeRuntimeFailure:
		return "Runtime/tool failure prevented a valid test result"
	case ExitCodeSafetyRefusal:
		return "Safety policy refused execution"
	default:
		return "Unknown exit code"
	}
}

// IsValid reports whether c is one of the 5 canonical exit codes.
func (c ExitCode) IsValid() bool {
	return c >= ExitCodeSuccess && c <= ExitCodeSafetyRefusal
}
