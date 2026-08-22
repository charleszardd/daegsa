package doctor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"time"
)

// CheckTLSConfiguration evaluates system root CAs and performs a loopback TLS handshake (§14).
func CheckTLSConfiguration(ctx context.Context) CheckResult {
	start := time.Now()

	// 1. Verify system root CA pool
	var rootPoolWarn string
	pool, err := x509.SystemCertPool()
	if err != nil {
		if runtime.GOOS == "windows" {
			// On Windows, Go 1.18+ uses native crypto API if SystemCertPool returns error or nil, which is standard.
			rootPoolWarn = "System cert pool loading used Windows CryptoAPI fallback"
		} else {
			rootPoolWarn = fmt.Sprintf("Failed to load system cert pool: %v", err)
		}
	} else if pool == nil && runtime.GOOS != "windows" {
		rootPoolWarn = "System root CA pool is empty"
	}

	// 2. Spin up ephemeral loopback TLS server
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("tls-ok"))
	}))
	defer ts.Close()

	// 3. Perform TLS client handshake
	client := ts.Client()
	client.Timeout = 2 * time.Second

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	if err != nil {
		return CheckResult{
			Name:       "TLS Handshake & Root Certificates",
			Category:   CategoryTLS,
			Status:     StatusFail,
			Summary:    "Failed to create test TLS request",
			Detail:     err.Error(),
			Suggestion: "Verify internal networking and runtime HTTP client state.",
			Duration:   time.Since(start),
		}
	}

	resp, err := client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		return CheckResult{
			Name:       "TLS Handshake & Root Certificates",
			Category:   CategoryTLS,
			Status:     StatusFail,
			Summary:    "Loopback TLS handshake failed",
			Detail:     fmt.Sprintf("TLS error: %v", err),
			Suggestion: "Check system TLS/crypto libraries and local firewall / antivirus TLS interception rules.",
			Duration:   elapsed,
		}
	}
	defer resp.Body.Close()

	// 4. Inspect TLS connection state
	var tlsVer string
	var cipherName string
	if resp.TLS != nil {
		switch resp.TLS.Version {
		case tls.VersionTLS13:
			tlsVer = "TLS 1.3"
		case tls.VersionTLS12:
			tlsVer = "TLS 1.2"
		default:
			tlsVer = fmt.Sprintf("TLS 0x%04x", resp.TLS.Version)
		}
		cipherName = tls.CipherSuiteName(resp.TLS.CipherSuite)
	}

	detail := fmt.Sprintf("Negotiated: %s with %s (Handshake & roundtrip: %v)", tlsVer, cipherName, elapsed.Truncate(time.Microsecond))

	if rootPoolWarn != "" && runtime.GOOS != "windows" {
		return CheckResult{
			Name:       "TLS Handshake & Root Certificates",
			Category:   CategoryTLS,
			Status:     StatusWarn,
			Summary:    rootPoolWarn,
			Detail:     detail,
			Suggestion: "Verify ca-certificates package or system trust store installation.",
			Duration:   elapsed,
		}
	}

	return CheckResult{
		Name:     "TLS Handshake & Root Certificates",
		Category: CategoryTLS,
		Status:   StatusPass,
		Summary:  fmt.Sprintf("Functional (%s, %s)", tlsVer, cipherName),
		Detail:   detail,
		Duration: elapsed,
	}
}
