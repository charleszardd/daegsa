package auth_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/charleszardd/daegsa/internal/auth"
	"github.com/charleszardd/daegsa/internal/config"
)

func TestExhaustiveSecretLeakage_AuthPrimitives(t *testing.T) {
	const (
		sentinelBearer  = "SENTINEL_BEARER_TOKEN_ALPHA_999"
		sentinelPass    = "SENTINEL_PASSWORD_BETA_888"
		sentinelAPIKey  = "SENTINEL_APIKEY_GAMMA_777"
		sentinelPoolTok = "SENTINEL_POOL_TOKEN_DELTA_666"
	)

	sentinels := []string{
		sentinelBearer,
		sentinelPass,
		sentinelAPIKey,
		sentinelPoolTok,
	}

	assertNoLeakage := func(t *testing.T, contextName string, output string) {
		t.Helper()
		for _, s := range sentinels {
			if strings.Contains(output, s) {
				t.Fatalf("CRITICAL SECURITY VIOLATION: Sentinel secret %q leaked in %s: %q", s, contextName, output)
			}
		}
	}

	// 1. URL Redaction
	rawURL := fmt.Sprintf("https://admin:%s@target.host:8443/v1/auth?token=%s&apikey=%s&auth=%s",
		sentinelPass, sentinelBearer, sentinelAPIKey, sentinelPoolTok)
	redactedURL := auth.RedactURL(rawURL)
	assertNoLeakage(t, "RedactURL", redactedURL)

	// 2. Map Headers Redaction
	headers := map[string]string{
		"Authorization":       "Bearer " + sentinelBearer,
		"Proxy-Authorization": "Basic " + sentinelPass,
		"X-Api-Key":           sentinelAPIKey,
		"X-Custom-Token":      sentinelPoolTok,
	}
	redactedHeaders := auth.RedactHeaders(headers)
	for k, v := range redactedHeaders {
		assertNoLeakage(t, "RedactHeaders["+k+"]", v)
	}

	// 3. HTTP Header Redaction
	httpHeader := http.Header{}
	for k, v := range headers {
		httpHeader.Set(k, v)
	}
	redactedHTTP := auth.RedactHTTPHeaders(httpHeader)
	for k, vals := range redactedHTTP {
		for _, val := range vals {
			assertNoLeakage(t, "RedactHTTPHeaders["+k+"]", val)
		}
	}

	// 4. Arbitrary Error and String Redaction
	errMsg := fmt.Sprintf("HTTP 500 error connecting to remote service: Authorization: Bearer %s; Password: %s; Key: %s; Pool: %s",
		sentinelBearer, sentinelPass, sentinelAPIKey, sentinelPoolTok)
	err := errors.New(errMsg)
	scrubbedErr := auth.RedactError(err, sentinels)
	assertNoLeakage(t, "RedactError", scrubbedErr.Error())

	// 5. Config Fingerprint Invariance and Sanitization
	cfg := &config.Config{
		SchemaVersion: 1,
		Name:          "sentinel-test",
		Request: config.RequestConfig{
			URL:     rawURL,
			Method:  "POST",
			Headers: headers,
		},
		Auth: config.AuthConfig{
			Type:       auth.AuthTypeTokenPool,
			TokenPool:  []string{sentinelBearer, sentinelPoolTok},
			Password:   sentinelPass,
			HeaderName: "X-Api-Key",
		},
	}
	fp, fpErr := config.ComputeFingerprint(cfg)
	if fpErr != nil {
		t.Fatalf("failed to compute fingerprint: %v", fpErr)
	}
	assertNoLeakage(t, "ComputeFingerprint", fp)
}
