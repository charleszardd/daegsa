package auth_test

import (
	"net/http"
	"testing"

	"github.com/charleszardd/daegsa/internal/auth"
	"github.com/charleszardd/daegsa/internal/config"
)

func TestRequestAuthenticator_Bearer(t *testing.T) {
	cfg := &config.AuthConfig{
		Type:  auth.AuthTypeBearer,
		Token: "my-jwt-token-xyz",
	}

	authn, err := auth.NewAuthenticator(cfg)
	if err != nil {
		t.Fatalf("failed to create authenticator: %v", err)
	}

	if authn.AuthMode() != auth.AuthTypeBearer {
		t.Errorf("expected AuthMode 'bearer', got %q", authn.AuthMode())
	}
	if authn.TokenCount() != 1 {
		t.Errorf("expected TokenCount 1, got %d", authn.TokenCount())
	}

	req, _ := http.NewRequest("GET", "http://example.com/api", nil)
	authn.AuthenticateRequest(req, 0)

	got := req.Header.Get("Authorization")
	want := "Bearer my-jwt-token-xyz"
	if got != want {
		t.Errorf("header Authorization: got %q, want %q", got, want)
	}
}

func TestRequestAuthenticator_CustomHeader(t *testing.T) {
	cfg := &config.AuthConfig{
		Type:       auth.AuthTypeCustomHeader,
		Token:      "api-key-9999",
		HeaderName: "X-API-Key",
	}

	authn, err := auth.NewAuthenticator(cfg)
	if err != nil {
		t.Fatalf("failed to create authenticator: %v", err)
	}

	if authn.AuthMode() != auth.AuthTypeCustomHeader {
		t.Errorf("expected AuthMode 'custom_header', got %q", authn.AuthMode())
	}

	req, _ := http.NewRequest("GET", "http://example.com/api", nil)
	authn.AuthenticateRequest(req, 0)

	if got := req.Header.Get("X-API-Key"); got != "api-key-9999" {
		t.Errorf("header X-API-Key: got %q, want 'api-key-9999'", got)
	}
	if req.Header.Get("Authorization") != "" {
		t.Errorf("expected no Authorization header")
	}
}

func TestRequestAuthenticator_Basic(t *testing.T) {
	cfg := &config.AuthConfig{
		Type:     auth.AuthTypeBasic,
		Username: "admin_user",
		Password: "secret_password_123",
	}

	authn, err := auth.NewAuthenticator(cfg)
	if err != nil {
		t.Fatalf("failed to create authenticator: %v", err)
	}

	req, _ := http.NewRequest("GET", "http://example.com/api", nil)
	authn.AuthenticateRequest(req, 0)

	user, pass, ok := req.BasicAuth()
	if !ok {
		t.Fatalf("failed to extract Basic auth credentials from request")
	}
	if user != "admin_user" || pass != "secret_password_123" {
		t.Errorf("basic auth credentials mismatch: got (%q, %q), want ('admin_user', 'secret_password_123')", user, pass)
	}
}

func TestRequestAuthenticator_TokenPool(t *testing.T) {
	cfg := &config.AuthConfig{
		Type:      auth.AuthTypeTokenPool,
		TokenPool: []string{"token-alpha", "token-beta", "token-gamma"},
	}

	authn, err := auth.NewAuthenticator(cfg)
	if err != nil {
		t.Fatalf("failed to create authenticator: %v", err)
	}

	if authn.TokenCount() != 3 {
		t.Errorf("expected TokenCount 3, got %d", authn.TokenCount())
	}

	expectedTokens := []string{"token-alpha", "token-beta", "token-gamma"}
	for workerID := 0; workerID < 6; workerID++ {
		req, _ := http.NewRequest("GET", "http://example.com/api", nil)
		authn.AuthenticateRequest(req, workerID)

		got := req.Header.Get("Authorization")
		want := "Bearer " + expectedTokens[workerID%3]
		if got != want {
			t.Errorf("worker %d: got %q, want %q", workerID, got, want)
		}
	}

	// Custom header token pool
	cfgCustom := &config.AuthConfig{
		Type:       auth.AuthTypeTokenPool,
		TokenPool:  []string{"k1", "k2"},
		HeaderName: "X-Auth-Token",
	}
	authnCustom, _ := auth.NewAuthenticator(cfgCustom)
	reqCustom, _ := http.NewRequest("GET", "http://example.com/api", nil)
	authnCustom.AuthenticateRequest(reqCustom, 1)
	if got := reqCustom.Header.Get("X-Auth-Token"); got != "k2" {
		t.Errorf("expected X-Auth-Token 'k2', got %q", got)
	}
}

func TestRequestAuthenticator_None(t *testing.T) {
	cfg := &config.AuthConfig{
		Type: auth.AuthTypeNone,
	}

	authn, err := auth.NewAuthenticator(cfg)
	if err != nil {
		t.Fatalf("failed to create authenticator: %v", err)
	}

	req, _ := http.NewRequest("GET", "http://example.com/api", nil)
	authn.AuthenticateRequest(req, 0)

	if len(req.Header) != 0 {
		t.Errorf("expected empty headers for None auth, got: %v", req.Header)
	}
}
