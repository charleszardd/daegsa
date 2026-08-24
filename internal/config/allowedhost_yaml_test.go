package config

import (
	"reflect"
	"testing"
)

func TestParseAndValidateYAMLNormalizesDeclarativeAllowedHosts(t *testing.T) {
	cfg, err := ParseAndValidateYAML([]byte(`
schema_version: 1
name: normalized-yaml-hosts
request:
  url: https://api.example.com/items
  method: GET
load:
  model: open
  rate: 1
  duration: 1s
  max_in_flight: 1
safety:
  allowed_hosts: [API.EXAMPLE.COM., "*.Staging.Example.COM."]
`))
	if err != nil {
		t.Fatalf("ParseAndValidateYAML() error = %v", err)
	}
	want := []string{"api.example.com", "*.staging.example.com"}
	if !reflect.DeepEqual(cfg.Safety.AllowedHosts, want) {
		t.Fatalf("allowed hosts = %v, want %v", cfg.Safety.AllowedHosts, want)
	}
}

func TestParseAndValidateYAMLRejectsNonHostAllowlistEntry(t *testing.T) {
	_, err := ParseAndValidateYAML([]byte(`
schema_version: 1
name: invalid-yaml-host
request:
  url: https://api.example.com/items
  method: GET
load:
  model: open
  rate: 1
  duration: 1s
  max_in_flight: 1
safety:
  allowed_hosts: [https://api.example.com]
`))
	if err == nil {
		t.Fatal("ParseAndValidateYAML() unexpectedly accepted a URL as an allowed host")
	}
}
