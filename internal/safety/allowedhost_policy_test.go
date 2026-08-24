package safety

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
)

func TestEmptyAllowlistRefusesExternalHost(t *testing.T) {
	if IsHostAllowed("api.example.com", nil) {
		t.Fatal("empty allowlist unexpectedly authorized an external host")
	}
}

func TestHostAllowlistCanonicalMatching(t *testing.T) {
	tests := []struct {
		name         string
		targetHost   string
		allowedHosts []string
		want         bool
	}{
		{name: "case and terminal dot", targetHost: "API.EXAMPLE.COM.", allowedHosts: []string{"api.example.com"}, want: true},
		{name: "IPv6", targetHost: "::1", allowedHosts: []string{"0:0:0:0:0:0:0:1"}, want: true},
		{name: "candidate port rejected", targetHost: "api.example.com:443", allowedHosts: []string{"api.example.com"}, want: false},
		{name: "malformed allowlist entry ignored", targetHost: "api.example.com", allowedHosts: []string{"https://api.example.com"}, want: false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := IsHostAllowed(testCase.targetHost, testCase.allowedHosts); got != testCase.want {
				t.Fatalf("IsHostAllowed() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestPreflightScenarioRequiresEveryStepHost(t *testing.T) {
	cfg := &config.Config{
		SchemaVersion: config.LegacySchemaVersion,
		Name:          "scenario-allowlist",
		Scenario: &config.ScenarioConfig{
			Name: "cross-host",
			Steps: []config.StepConfig{
				{Name: "allowed", URL: "https://api.example.com/one", Method: "GET"},
				{Name: "refused", URL: "https://other.example.com/two", Method: "GET"},
			},
		},
		Load: config.LoadConfig{
			Model:    core.WorkloadModelClosed,
			Users:    1,
			Duration: config.Duration(time.Second),
		},
		Safety: config.SafetyConfig{AllowedHosts: []string{"api.example.com"}},
	}

	_, err := NewPreflightEngine().Check(context.Background(), cfg, SafetyFlags{SkipDNSPreflight: true})
	if !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("Check() error = %v, want ErrHostNotAllowed", err)
	}
}
