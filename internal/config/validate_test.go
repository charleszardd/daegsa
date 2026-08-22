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

func TestValidateConfig_ScenarioValidation(t *testing.T) {
	validSteps := func() []config.StepConfig {
		return []config.StepConfig{
			{
				Name:   "login",
				URL:    "https://api.example.com/login",
				Method: "POST",
				Body:   `{"user":"test"}`,
				Extract: map[string]config.ExtractRuleConfig{
					"token": {
						From:       "json",
						Expression: "token",
					},
					"cookie_val": {
						From:       "cookie",
						Expression: "session_id",
					},
				},
				OnFailure: "stop",
			},
			{
				Name:   "get_items",
				URL:    "https://api.example.com/items",
				Method: "GET",
				Headers: map[string]string{
					"Authorization": "Bearer ${token}",
				},
				ThinkTime: config.Duration(100 * time.Millisecond),
				Extract: map[string]config.ExtractRuleConfig{
					"first_id": {
						From:       "jsonpath",
						Expression: "$.items[0].id",
					},
					"header_val": {
						From:       "header",
						Expression: "X-Request-Id",
					},
					"regex_val": {
						From:       "regex",
						Expression: "id=([0-9]+)",
					},
				},
				OnFailure: "continue",
			},
			{
				Name:      "logout",
				URL:       "https://api.example.com/logout",
				Method:    "POST",
				OnFailure: "abort_vu",
			},
		}
	}

	tests := []struct {
		name      string
		mutate    func(cfg *config.Config)
		expectErr bool
	}{
		{
			name: "valid multi-step scenario",
			mutate: func(cfg *config.Config) {
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: validSteps(),
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: false,
		},
		{
			name: "mutual exclusivity: both request and scenario defined",
			mutate: func(cfg *config.Config) {
				cfg.Request = config.RequestConfig{URL: "https://api.example.com/single", Method: "GET"}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: validSteps(),
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "mutual exclusivity: neither request nor scenario defined",
			mutate: func(cfg *config.Config) {
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = nil
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "scenario requires closed model",
			mutate: func(cfg *config.Config) {
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: validSteps(),
				}
				cfg.Load = config.LoadConfig{
					Model:       core.WorkloadModelOpen,
					Rate:        10,
					MaxInFlight: 10,
					Duration:    config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "empty scenario name",
			mutate: func(cfg *config.Config) {
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "   ",
					Steps: validSteps(),
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "empty steps slice",
			mutate: func(cfg *config.Config) {
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: []config.StepConfig{},
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "duplicate step names",
			mutate: func(cfg *config.Config) {
				steps := validSteps()
				steps[1].Name = "login" // duplicate with step 0
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: steps,
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "step missing URL",
			mutate: func(cfg *config.Config) {
				steps := validSteps()
				steps[0].URL = ""
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: steps,
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "step unsupported HTTP method",
			mutate: func(cfg *config.Config) {
				steps := validSteps()
				steps[0].Method = "INVALID_METHOD"
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: steps,
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "step negative think time",
			mutate: func(cfg *config.Config) {
				steps := validSteps()
				steps[0].ThinkTime = config.Duration(-1 * time.Second)
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: steps,
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "step invalid on_failure policy",
			mutate: func(cfg *config.Config) {
				steps := validSteps()
				steps[0].OnFailure = "retry_forever"
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: steps,
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "extract rule invalid source",
			mutate: func(cfg *config.Config) {
				steps := validSteps()
				steps[0].Extract["bad"] = config.ExtractRuleConfig{
					From:       "xpath",
					Expression: "/root/item",
				}
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: steps,
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "extract rule empty expression",
			mutate: func(cfg *config.Config) {
				steps := validSteps()
				steps[0].Extract["bad"] = config.ExtractRuleConfig{
					From:       "json",
					Expression: "",
				}
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: steps,
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "extract rule invalid regex syntax",
			mutate: func(cfg *config.Config) {
				steps := validSteps()
				steps[0].Extract["bad"] = config.ExtractRuleConfig{
					From:       "regex",
					Expression: "[a-z(",
				}
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: steps,
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
		{
			name: "extract rule empty variable name",
			mutate: func(cfg *config.Config) {
				steps := validSteps()
				steps[0].Extract["  "] = config.ExtractRuleConfig{
					From:       "json",
					Expression: "token",
				}
				cfg.Request = config.RequestConfig{}
				cfg.Scenario = &config.ScenarioConfig{
					Name:  "user_journey",
					Steps: steps,
				}
				cfg.Load = config.LoadConfig{
					Model:    core.WorkloadModelClosed,
					Users:    10,
					Duration: config.Duration(10 * time.Second),
				}
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				SchemaVersion: 1,
				Name:          "test-config",
			}
			tt.mutate(cfg)
			err := config.ValidateConfig(cfg)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected validation error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected validation error: %v", err)
				}
			}
		})
	}
}
