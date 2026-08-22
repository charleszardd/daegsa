package auth_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/charleszardd/daegsa/internal/auth"
)

func TestRedact_HeadersAndParams(t *testing.T) {
	headers := map[string]string{
		"Authorization":       "Bearer my-secret-jwt",
		"Proxy-Authorization": "Basic dXNlcjpwYXNz",
		"X-Api-Key":           "key_live_12345",
		"X-Auth-Token":        "token_abcdef",
		"Cookie":              "session=token123",
		"Set-Cookie":          "id=token456; Secure",
		"Content-Type":        "application/json",
		"X-Custom-Secret":     "sensitive_value",
	}

	redacted := auth.RedactHeaders(headers)
	sensitiveKeys := []string{
		"Authorization",
		"Proxy-Authorization",
		"X-Api-Key",
		"X-Auth-Token",
		"Cookie",
		"Set-Cookie",
		"X-Custom-Secret",
	}

	for _, k := range sensitiveKeys {
		if val, ok := redacted[k]; !ok || val != auth.RedactedPlaceholder {
			t.Errorf("header %q expected %q, got %q", k, auth.RedactedPlaceholder, val)
		}
	}

	if redacted["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type preserved, got %q", redacted["Content-Type"])
	}

	// HTTP headers
	httpHeaders := http.Header{}
	for k, v := range headers {
		httpHeaders.Set(k, v)
	}
	redactedHTTP := auth.RedactHTTPHeaders(httpHeaders)
	for _, k := range sensitiveKeys {
		if val := redactedHTTP.Get(k); val != auth.RedactedPlaceholder {
			t.Errorf("http.Header %q expected %q, got %q", k, auth.RedactedPlaceholder, val)
		}
	}
}

func TestRedact_URL(t *testing.T) {
	raw := "https://user:password123@api.internal.net/items?token=tok_999&auth=auth_888&page=5"
	redacted := auth.RedactURL(raw)

	if strings.Contains(redacted, "password123") || strings.Contains(redacted, "tok_999") || strings.Contains(redacted, "auth_888") {
		t.Errorf("secret leaked in redacted URL: %q", redacted)
	}
	if !strings.Contains(redacted, "page=5") {
		t.Errorf("expected non-sensitive param page=5 preserved, got %q", redacted)
	}
}

func TestRedact_StringAndError(t *testing.T) {
	knownSecrets := []string{"CRITICAL_SECRET_111", "PASSPHRASE_222"}

	rawErr := errors.New("failed to connect to host with Bearer token_abc_123 and CRITICAL_SECRET_111")
	scrubbedErr := auth.RedactError(rawErr, knownSecrets)

	if scrubbedErr == nil {
		t.Fatalf("expected non-nil error")
	}

	msg := scrubbedErr.Error()
	if strings.Contains(msg, "token_abc_123") || strings.Contains(msg, "CRITICAL_SECRET_111") {
		t.Errorf("secrets leaked in scrubbed error: %q", msg)
	}
	if !strings.Contains(msg, "Bearer "+auth.RedactedPlaceholder) {
		t.Errorf("expected Bearer [REDACTED] in scrubbed error: %q", msg)
	}
}
