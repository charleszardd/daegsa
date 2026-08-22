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

	// 1. Collect target URLs, hosts, and methods to validate
	type targetEndpoint struct {
		targetURL *url.URL
		host      string
		method    string
		bodyLimit string
	}
	var targets []targetEndpoint

	if cfg.Scenario != nil {
		for _, step := range cfg.Scenario.Steps {
			parsedURL, err := url.Parse(step.URL)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid step URL %q: %w", ErrSafetyRefusal, config.RedactURL(step.URL), err)
			}
			host := parsedURL.Hostname()
			if host == "" {
				host = parsedURL.Host
			}
			targets = append(targets, targetEndpoint{
				targetURL: parsedURL,
				host:      host,
				method:    step.Method,
				bodyLimit: step.ResponseBodyLimit,
			})
		}
	} else {
		targetURL, err := url.Parse(cfg.Request.URL)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid target URL %q", ErrSafetyRefusal, config.RedactURL(cfg.Request.URL))
		}
		host := targetURL.Hostname()
		if host == "" {
			host = targetURL.Host
		}
		targets = append(targets, targetEndpoint{
			targetURL: targetURL,
			host:      host,
			method:    cfg.Request.Method,
			bodyLimit: cfg.Request.ResponseBodyLimit,
		})
	}

	// 2. Validate Host Allowlist & Destructive Methods for each target
	for _, tgt := range targets {
		if tgt.host != "" && !IsHostAllowed(tgt.host, cfg.Safety.AllowedHosts) {
			return nil, fmt.Errorf("%w: %w: target host %q is not in allowed_hosts %v",
				ErrSafetyRefusal, ErrHostNotAllowed, tgt.host, cfg.Safety.AllowedHosts)
		}

		if IsDestructiveMethod(tgt.method) {
			if !cfg.Safety.AllowNonIdempotent && !flags.AllowDestructive {
				return nil, fmt.Errorf("%w: %w: HTTP method %s requires explicit authorization (safety.allow_non_idempotent: true or --allow-destructive)",
					ErrSafetyRefusal, ErrDestructiveMethodUnauthorized, tgt.method)
			}
		}

		if tgt.bodyLimit != "" {
			limitBytes, err := config.ParseByteSize(tgt.bodyLimit)
			if err == nil && limitBytes > MaxAllowedResponseBodyLimit {
				return nil, fmt.Errorf("%w: %w: response_body_limit %d exceeds hard ceiling %d",
					ErrSafetyRefusal, ErrSafetyCeilingExceeded, limitBytes, MaxAllowedResponseBodyLimit)
			}
		}
	}

	// 3. Safety Ceiling Enforcement
	effectiveDuration := cfg.Load.Duration.Duration()
	peakTargetRPS := cfg.Load.Rate
	if cfg.Load.TimeUnit.Duration() > 0 {
		peakTargetRPS = cfg.Load.Rate / cfg.Load.TimeUnit.Duration().Seconds()
	}
	if len(cfg.Load.Segments) > 0 {
		compiled, compileErr := config.CompileLoadProfile(&cfg.Load)
		if compileErr != nil {
			return nil, compileErr
		}
		effectiveDuration = compiled.TotalDuration
		peakTargetRPS = compiled.PeakTargetRPS
	}
	if effectiveDuration > MaxAllowedDuration {
		return nil, fmt.Errorf("%w: %w: duration %v exceeds hard ceiling %v",
			ErrSafetyRefusal, ErrSafetyCeilingExceeded, effectiveDuration, MaxAllowedDuration)
	}
	if peakTargetRPS > MaxAllowedRate {
		return nil, fmt.Errorf("%w: %w: peak target RPS %v exceeds hard ceiling %v",
			ErrSafetyRefusal, ErrSafetyCeilingExceeded, peakTargetRPS, MaxAllowedRate)
	}
	if cfg.Load.Users > MaxAllowedUsers {
		return nil, fmt.Errorf("%w: %w: users %d exceeds hard ceiling %d",
			ErrSafetyRefusal, ErrSafetyCeilingExceeded, cfg.Load.Users, MaxAllowedUsers)
	}
	if cfg.Load.MaxInFlight > MaxAllowedInFlight {
		return nil, fmt.Errorf("%w: %w: max_in_flight %d exceeds hard ceiling %d",
			ErrSafetyRefusal, ErrSafetyCeilingExceeded, cfg.Load.MaxInFlight, MaxAllowedInFlight)
	}

	// 4. DNS Preflight Resolution on all unique hosts
	uniqueHosts := make(map[string]struct{})
	for _, tgt := range targets {
		if tgt.host != "" {
			uniqueHosts[tgt.host] = struct{}{}
		}
	}

	var resolvedIPs []net.IP
	for host := range uniqueHosts {
		if ip := net.ParseIP(host); ip != nil {
			resolvedIPs = append(resolvedIPs, ip)
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
			for _, addr := range ipAddrs {
				resolvedIPs = append(resolvedIPs, addr.IP)
			}
		}
	}

	primaryTargetURL := targets[0].targetURL
	primaryMethod := targets[0].method
	if cfg.Scenario != nil {
		primaryMethod = "SCENARIO"
	}

	return &PreflightResult{
		TargetURL:   primaryTargetURL,
		Method:      primaryMethod,
		ResolvedIPs: resolvedIPs,
		Authorized:  true,
	}, nil
}
