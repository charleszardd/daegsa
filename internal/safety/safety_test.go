package safety

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
)

func TestHostAllowlist(t *testing.T) {
	tests := []struct {
		name         string
		targetHost   string
		allowedHosts []string
		shouldAllow  bool
	}{
		{
			name:         "empty allowlist denies all",
			targetHost:   "api.example.com",
			allowedHosts: nil,
			shouldAllow:  false,
		},
		{
			name:         "exact match",
			targetHost:   "api.example.com",
			allowedHosts: []string{"api.example.com"},
			shouldAllow:  true,
		},
		{
			name:         "exact match case insensitive",
			targetHost:   "API.Example.Com",
			allowedHosts: []string{"api.example.com"},
			shouldAllow:  true,
		},
		{
			name:         "wildcard match subdomain",
			targetHost:   "sub.api.example.com",
			allowedHosts: []string{"*.example.com"},
			shouldAllow:  true,
		},
		{
			name:         "wildcard match root domain",
			targetHost:   "example.com",
			allowedHosts: []string{"*.example.com"},
			shouldAllow:  true,
		},
		{
			name:         "disallowed different domain",
			targetHost:   "evil.com",
			allowedHosts: []string{"api.example.com"},
			shouldAllow:  false,
		},
		{
			name:         "disallowed partial domain prefix",
			targetHost:   "myexample.com",
			allowedHosts: []string{"*.example.com"},
			shouldAllow:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := IsHostAllowed(tt.targetHost, tt.allowedHosts)
			if allowed != tt.shouldAllow {
				t.Errorf("IsHostAllowed(%q, %v) = %v; want %v", tt.targetHost, tt.allowedHosts, allowed, tt.shouldAllow)
			}
		})
	}
}

func TestPreflightEngine_HostAllowlistCheck(t *testing.T) {
	engine := NewPreflightEngine()
	ctx := context.Background()

	cfg := &config.Config{
		SchemaVersion: 1,
		Name:          "safety-test",
		Request: config.RequestConfig{
			URL:    "http://127.0.0.1:8080/api",
			Method: "GET",
		},
		Load: config.LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        10,
			Duration:    config.Duration(5 * time.Second),
			MaxInFlight: 20,
		},
		Safety: config.SafetyConfig{
			AllowedHosts: []string{"api.example.com"},
		},
	}

	// 127.0.0.1 is not in allowed_hosts
	_, err := engine.Check(ctx, cfg, SafetyFlags{SkipDNSPreflight: true})
	if err == nil {
		t.Fatalf("expected host allowlist error, got nil")
	}
	if !errors.Is(err, ErrSafetyRefusal) {
		t.Errorf("expected ErrSafetyRefusal, got %v", err)
	}
	if !errors.Is(err, ErrHostNotAllowed) {
		t.Errorf("expected ErrHostNotAllowed, got %v", err)
	}

	// Add 127.0.0.1 to allowed_hosts -> passes
	cfg.Safety.AllowedHosts = append(cfg.Safety.AllowedHosts, "127.0.0.1")
	result, err := engine.Check(ctx, cfg, SafetyFlags{SkipDNSPreflight: true})
	if err != nil {
		t.Fatalf("unexpected error when host is allowed: %v", err)
	}
	if result == nil || !result.Authorized {
		t.Errorf("expected result to be authorized, got %v", result)
	}
}

func TestPreflightEngine_DestructiveMethods(t *testing.T) {
	engine := NewPreflightEngine()
	ctx := context.Background()

	destructiveMethods := []string{"POST", "PUT", "PATCH", "DELETE"}
	safeMethods := []string{"GET", "HEAD", "OPTIONS"}

	for _, method := range safeMethods {
		t.Run("Safe_"+method, func(t *testing.T) {
			cfg := &config.Config{
				SchemaVersion: 1,
				Name:          "safe-method-test",
				Request: config.RequestConfig{
					URL:    "http://127.0.0.1:8080/api",
					Method: method,
				},
				Load: config.LoadConfig{
					Model:       core.WorkloadModelOpen,
					Rate:        10,
					Duration:    config.Duration(5 * time.Second),
					MaxInFlight: 20,
				},
			}
			_, err := engine.Check(ctx, withAllowedHost(cfg, "127.0.0.1"), SafetyFlags{SkipDNSPreflight: true})
			if err != nil {
				t.Fatalf("safe method %s should pass without authorization: %v", method, err)
			}
		})
	}

	for _, method := range destructiveMethods {
		t.Run("UnauthorizedDestructive_"+method, func(t *testing.T) {
			cfg := &config.Config{
				SchemaVersion: 1,
				Name:          "destructive-method-test",
				Request: config.RequestConfig{
					URL:    "http://127.0.0.1:8080/api",
					Method: method,
				},
				Load: config.LoadConfig{
					Model:       core.WorkloadModelOpen,
					Rate:        10,
					Duration:    config.Duration(5 * time.Second),
					MaxInFlight: 20,
				},
				Safety: config.SafetyConfig{
					AllowNonIdempotent: false,
				},
			}
			_, err := engine.Check(ctx, withAllowedHost(cfg, "127.0.0.1"), SafetyFlags{SkipDNSPreflight: true, AllowDestructive: false})
			if err == nil {
				t.Fatalf("destructive method %s must fail without authorization", method)
			}
			if !errors.Is(err, ErrSafetyRefusal) {
				t.Errorf("expected ErrSafetyRefusal, got %v", err)
			}
			if !errors.Is(err, ErrDestructiveMethodUnauthorized) {
				t.Errorf("expected ErrDestructiveMethodUnauthorized, got %v", err)
			}
		})

		t.Run("AuthorizedDestructive_ViaConfig_"+method, func(t *testing.T) {
			cfg := &config.Config{
				SchemaVersion: 1,
				Name:          "destructive-config-auth",
				Request: config.RequestConfig{
					URL:    "http://127.0.0.1:8080/api",
					Method: method,
				},
				Load: config.LoadConfig{
					Model:       core.WorkloadModelOpen,
					Rate:        10,
					Duration:    config.Duration(5 * time.Second),
					MaxInFlight: 20,
				},
				Safety: config.SafetyConfig{
					AllowNonIdempotent: true,
				},
			}
			_, err := engine.Check(ctx, withAllowedHost(cfg, "127.0.0.1"), SafetyFlags{SkipDNSPreflight: true})
			if err != nil {
				t.Fatalf("destructive method %s should pass with config authorization: %v", method, err)
			}
		})

		t.Run("AuthorizedDestructive_ViaFlag_"+method, func(t *testing.T) {
			cfg := &config.Config{
				SchemaVersion: 1,
				Name:          "destructive-flag-auth",
				Request: config.RequestConfig{
					URL:    "http://127.0.0.1:8080/api",
					Method: method,
				},
				Load: config.LoadConfig{
					Model:       core.WorkloadModelOpen,
					Rate:        10,
					Duration:    config.Duration(5 * time.Second),
					MaxInFlight: 20,
				},
			}
			_, err := engine.Check(ctx, withAllowedHost(cfg, "127.0.0.1"), SafetyFlags{SkipDNSPreflight: true, AllowDestructive: true})
			if err != nil {
				t.Fatalf("destructive method %s should pass with flag authorization: %v", method, err)
			}
		})
	}
}

func TestPreflightEngine_Ceilings(t *testing.T) {
	engine := NewPreflightEngine()
	ctx := context.Background()

	// Rate ceiling breach
	cfgRate := &config.Config{
		SchemaVersion: 1,
		Name:          "rate-ceiling",
		Request: config.RequestConfig{
			URL:    "http://127.0.0.1:8080",
			Method: "GET",
		},
		Load: config.LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        MaxAllowedRate + 1,
			Duration:    config.Duration(10 * time.Second),
			MaxInFlight: 100,
		},
	}
	_, err := engine.Check(ctx, withAllowedHost(cfgRate, "127.0.0.1"), SafetyFlags{SkipDNSPreflight: true})
	if !errors.Is(err, ErrSafetyCeilingExceeded) {
		t.Errorf("expected ErrSafetyCeilingExceeded for rate, got %v", err)
	}

	// Users ceiling breach
	cfgUsers := &config.Config{
		SchemaVersion: 1,
		Name:          "users-ceiling",
		Request: config.RequestConfig{
			URL:    "http://127.0.0.1:8080",
			Method: "GET",
		},
		Load: config.LoadConfig{
			Model:    core.WorkloadModelClosed,
			Users:    MaxAllowedUsers + 1,
			Duration: config.Duration(10 * time.Second),
		},
	}
	_, err = engine.Check(ctx, withAllowedHost(cfgUsers, "127.0.0.1"), SafetyFlags{SkipDNSPreflight: true})
	if !errors.Is(err, ErrSafetyCeilingExceeded) {
		t.Errorf("expected ErrSafetyCeilingExceeded for users, got %v", err)
	}

	// Duration ceiling breach
	cfgDuration := &config.Config{
		SchemaVersion: 1,
		Name:          "duration-ceiling",
		Request: config.RequestConfig{
			URL:    "http://127.0.0.1:8080",
			Method: "GET",
		},
		Load: config.LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        10,
			Duration:    config.Duration(MaxAllowedDuration + time.Hour),
			MaxInFlight: 100,
		},
	}
	_, err = engine.Check(ctx, withAllowedHost(cfgDuration, "127.0.0.1"), SafetyFlags{SkipDNSPreflight: true})
	if !errors.Is(err, ErrSafetyCeilingExceeded) {
		t.Errorf("expected ErrSafetyCeilingExceeded for duration, got %v", err)
	}
}

func TestPreflightEngine_DNSPreflight(t *testing.T) {
	engine := NewPreflightEngine()
	ctx := context.Background()

	// 1. IP literal
	cfgIP := &config.Config{
		SchemaVersion: 1,
		Name:          "ip-preflight",
		Request: config.RequestConfig{
			URL:    "http://127.0.0.1:8080",
			Method: "GET",
		},
		Load: config.LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        10,
			Duration:    config.Duration(5 * time.Second),
			MaxInFlight: 20,
		},
	}
	resIP, err := engine.Check(ctx, withAllowedHost(cfgIP, "127.0.0.1"), SafetyFlags{})
	if err != nil {
		t.Fatalf("unexpected error for IP literal: %v", err)
	}
	if len(resIP.ResolvedIPs) == 0 {
		t.Errorf("expected resolved IP for 127.0.0.1")
	}

	// 2. Loopback localhost
	cfgLocalhost := &config.Config{
		SchemaVersion: 1,
		Name:          "localhost-preflight",
		Request: config.RequestConfig{
			URL:    "http://localhost:8080",
			Method: "GET",
		},
		Load: config.LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        10,
			Duration:    config.Duration(5 * time.Second),
			MaxInFlight: 20,
		},
	}
	resLocal, err := engine.Check(ctx, withAllowedHost(cfgLocalhost, "localhost"), SafetyFlags{})
	if err != nil {
		t.Fatalf("unexpected error resolving localhost: %v", err)
	}
	if len(resLocal.ResolvedIPs) == 0 {
		t.Errorf("expected resolved IPs for localhost")
	}

	// 3. Unresolvable domain
	cfgBadDomain := &config.Config{
		SchemaVersion: 1,
		Name:          "bad-domain",
		Request: config.RequestConfig{
			URL:    "http://nonexistent-domain-that-does-not-exist-daegsa.invalid:8080",
			Method: "GET",
		},
		Load: config.LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        10,
			Duration:    config.Duration(5 * time.Second),
			MaxInFlight: 20,
		},
	}
	_, err = engine.Check(ctx, withAllowedHost(cfgBadDomain, "nonexistent-domain-that-does-not-exist-daegsa.invalid"), SafetyFlags{})
	if err == nil {
		t.Fatalf("expected DNS error for unresolvable domain, got nil")
	}
	if !errors.Is(err, ErrDNSPreflightFailed) {
		t.Errorf("expected ErrDNSPreflightFailed, got %v", err)
	}
}

func TestHostAllowlistRequiresHostOnlyCandidates(t *testing.T) {
	if IsHostAllowed("localhost:8080", []string{"localhost"}) {
		t.Error("candidate with a port unexpectedly matched")
	}
	if IsHostAllowed("api.example.com:443", []string{"api.example.com"}) {
		t.Error("candidate with a port unexpectedly matched")
	}
	if !IsHostAllowed("::1", []string{"0:0:0:0:0:0:0:1"}) {
		t.Error("canonical IPv6 host did not match")
	}
}

func TestPreflightEngine_ResponseBodyLimitCeiling(t *testing.T) {
	engine := NewPreflightEngine()
	ctx := context.Background()

	// 100 MiB exceeds MaxAllowedResponseBodyLimit (50 MiB)
	cfg := &config.Config{
		SchemaVersion: 1,
		Name:          "body-limit-ceiling",
		Request: config.RequestConfig{
			URL:               "http://127.0.0.1:8080",
			Method:            "GET",
			ResponseBodyLimit: "100MiB",
		},
		Load: config.LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        10,
			Duration:    config.Duration(5 * time.Second),
			MaxInFlight: 20,
		},
	}

	_, err := engine.Check(ctx, withAllowedHost(cfg, "127.0.0.1"), SafetyFlags{SkipDNSPreflight: true})
	if err == nil {
		t.Fatalf("expected error for 100MiB response body limit, got nil")
	}
	if !errors.Is(err, ErrSafetyCeilingExceeded) {
		t.Errorf("expected ErrSafetyCeilingExceeded, got %v", err)
	}
}

func withAllowedHost(cfg *config.Config, host string) *config.Config {
	cfg.Safety.AllowedHosts = []string{host}
	return cfg
}
