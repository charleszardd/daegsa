package config_test

import (
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
)

func TestValidateConfig_AuthValidation(t *testing.T) {
	baseCfg := func() *config.Config {
		return &config.Config{
			SchemaVersion: 1,
			Name:          "auth-validation-test",
			Request: config.RequestConfig{
				URL:    "https://api.example.com/data",
				Method: "GET",
			},
			Load: config.LoadConfig{
				Model:    core.WorkloadModelClosed,
				Users:    5,
				Duration: config.Duration(5 * time.Second),
			},
		}
	}

	tests := []struct {
		name        string
		auth        config.AuthConfig
		expectErr   bool
		checkHeader string
	}{
		{
			name:      "empty auth block (default none)",
			auth:      config.AuthConfig{},
			expectErr: false,
		},
		{
			name: "explicit none auth",
			auth: config.AuthConfig{
				Type: "none",
			},
			expectErr: false,
		},
		{
			name: "valid bearer auth",
			auth: config.AuthConfig{
				Type:  "bearer",
				Token: "valid-bearer-token",
			},
			expectErr:   false,
			checkHeader: "Authorization",
		},
		{
			name: "bearer auth rejects custom header",
			auth: config.AuthConfig{
				Type:       "bearer",
				Token:      "valid-bearer-token",
				HeaderName: "X-API-Key",
			},
			expectErr: true,
		}, {
			name: "bearer auth missing token",
			auth: config.AuthConfig{
				Type:  "bearer",
				Token: "",
			},
			expectErr: true,
		},
		{
			name: "valid custom header auth",
			auth: config.AuthConfig{
				Type:       "custom_header",
				Token:      "my-api-key-12345",
				HeaderName: "X-API-Key",
			},
			expectErr:   false,
			checkHeader: "X-API-Key",
		},
		{
			name: "custom header auth missing header_name",
			auth: config.AuthConfig{
				Type:       "custom_header",
				Token:      "my-api-key-12345",
				HeaderName: "",
			},
			expectErr: true,
		},
		{
			name: "custom header auth missing token",
			auth: config.AuthConfig{
				Type:       "custom_header",
				Token:      "",
				HeaderName: "X-API-Key",
			},
			expectErr: true,
		},
		{
			name: "valid basic auth",
			auth: config.AuthConfig{
				Type:     "basic",
				Username: "admin_user",
				Password: "secret_password",
			},
			expectErr:   false,
			checkHeader: "Authorization",
		},
		{
			name: "basic auth missing username",
			auth: config.AuthConfig{
				Type:     "basic",
				Username: "",
				Password: "secret_password",
			},
			expectErr: true,
		},
		{
			name: "valid token pool auth",
			auth: config.AuthConfig{
				Type:      "token_pool",
				TokenPool: []string{"tok_1", "tok_2", "tok_3"},
			},
			expectErr:   false,
			checkHeader: "Authorization",
		},
		{
			name: "token pool auth empty slice",
			auth: config.AuthConfig{
				Type:      "token_pool",
				TokenPool: []string{},
			},
			expectErr: true,
		},
		{
			name: "token pool auth with empty token element",
			auth: config.AuthConfig{
				Type:      "token_pool",
				TokenPool: []string{"tok_1", "  ", "tok_3"},
			},
			expectErr: true,
		},
		{
			name: "unsupported auth type",
			auth: config.AuthConfig{
				Type:  "oauth_dance",
				Token: "token",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseCfg()
			cfg.Auth = tt.auth
			err := config.ValidateConfig(cfg)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected validation error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected validation error: %v", err)
				}
				if tt.checkHeader != "" && cfg.Auth.HeaderName != tt.checkHeader {
					t.Errorf("expected header_name %q, got %q", tt.checkHeader, cfg.Auth.HeaderName)
				}
			}
		})
	}
}
