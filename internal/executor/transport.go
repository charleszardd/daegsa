package executor

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// TransportOptions configures connection pooling and socket timeouts for the shared HTTP transport (§8).
type TransportOptions struct {
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	MaxConnsPerHost       int
	IdleConnTimeout       time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	DisableKeepAlives     bool
	DisableCompression    bool
	TLSClientConfig       *tls.Config
}

// DefaultTransportOptions returns recommended production defaults for high-throughput load generation (§8).
func DefaultTransportOptions() TransportOptions {
	return TransportOptions{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   500,
		MaxConnsPerHost:       0, // unlimited pooled connections
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		DisableCompression:    false,
	}
}

// NewSharedTransport creates a tuned, pooled *http.Transport instance to be shared across workers (§8).
func NewSharedTransport(opts TransportOptions) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          opts.MaxIdleConns,
		MaxIdleConnsPerHost:   opts.MaxIdleConnsPerHost,
		MaxConnsPerHost:       opts.MaxConnsPerHost,
		IdleConnTimeout:       opts.IdleConnTimeout,
		TLSHandshakeTimeout:   opts.TLSHandshakeTimeout,
		ExpectContinueTimeout: opts.ExpectContinueTimeout,
		DisableKeepAlives:     opts.DisableKeepAlives,
		DisableCompression:    opts.DisableCompression,
		TLSClientConfig:       opts.TLSClientConfig,
	}

	return transport
}
