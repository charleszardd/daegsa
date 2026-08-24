package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/testtarget"
)

func TestFlagValuesAllowedHostsDefensiveCopy(t *testing.T) {
	values := &flagValues{allowedHosts: []string{"api.example.com"}}
	flags := values.toCLIFlags()
	values.allowedHosts[0] = "mutated.example.com"
	if flags.AllowedHosts[0] != "api.example.com" {
		t.Fatalf("CLIFlags allowed hosts mutated through flagValues: %v", flags.AllowedHosts)
	}
}

func TestCLIAllowedHostIsRepeatableAndReplacesYAML(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "allowed-host.yaml")
	manifest := fmt.Sprintf(`
schema_version: 1
name: cli-allowed-host
request:
  url: %s
  method: GET
load:
  model: open
  rate: 1
  duration: 1s
  max_in_flight: 1
safety:
  allowed_hosts: [yaml.example.com]
`, server.URL())
	if err := os.WriteFile(configPath, []byte(manifest), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := executeContext(context.Background(), []string{
		"run", "--config", configPath, "--dry-run",
		"--allowed-host", "LOCALHOST.",
		"--allowed-host", "127.0.0.1",
	}, &stdout, &stderr)
	if code != core.ExitCodeSuccess {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Allowed Hosts:        [localhost 127.0.0.1]") {
		t.Fatalf("dry-run output missing canonical repeated hosts:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "yaml.example.com") {
		t.Fatalf("YAML allowlist was not replaced:\n%s", stdout.String())
	}
}

func TestCLIExternalURLRequiresAllowedHost(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := executeContext(context.Background(), []string{
		"run", "--url", "https://api.example.com/items", "--dry-run",
	}, &stdout, &stderr)
	if code != core.ExitCodeSafetyRefusal {
		t.Fatalf("exit code = %d, want %d; stderr = %s", code, core.ExitCodeSafetyRefusal, stderr.String())
	}
}

func TestCLIAllowedHostRejectsWildcard(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := executeContext(context.Background(), []string{
		"run", "--url", "https://api.example.com/items", "--allowed-host", "*.example.com", "--dry-run",
	}, &stdout, &stderr)
	if code != core.ExitCodeValidationFailure {
		t.Fatalf("exit code = %d, want %d; stderr = %s", code, core.ExitCodeValidationFailure, stderr.String())
	}
}

func TestCLIHelpDocumentsAllowedHost(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := executeContext(context.Background(), []string{"run", "--help"}, &stdout, &stderr)
	if code != core.ExitCodeSuccess {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "--allowed-host") || !strings.Contains(stdout.String(), "repeatable") {
		t.Fatalf("run help does not document repeatable --allowed-host:\n%s", stdout.String())
	}
}
