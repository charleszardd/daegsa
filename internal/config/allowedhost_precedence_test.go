package config

import (
	"reflect"
	"testing"
)

func TestApplyCLIOverridesAllowedHostsReplaceYAMLAndDefensivelyCopy(t *testing.T) {
	cfg, err := ParseAndValidateYAML([]byte(`
schema_version: 1
name: allowed-host-precedence
request:
  url: https://api.example.com/items
  method: GET
load:
  model: open
  rate: 10
  duration: 1s
  max_in_flight: 5
safety:
  allowed_hosts: [yaml.example.com]
`))
	if err != nil {
		t.Fatalf("ParseAndValidateYAML() error = %v", err)
	}

	providedHosts := []string{"API.EXAMPLE.COM.", "backup.example.com"}
	flags := &CLIFlags{AllowedHosts: providedHosts}
	if err := ApplyCLIOverrides(cfg, flags); err != nil {
		t.Fatalf("ApplyCLIOverrides() error = %v", err)
	}

	wantHosts := []string{"api.example.com", "backup.example.com"}
	if !reflect.DeepEqual(cfg.Safety.AllowedHosts, wantHosts) {
		t.Fatalf("allowed hosts = %v, want %v", cfg.Safety.AllowedHosts, wantHosts)
	}
	providedHosts[0] = "mutated.example.com"
	flags.AllowedHosts[1] = "also-mutated.example.com"
	if !reflect.DeepEqual(cfg.Safety.AllowedHosts, wantHosts) {
		t.Fatalf("config allowed hosts mutated through caller slice: %v", cfg.Safety.AllowedHosts)
	}
}

func TestApplyCLIOverridesRejectsWildcardAllowedHost(t *testing.T) {
	cfg, err := ParseAndValidateYAML([]byte(`
schema_version: 1
name: wildcard-rejection
request:
  url: https://api.example.com/items
  method: GET
load:
  model: open
  rate: 10
  duration: 1s
  max_in_flight: 5
`))
	if err != nil {
		t.Fatalf("ParseAndValidateYAML() error = %v", err)
	}

	if err := ApplyCLIOverrides(cfg, &CLIFlags{AllowedHosts: []string{"*.example.com"}}); err == nil {
		t.Fatal("ApplyCLIOverrides() unexpectedly accepted a CLI wildcard")
	}
}

func TestApplyCLIOverridesAddsOnlyExplicitLoopbackConvenience(t *testing.T) {
	tests := []struct {
		name      string
		targetURL string
		wantHosts []string
	}{
		{name: "localhost", targetURL: "http://localhost:8080/items", wantHosts: []string{"localhost"}},
		{name: "IPv4 loopback", targetURL: "http://127.1.2.3:8080/items", wantHosts: []string{"127.1.2.3"}},
		{name: "IPv6 loopback", targetURL: "http://[::1]:8080/items", wantHosts: []string{"::1"}},
		{name: "external remains unauthorized", targetURL: "https://api.example.com/items", wantHosts: nil},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := &Config{SchemaVersion: LegacySchemaVersion, Name: "cli-execution"}
			flags := &CLIFlags{URL: testCase.targetURL}
			if err := ApplyCLIOverrides(cfg, flags); err != nil {
				t.Fatalf("ApplyCLIOverrides() error = %v", err)
			}
			if !reflect.DeepEqual(cfg.Safety.AllowedHosts, testCase.wantHosts) {
				t.Fatalf("allowed hosts = %v, want %v", cfg.Safety.AllowedHosts, testCase.wantHosts)
			}
		})
	}
}

func TestApplyCLIOverridesDoesNotAddLoopbackForConfigFile(t *testing.T) {
	cfg := &Config{SchemaVersion: LegacySchemaVersion, Name: "configured"}
	flags := &CLIFlags{ConfigFile: "test.yaml", URL: "http://127.0.0.1:8080/items"}
	if err := ApplyCLIOverrides(cfg, flags); err != nil {
		t.Fatalf("ApplyCLIOverrides() error = %v", err)
	}
	if len(cfg.Safety.AllowedHosts) != 0 {
		t.Fatalf("configured run unexpectedly received implicit allowed hosts: %v", cfg.Safety.AllowedHosts)
	}
}
