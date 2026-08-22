package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
)

func TestParseAndValidateYAML_ValidOpenConfig(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "schemas", "valid_open_config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read valid_open_config.yaml: %v", err)
	}

	cfg, err := config.ParseAndValidateYAML(data)
	if err != nil {
		t.Fatalf("unexpected validation error on valid open config: %v", err)
	}

	if cfg.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", cfg.SchemaVersion)
	}
	if cfg.Name != "donation-api-capacity" {
		t.Errorf("name = %q, want 'donation-api-capacity'", cfg.Name)
	}
	if cfg.Request.URL != "http://127.0.0.1:8080/api/donations" {
		t.Errorf("url = %q", cfg.Request.URL)
	}
	if cfg.Request.Method != "GET" {
		t.Errorf("method = %q, want 'GET'", cfg.Request.Method)
	}
	if cfg.Request.Timeout.Duration() != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", cfg.Request.Timeout)
	}
	if cfg.Load.Model != core.WorkloadModelOpen {
		t.Errorf("load.model = %s, want 'open'", cfg.Load.Model)
	}
	if cfg.Load.Rate != 100 {
		t.Errorf("load.rate = %v, want 100", cfg.Load.Rate)
	}
	if cfg.Load.TimeUnit.Duration() != 1*time.Second {
		t.Errorf("load.time_unit = %v, want 1s", cfg.Load.TimeUnit)
	}
	if cfg.Load.MaxInFlight != 500 {
		t.Errorf("load.max_in_flight = %d, want 500", cfg.Load.MaxInFlight)
	}
	if cfg.Load.Duration.Duration() != 30*time.Second {
		t.Errorf("load.duration = %v, want 30s", cfg.Load.Duration)
	}
	if cfg.Load.GracefulStop.Duration() != 10*time.Second {
		t.Errorf("load.graceful_stop = %v, want 10s", cfg.Load.GracefulStop)
	}
	if len(cfg.Thresholds) != 5 {
		t.Errorf("thresholds count = %d, want 5", len(cfg.Thresholds))
	}
}

func TestParseAndValidateYAML_ValidClosedConfig(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "schemas", "valid_closed_config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read valid_closed_config.yaml: %v", err)
	}

	cfg, err := config.ParseAndValidateYAML(data)
	if err != nil {
		t.Fatalf("unexpected validation error on valid closed config: %v", err)
	}

	if cfg.Load.Model != core.WorkloadModelClosed {
		t.Errorf("load.model = %s, want 'closed'", cfg.Load.Model)
	}
	if cfg.Load.Users != 50 {
		t.Errorf("load.users = %d, want 50", cfg.Load.Users)
	}
	if cfg.Load.ThinkTime.Duration() != 250*time.Millisecond {
		t.Errorf("load.think_time = %v, want 250ms", cfg.Load.ThinkTime)
	}
	if cfg.RateLimit.Treat429AsExpected != true {
		t.Errorf("rate_limit.treat_429_as_expected = %v, want true", cfg.RateLimit.Treat429AsExpected)
	}
}

func TestParseAndValidateYAML_RejectsUnknownFields(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "schemas", "invalid_configs", "unknown_field.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read unknown_field.yaml: %v", err)
	}

	_, err = config.ParseAndValidateYAML(data)
	if err == nil {
		t.Fatalf("expected error on config with unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "unrecognized_property") && !strings.Contains(err.Error(), "field") {
		t.Errorf("expected error mentioning unknown field, got: %v", err)
	}
}

func TestParseAndValidateYAML_RejectsDuplicateKeys(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "schemas", "invalid_configs", "duplicate_keys.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read duplicate_keys.yaml: %v", err)
	}

	_, err = config.ParseAndValidateYAML(data)
	if err == nil {
		t.Fatalf("expected error on config with duplicate keys, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected error mentioning duplicate key, got: %v", err)
	}
}

func TestParseAndValidateYAML_RejectsMixedModels(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "schemas", "invalid_configs", "mixed_models.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read mixed_models.yaml: %v", err)
	}

	_, err = config.ParseAndValidateYAML(data)
	if err == nil {
		t.Fatalf("expected error on mixed open/closed model config, got nil")
	}
	if !strings.Contains(err.Error(), "users") {
		t.Errorf("expected error mentioning incompatible users field, got: %v", err)
	}
}

func TestParseAndValidateYAML_RejectsBadSchemaVersion(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "schemas", "invalid_configs", "bad_version.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read bad_version.yaml: %v", err)
	}

	_, err = config.ParseAndValidateYAML(data)
	if err == nil {
		t.Fatalf("expected error on unsupported schema version, got nil")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("expected error mentioning schema_version, got: %v", err)
	}
}

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{input: "1024", want: 1024, wantErr: false},
		{input: "1024B", want: 1024, wantErr: false},
		{input: "1KB", want: 1000, wantErr: false},
		{input: "1KiB", want: 1024, wantErr: false},
		{input: "500KB", want: 500000, wantErr: false},
		{input: "1MB", want: 1000000, wantErr: false},
		{input: "1MiB", want: 1048576, wantErr: false},
		{input: "10MiB", want: 10485760, wantErr: false},
		{input: "1GiB", want: 1073741824, wantErr: false},
		{input: "0B", want: 0, wantErr: false},
		{input: "", want: 0, wantErr: true},
		{input: "invalid", want: 0, wantErr: true},
		{input: "-10MB", want: 0, wantErr: true},
		{input: "100UnknownUnit", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := config.ParseByteSize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseByteSize(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseByteSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseAndValidateYAML_ExampleConfigs(t *testing.T) {
	exampleFiles := []string{
		filepath.Join("..", "..", "examples", "open-api-capacity.yaml"),
		filepath.Join("..", "..", "examples", "closed-api-smoke.yaml"),
	}

	for _, file := range exampleFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			rawBytes, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("failed to read %s: %v", file, err)
			}

			expanded, err := config.ExpandEnv(rawBytes, func(k string) string {
				if k == "TARGET_URL" {
					return "http://127.0.0.1:8080"
				}
				return ""
			})
			if err != nil {
				t.Fatalf("failed to expand env in %s: %v", file, err)
			}

			cfg, err := config.ParseAndValidateYAML(expanded)
			if err != nil {
				t.Fatalf("failed to parse and validate %s: %v", file, err)
			}

			if cfg.SchemaVersion != 1 {
				t.Errorf("schema_version = %d, want 1", cfg.SchemaVersion)
			}
			if len(cfg.Thresholds) == 0 {
				t.Errorf("expected thresholds in %s, got 0", file)
			}
		})
	}
}
