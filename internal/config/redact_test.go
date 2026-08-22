package config

import (
	"errors"
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
		"X-Credential":        "credential_9999",
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
		"X-Credential",
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

func TestRedactString(t *testing.T) {
	knownSecrets := []string{"SUPER_SECRET_TOKEN_999", "SECRET_PASS_123"}

	tests := []struct {
		name         string
		input        string
		knownSecrets []string
		mustNotHave  []string
		mustHave     string
	}{
		{
			name:         "bearer token in error message",
			input:        "HTTP 401 Unauthorized: Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.t-ID",
			knownSecrets: nil,
			mustNotHave:  []string{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.t-ID"},
			mustHave:     "Bearer [REDACTED]",
		},
		{
			name:         "basic auth in error log",
			input:        "Connection failed to endpoint with Basic dXNlcm5hbWU6cGFzc3dvcmQxMjM=",
			knownSecrets: nil,
			mustNotHave:  []string{"dXNlcm5hbWU6cGFzc3dvcmQxMjM="},
			mustHave:     "Basic [REDACTED]",
		},
		{
			name:         "known secret in stack trace or plain string",
			input:        "Failed to authenticate with token SUPER_SECRET_TOKEN_999 or password SECRET_PASS_123",
			knownSecrets: knownSecrets,
			mustNotHave:  []string{"SUPER_SECRET_TOKEN_999", "SECRET_PASS_123"},
			mustHave:     "[REDACTED]",
		},
		{
			name:         "sensitive query params in error string",
			input:        "Get https://api.internal/v1/auth?token=my_secret_token_abc&apikey=secret_key_123: dial tcp timeout",
			knownSecrets: nil,
			mustNotHave:  []string{"my_secret_token_abc", "secret_key_123"},
			mustHave:     "token=[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactString(tt.input, tt.knownSecrets)
			for _, prohibited := range tt.mustNotHave {
				if strings.Contains(got, prohibited) {
					t.Errorf("secret %q leaked in redacted string: %q", prohibited, got)
				}
			}
			if !strings.Contains(got, tt.mustHave) {
				t.Errorf("expected string to contain %q, got %q", tt.mustHave, got)
			}
		})
	}
}

func TestRedactError(t *testing.T) {
	err := RedactError(nil, nil)
	if err != nil {
		t.Errorf("expected nil for nil error, got %v", err)
	}

	rawErr := strings.NewReader("dummy") // just getting an error
	_ = rawErr
	origErr := &netError{msg: "connection refused with Bearer secret-raw-token and SECRET_KNOWN_XYZ"}
	redactedErr := RedactError(origErr, []string{"SECRET_KNOWN_XYZ"})
	if redactedErr == nil {
		t.Fatalf("expected non-nil error")
	}

	errMsg := redactedErr.Error()
	if strings.Contains(errMsg, "secret-raw-token") || strings.Contains(errMsg, "SECRET_KNOWN_XYZ") {
		t.Errorf("secrets leaked in error string: %q", errMsg)
	}
	if !strings.Contains(errMsg, "Bearer [REDACTED]") {
		t.Errorf("expected Bearer [REDACTED] in error string, got %q", errMsg)
	}
	if unwrapped := errors.Unwrap(redactedErr); unwrapped != nil {
		t.Fatalf("redacted error exposed its original error: %q", unwrapped.Error())
	}
}

type netError struct {
	msg string
}

func (e *netError) Error() string {
	return e.msg
}

func TestRedactURL_MalformedURLRedactsSensitiveQuery(t *testing.T) {
	const secret = "SECRET_QUERY_ZETA_444"
	malformedURL := "http://127.0.0.1/%zz?token=" + secret

	redacted := RedactURL(malformedURL)
	if strings.Contains(redacted, secret) {
		t.Fatalf("malformed URL redaction leaked secret: %s", redacted)
	}
	if redacted != RedactedPlaceholder {
		t.Errorf("malformed URL redaction = %q, want %q", redacted, RedactedPlaceholder)
	}
}

func TestValidateRequestConfig_InvalidURLRedactsSensitiveQuery(t *testing.T) {
	const secret = "SECRET_QUERY_ZETA_444"
	requestConfig := RequestConfig{
		URL:    "http://127.0.0.1/%zz?token=" + secret,
		Method: http.MethodGet,
	}

	err := validateRequestConfig(&requestConfig)
	if err == nil {
		t.Fatal("expected malformed URL validation error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("URL validation error leaked secret: %v", err)
	}
	if !strings.Contains(err.Error(), RedactedPlaceholder) {
		t.Errorf("URL validation error = %q, want redaction placeholder", err)
	}
}

func TestRedactURL_MalformedSensitiveComponents(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		prohibited string
	}{
		{
			name:       "compound client secret query key",
			input:      "http://127.0.0.1/%zz?client_secret=SECRET_CLIENT_333&page=1",
			prohibited: "SECRET_CLIENT_333",
		},
		{
			name:       "compound api token query key",
			input:      "http://127.0.0.1/%zz?page=1&api_token=SECRET_API_TOKEN_222",
			prohibited: "SECRET_API_TOKEN_222",
		},
		{
			name:       "URL userinfo password",
			input:      "http://load-user:SECRET_PASS_BETA_888@127.0.0.1/%zz",
			prohibited: "SECRET_PASS_BETA_888",
		},
		{
			name:       "URL username-only userinfo",
			input:      "http://SECRET_USER_ALPHA_111@127.0.0.1/%zz",
			prohibited: "SECRET_USER_ALPHA_111",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			redacted := RedactURL(testCase.input)
			if strings.Contains(redacted, testCase.prohibited) {
				t.Fatalf("malformed URL redaction leaked %q: %s", testCase.prohibited, redacted)
			}
			if !strings.Contains(redacted, RedactedPlaceholder) {
				t.Errorf("malformed URL redaction = %q, want redaction placeholder", redacted)
			}
		})
	}
}

func TestRedactString_URLContextDoesNotOverRedactArbitraryProse(t *testing.T) {
	const prose = "the documented field is client_secret=example-placeholder"
	if got := RedactString(prose, nil); got != prose {
		t.Errorf("RedactString() changed non-URL prose: got %q, want %q", got, prose)
	}
}
