package config

import (
	"errors"
	"strings"
	"testing"
)

func TestExpandEnv_Basic(t *testing.T) {
	mockEnv := map[string]string{
		"BASE_URL":  "https://api.example.com",
		"API_TOKEN": "secret_token_123",
		"PORT":      "8080",
	}
	getenv := func(k string) string {
		return mockEnv[k]
	}

	input := []byte(`
schema_version: 1
name: test
request:
  url: ${BASE_URL}/v1/data
  headers:
    Authorization: Bearer ${API_TOKEN}
`)

	expanded, err := ExpandEnv(input, getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expStr := string(expanded)
	if !strings.Contains(expStr, "https://api.example.com/v1/data") {
		t.Errorf("expected expanded URL, got:\n%s", expStr)
	}
	if !strings.Contains(expStr, "Bearer secret_token_123") {
		t.Errorf("expected expanded Authorization header, got:\n%s", expStr)
	}
}

func TestExpandEnv_Escaping(t *testing.T) {
	mockEnv := map[string]string{
		"VAR": "replaced_value",
	}
	getenv := func(k string) string {
		return mockEnv[k]
	}

	input := []byte(`
literal: $${VAR}
expanded: ${VAR}
double_escape: $$$${VAR}
`)

	expanded, err := ExpandEnv(input, getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expStr := string(expanded)
	if !strings.Contains(expStr, "literal: ${VAR}") {
		t.Errorf("expected escaped ${VAR}, got:\n%s", expStr)
	}
	if !strings.Contains(expStr, "expanded: replaced_value") {
		t.Errorf("expected expanded value, got:\n%s", expStr)
	}
	if !strings.Contains(expStr, "double_escape: $${VAR}") {
		t.Errorf("expected double escape, got:\n%s", expStr)
	}
}

func TestExpandEnv_MissingVar(t *testing.T) {
	getenv := func(k string) string {
		return ""
	}

	input := []byte(`url: ${UNDEFINED_VAR}/api`)
	_, err := ExpandEnv(input, getenv)
	if err == nil {
		t.Fatalf("expected error for missing environment variable, got nil")
	}
	if !errors.Is(err, ErrMissingEnvVar) {
		t.Errorf("expected ErrMissingEnvVar, got %v", err)
	}
}

func TestExpandEnv_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty placeholder", "url: ${}"},
		{"digit start", "url: ${123VAR}"},
		{"unclosed placeholder", "url: ${UNCLOSED"},
		{"unclosed escape", "url: $${UNCLOSED"},
		{"invalid char", "url: ${VAR-NAME}"},
	}

	getenv := func(k string) string { return "val" }

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExpandEnv([]byte(tt.input), getenv)
			if err == nil {
				t.Errorf("expected syntax error for %q, got nil", tt.input)
			}
			if !errors.Is(err, ErrInvalidEnvSyntax) {
				t.Errorf("expected ErrInvalidEnvSyntax, got %v", err)
			}
		})
	}
}

func TestExpandEnv_MultipleOnSameLine(t *testing.T) {
	mockEnv := map[string]string{
		"HOST": "127.0.0.1",
		"PORT": "9090",
	}
	getenv := func(k string) string { return mockEnv[k] }

	input := []byte(`url: http://${HOST}:${PORT}/health`)
	expanded, err := ExpandEnv(input, getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(expanded) != "url: http://127.0.0.1:9090/health" {
		t.Errorf("expected expanded multi-var string, got: %s", string(expanded))
	}
}
