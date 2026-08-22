package config

import (
	"net/http"
	"strings"
	"testing"
)

func TestRedactHeaders(t *testing.T) {
	headers := map[string]string{
		"Authorization":       "Bearer super-secret-jwt-token",
		"Proxy-Authorization": "Basic dXNlcjpwYXNz",
		"Cookie":              "session_id=abcdef123456",
		"Set-Cookie":          "tracking=xyz; Path=/",
		"X-Api-Key":           "key_live_9999",
		"X-Auth-Token":        "token_8888",
		"Content-Type":        "application/json",
		"Accept":              "application/json",
		"User-Agent":          "daegsa/v0.1",
	}

	redacted := RedactHeaders(headers)

	// Sensitive keys must be masked
	sensitiveKeys := []string{
		"Authorization",
		"Proxy-Authorization",
		"Cookie",
		"Set-Cookie",
		"X-Api-Key",
		"X-Auth-Token",
	}
	for _, k := range sensitiveKeys {
		if val, ok := redacted[k]; !ok || val != RedactedPlaceholder {
			t.Errorf("expected header %q to be %q, got %q", k, RedactedPlaceholder, val)
		}
	}

	// Safe keys must be preserved
	if redacted["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type preserved, got %q", redacted["Content-Type"])
	}
	if redacted["Accept"] != "application/json" {
		t.Errorf("expected Accept preserved, got %q", redacted["Accept"])
	}
	if redacted["User-Agent"] != "daegsa/v0.1" {
		t.Errorf("expected User-Agent preserved, got %q", redacted["User-Agent"])
	}
}

func TestRedactHTTPHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer secret")
	h.Set("Content-Type", "application/json")
	h.Add("Cookie", "a=1")
	h.Add("Cookie", "b=2")

	redacted := RedactHTTPHeaders(h)
	if redacted.Get("Authorization") != RedactedPlaceholder {
		t.Errorf("expected Authorization redacted, got %q", redacted.Get("Authorization"))
	}
	if redacted.Get("Cookie") != RedactedPlaceholder {
		t.Errorf("expected Cookie redacted, got %q", redacted.Get("Cookie"))
	}
	if redacted.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type preserved, got %q", redacted.Get("Content-Type"))
	}
}

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic URL no secrets",
			input:    "https://api.example.com/v1/users",
			expected: "https://api.example.com/v1/users",
		},
		{
			name:     "userinfo in URL",
			input:    "https://admin:supersecret@api.example.com/data",
			expected: "https://%5BREDACTED%5D:%5BREDACTED%5D@api.example.com/data",
		},
		{
			name:     "sensitive query parameters",
			input:    "https://api.example.com/items?token=my_secret_token&page=2&api_key=key123",
			expected: "token=%5BREDACTED%5D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactURL(tt.input)
			if tt.name == "sensitive query parameters" {
				if !strings.Contains(got, "page=2") {
					t.Errorf("expected page=2 preserved, got %q", got)
				}
				if strings.Contains(got, "my_secret_token") || strings.Contains(got, "key123") {
					t.Errorf("secrets leaked in redacted URL: %q", got)
				}
				if !strings.Contains(got, "%5BREDACTED%5D") && !strings.Contains(got, "[REDACTED]") {
					t.Errorf("expected redacted parameter, got %q", got)
				}
			} else {
				if got != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, got)
				}
			}
		})
	}
}
