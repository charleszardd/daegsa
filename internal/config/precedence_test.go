package config

import (
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
)

func TestApplyCLIOverrides_Precedence(t *testing.T) {
	yamlData := []byte(`
schema_version: 1
name: test-config
request:
  url: https://api.example.com/items
  method: GET
  timeout: 5s
load:
  model: open
  rate: 50
  duration: 20s
  max_in_flight: 200
`)

	cfg, err := ParseAndValidateYAML(yamlData)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}

	flags := &CLIFlags{
		URL:              "https://override.example.com/other",
		Method:           "POST",
		Rate:             150,
		Duration:         45 * time.Second,
		Timeout:          8 * time.Second,
		MaxInFlight:      600,
		AllowDestructive: true,
	}

	if err := ApplyCLIOverrides(cfg, flags); err != nil {
		t.Fatalf("failed to apply CLI overrides: %v", err)
	}

	if cfg.Request.URL != "https://override.example.com/other" {
		t.Errorf("expected overridden URL, got %s", cfg.Request.URL)
	}
	if cfg.Request.Method != "POST" {
		t.Errorf("expected overridden Method, got %s", cfg.Request.Method)
	}
	if cfg.Load.Rate != 150 {
		t.Errorf("expected overridden Rate 150, got %v", cfg.Load.Rate)
	}
	if cfg.Load.Duration.Duration() != 45*time.Second {
		t.Errorf("expected overridden Duration 45s, got %v", cfg.Load.Duration)
	}
	if cfg.Request.Timeout.Duration() != 8*time.Second {
		t.Errorf("expected overridden Timeout 8s, got %v", cfg.Request.Timeout)
	}
	if cfg.Load.MaxInFlight != 600 {
		t.Errorf("expected overridden MaxInFlight 600, got %d", cfg.Load.MaxInFlight)
	}
	if !cfg.Safety.AllowNonIdempotent {
		t.Errorf("expected AllowNonIdempotent to be true after --allow-destructive")
	}
}

func TestApplyCLIOverrides_ModelSwitch(t *testing.T) {
	// Start with open model
	yamlData := []byte(`
schema_version: 1
name: test-config
request:
  url: https://api.example.com/items
  method: GET
load:
  model: open
  rate: 50
  duration: 20s
  max_in_flight: 200
`)

	cfg, err := ParseAndValidateYAML(yamlData)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}

	// Switch to closed model
	flags := &CLIFlags{
		Model: core.WorkloadModelClosed,
		Users: 25,
	}

	if err := ApplyCLIOverrides(cfg, flags); err != nil {
		t.Fatalf("failed to switch model to closed: %v", err)
	}

	if cfg.Load.Model != core.WorkloadModelClosed {
		t.Errorf("expected model closed, got %s", cfg.Load.Model)
	}
	if cfg.Load.Users != 25 {
		t.Errorf("expected users 25, got %d", cfg.Load.Users)
	}
	if cfg.Load.Rate != 0 {
		t.Errorf("expected rate to be zeroed in closed model, got %v", cfg.Load.Rate)
	}
	if cfg.Load.MaxInFlight != 0 {
		t.Errorf("expected max_in_flight to be zeroed in closed model, got %d", cfg.Load.MaxInFlight)
	}
}

func TestApplyCLIOverrides_InvalidOverrides(t *testing.T) {
	yamlData := []byte(`
schema_version: 1
name: test-config
request:
  url: https://api.example.com/items
  method: GET
load:
  model: open
  rate: 50
  duration: 20s
  max_in_flight: 200
`)

	cfg, err := ParseAndValidateYAML(yamlData)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}

	// Invalid URL override
	flags := &CLIFlags{
		URL: "not-a-valid-url",
	}

	err = ApplyCLIOverrides(cfg, flags)
	if err == nil {
		t.Fatalf("expected validation error on invalid URL override, got nil")
	}
}

func TestApplyCLIOverrides_ResponseBodyLimitAndRedirects(t *testing.T) {
	yamlData := []byte(`
schema_version: 1
name: test-config
request:
  url: https://api.example.com/items
  method: GET
  response_body_limit: 500KB
  redirects: same-origin
load:
  model: open
  rate: 10
  duration: 10s
  max_in_flight: 50
`)

	cfg, err := ParseAndValidateYAML(yamlData)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}

	flags := &CLIFlags{
		ResponseBodyLimit: "2MiB",
		Redirects:         "none",
	}

	if err := ApplyCLIOverrides(cfg, flags); err != nil {
		t.Fatalf("failed to apply overrides: %v", err)
	}

	if cfg.Request.ResponseBodyLimit != "2MiB" {
		t.Errorf("expected ResponseBodyLimit '2MiB', got %q", cfg.Request.ResponseBodyLimit)
	}
	if cfg.Request.Redirects != "none" {
		t.Errorf("expected Redirects 'none', got %q", cfg.Request.Redirects)
	}
}

func TestApplyCLIOverrides_SegmentConflicts(t *testing.T) {
	yamlData := []byte(`
schema_version: 2
name: profile-test
request:
  url: https://api.example.com/items
  method: GET
load:
  model: open
  time_unit: 1s
  max_in_flight: 50
  segments:
    - {name: m, stage: measured, duration: 10s, rate: 20}
`)

	cfg1, err := ParseAndValidateYAML(yamlData)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyCLIOverrides(cfg1, &CLIFlags{Rate: 100}); err == nil {
		t.Error("expected error when overriding segmented profile with --rate")
	}

	cfg2, err := ParseAndValidateYAML(yamlData)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyCLIOverrides(cfg2, &CLIFlags{Duration: 30 * time.Second}); err == nil {
		t.Error("expected error when overriding segmented profile with --duration")
	}
}
