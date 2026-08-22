package core_test

import (
	"testing"

	"github.com/charleszardd/daegsa/internal/core"
)

func TestExitCodes_ValuesAndDescriptions(t *testing.T) {
	tests := []struct {
		code        core.ExitCode
		intValue    int
		name        string
		description string
		valid       bool
	}{
		{
			code:        core.ExitCodeSuccess,
			intValue:    0,
			name:        "PASS",
			description: "Test completed and all thresholds passed",
			valid:       true,
		},
		{
			code:        core.ExitCodeThresholdFailure,
			intValue:    1,
			name:        "FAIL_THRESHOLDS",
			description: "Test completed but one or more thresholds failed",
			valid:       true,
		},
		{
			code:        core.ExitCodeValidationFailure,
			intValue:    2,
			name:        "VALIDATION_FAILURE",
			description: "CLI usage or configuration validation failed",
			valid:       true,
		},
		{
			code:        core.ExitCodeRuntimeFailure,
			intValue:    3,
			name:        "RUNTIME_FAILURE",
			description: "Runtime/tool failure prevented a valid test result",
			valid:       true,
		},
		{
			code:        core.ExitCodeSafetyRefusal,
			intValue:    4,
			name:        "SAFETY_REFUSAL",
			description: "Safety policy refused execution",
			valid:       true,
		},
		{
			code:        core.ExitCode(99),
			intValue:    99,
			name:        "EXIT_CODE_99",
			description: "Unknown exit code",
			valid:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.code) != tt.intValue {
				t.Errorf("ExitCode integer value = %d, want %d", int(tt.code), tt.intValue)
			}
			if tt.code.String() != tt.name {
				t.Errorf("ExitCode.String() = %q, want %q", tt.code.String(), tt.name)
			}
			if tt.code.Description() != tt.description {
				t.Errorf("ExitCode.Description() = %q, want %q", tt.code.Description(), tt.description)
			}
			if tt.code.IsValid() != tt.valid {
				t.Errorf("ExitCode.IsValid() = %v, want %v", tt.code.IsValid(), tt.valid)
			}
		})
	}
}
