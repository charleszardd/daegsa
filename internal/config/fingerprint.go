package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// ComputeFingerprint calculates a deterministic SHA-256 hex digest of the sanitized configuration (§6, §13).
// Sensitive headers and URL query credentials are fully redacted before fingerprint calculation so that
// credential rotations do not alter the test manifest fingerprint.
func ComputeFingerprint(cfg *Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("%w: cannot fingerprint nil config", ErrConfigValidation)
	}

	sanitized := cloneConfigSanitized(cfg)
	canonicalBytes, err := json.Marshal(sanitized)
	if err != nil {
		return "", fmt.Errorf("%w: failed to serialize config for fingerprinting: %w", ErrConfigValidation, err)
	}

	hash := sha256.Sum256(canonicalBytes)
	return fmt.Sprintf("%x", hash), nil
}

func cloneConfigSanitized(cfg *Config) *Config {
	c := &Config{
		SchemaVersion: cfg.SchemaVersion,
		Name:          cfg.Name,
		Request: RequestConfig{
			URL:               RedactURL(cfg.Request.URL),
			Method:            cfg.Request.Method,
			ExpectedStatuses:  make([]int, len(cfg.Request.ExpectedStatuses)),
			Timeout:           cfg.Request.Timeout,
			ResponseBodyLimit: cfg.Request.ResponseBodyLimit,
			Redirects:         cfg.Request.Redirects,
		},
		Load: LoadConfig{
			Model:        cfg.Load.Model,
			Rate:         cfg.Load.Rate,
			TimeUnit:     cfg.Load.TimeUnit,
			MaxInFlight:  cfg.Load.MaxInFlight,
			Duration:     cfg.Load.Duration,
			GracefulStop: cfg.Load.GracefulStop,
			Users:        cfg.Load.Users,
			ThinkTime:    cfg.Load.ThinkTime,
			Segments:     make([]ProfileSegmentConfig, len(cfg.Load.Segments)),
		},
		RateLimit: RateLimitConfig{
			Treat429AsExpected: cfg.RateLimit.Treat429AsExpected,
		},
		Auth: AuthConfig{
			Type:       cfg.Auth.Type,
			HeaderName: cfg.Auth.HeaderName,
			CookieJar:  cfg.Auth.CookieJar,
		},
		Safety: SafetyConfig{
			AllowedHosts:       make([]string, len(cfg.Safety.AllowedHosts)),
			AllowNonIdempotent: cfg.Safety.AllowNonIdempotent,
		},
	}

	if cfg.Auth.Token != "" {
		c.Auth.Token = RedactedPlaceholder
	}
	if cfg.Auth.Username != "" {
		c.Auth.Username = RedactedPlaceholder
	}
	if cfg.Auth.Password != "" {
		c.Auth.Password = RedactedPlaceholder
	}
	if len(cfg.Auth.TokenPool) > 0 {
		c.Auth.TokenPool = make([]string, len(cfg.Auth.TokenPool))
		for i := range cfg.Auth.TokenPool {
			c.Auth.TokenPool[i] = RedactedPlaceholder
		}
	}

	copy(c.Request.ExpectedStatuses, cfg.Request.ExpectedStatuses)
	copy(c.Safety.AllowedHosts, cfg.Safety.AllowedHosts)
	copy(c.Load.Segments, cfg.Load.Segments)

	if cfg.Request.Headers != nil {
		c.Request.Headers = RedactHeaders(cfg.Request.Headers)
	}

	if cfg.Thresholds != nil {
		c.Thresholds = make(map[string]string, len(cfg.Thresholds))
		for k, v := range cfg.Thresholds {
			c.Thresholds[k] = v
		}
	}

	return c
}
