package safety

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/charleszardd/daegsa/internal/config"
)

// SafetyFlags holds CLI safety execution flags (§12).
type SafetyFlags struct {
	AllowDestructive bool
	NonInteractive   bool
	SkipDNSPreflight bool
}

// PreflightResult holds validated safety metadata and resolved network endpoints (§12).
type PreflightResult struct {
	TargetURL   *url.URL
	Method      string
	ResolvedIPs []net.IP
	Authorized  bool
}

// PreflightEngine executes preflight safety validation prior to test execution (§12).
type PreflightEngine struct {
	resolver *net.Resolver
}

// NewPreflightEngine creates a new PreflightEngine with default network resolver.
func NewPreflightEngine() *PreflightEngine {
	return &PreflightEngine{
		resolver: net.DefaultResolver,
	}
}

// NewPreflightEngineWithResolver creates a new PreflightEngine with a custom DNS resolver.
func NewPreflightEngineWithResolver(r *net.Resolver) *PreflightEngine {
	return &PreflightEngine{
		resolver: r,
	}
}

// Check validates configuration against host allowlists, destructive method rules,
// safety ceilings, and performs DNS preflight resolution (§12).
func (e *PreflightEngine) Check(ctx context.Context, cfg *config.Config, flags SafetyFlags) (*PreflightResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: config cannot be nil", ErrSafetyRefusal)
	}

	targetURL, err := url.Parse(cfg.Request.URL)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid target URL %q", ErrSafetyRefusal, config.RedactURL(cfg.Request.URL))
	}

	host := targetURL.Hostname()
	if host == "" {
		host = targetURL.Host
	}

	// 1. Host Allowlist Check
	if !IsHostAllowed(host, cfg.Safety.AllowedHosts) {
		return nil, fmt.Errorf("%w: %w: target host %q is not in allowed_hosts %v",
			ErrSafetyRefusal, ErrHostNotAllowed, host, cfg.Safety.AllowedHosts)
	}

	// 2. Destructive HTTP Method Check
	if IsDestructiveMethod(cfg.Request.Method) {
		if !cfg.Safety.AllowNonIdempotent && !flags.AllowDestructive {
			return nil, fmt.Errorf("%w: %w: HTTP method %s requires explicit authorization (safety.allow_non_idempotent: true or --allow-destructive)",
				ErrSafetyRefusal, ErrDestructiveMethodUnauthorized, cfg.Request.Method)
		}
	}

	// 3. Safety Ceiling Enforcement
	if cfg.Load.Duration.Duration() > MaxAllowedDuration {
		return nil, fmt.Errorf("%w: %w: duration %v exceeds hard ceiling %v",
			ErrSafetyRefusal, ErrSafetyCeilingExceeded, cfg.Load.Duration, MaxAllowedDuration)
	}
	if cfg.Load.Rate > MaxAllowedRate {
		return nil, fmt.Errorf("%w: %w: rate %v exceeds hard ceiling %v",
			ErrSafetyRefusal, ErrSafetyCeilingExceeded, cfg.Load.Rate, MaxAllowedRate)
	}
	if cfg.Load.Users > MaxAllowedUsers {
		return nil, fmt.Errorf("%w: %w: users %d exceeds hard ceiling %d",
			ErrSafetyRefusal, ErrSafetyCeilingExceeded, cfg.Load.Users, MaxAllowedUsers)
	}
	if cfg.Load.MaxInFlight > MaxAllowedInFlight {
		return nil, fmt.Errorf("%w: %w: max_in_flight %d exceeds hard ceiling %d",
			ErrSafetyRefusal, ErrSafetyCeilingExceeded, cfg.Load.MaxInFlight, MaxAllowedInFlight)
	}
	if cfg.Request.ResponseBodyLimit != "" {
		limitBytes, err := config.ParseByteSize(cfg.Request.ResponseBodyLimit)
		if err == nil && limitBytes > MaxAllowedResponseBodyLimit {
			return nil, fmt.Errorf("%w: %w: response_body_limit %d exceeds hard ceiling %d",
				ErrSafetyRefusal, ErrSafetyCeilingExceeded, limitBytes, MaxAllowedResponseBodyLimit)
		}
	}

	// 4. DNS Preflight Resolution
	var resolvedIPs []net.IP
	if ip := net.ParseIP(host); ip != nil {
		resolvedIPs = []net.IP{ip}
	} else if !flags.SkipDNSPreflight {
		resolver := e.resolver
		if resolver == nil {
			resolver = net.DefaultResolver
		}
		ipAddrs, err := resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("%w: %w: DNS preflight lookup failed for host %q: %w",
				ErrSafetyRefusal, ErrDNSPreflightFailed, host, err)
		}
		if len(ipAddrs) == 0 {
			return nil, fmt.Errorf("%w: %w: no IP addresses resolved for host %q",
				ErrSafetyRefusal, ErrDNSPreflightFailed, host)
		}
		resolvedIPs = make([]net.IP, len(ipAddrs))
		for i, addr := range ipAddrs {
			resolvedIPs[i] = addr.IP
		}
	}

	return &PreflightResult{
		TargetURL:   targetURL,
		Method:      cfg.Request.Method,
		ResolvedIPs: resolvedIPs,
		Authorized:  true,
	}, nil
}
