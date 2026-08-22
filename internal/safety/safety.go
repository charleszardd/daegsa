package safety

import (
	"errors"
	"strings"
	"time"
)

// Hard safety ceiling constants (§12).
const (
	MaxAllowedDuration          = 24 * time.Hour
	MaxAllowedResponseBodyLimit = 50 * 1024 * 1024 // 50 MiB
	MaxAllowedRate              = 1000000.0        // 1,000,000 RPS
	MaxAllowedUsers             = 100000           // 100,000 VUs
	MaxAllowedInFlight          = 100000           // 100,000 in-flight requests
	MaxRedirectHops             = 10               // 10 redirect hops
)

var (
	// ErrSafetyRefusal is the base sentinel error for all safety policy refusals (§10, §12).
	// It is mapped directly to ExitCode 4 (SAFETY_REFUSAL).
	ErrSafetyRefusal = errors.New("safety policy refusal")

	// ErrHostNotAllowed indicates the target host is not in safety.allowed_hosts.
	ErrHostNotAllowed = errors.New("target host is not allowlisted")

	// ErrDestructiveMethodUnauthorized indicates a non-idempotent HTTP method was used without explicit authorization.
	ErrDestructiveMethodUnauthorized = errors.New("destructive HTTP method requires explicit authorization")

	// ErrSafetyCeilingExceeded indicates a workload or resource parameter exceeds the hard safety ceiling.
	ErrSafetyCeilingExceeded = errors.New("parameter exceeds hard safety ceiling")

	// ErrCrossOriginRedirectBlocked indicates a redirect to a different origin was blocked by same-origin policy.
	ErrCrossOriginRedirectBlocked = errors.New("cross-origin redirect blocked by safety policy")

	// ErrDNSPreflightFailed indicates target hostname resolution failed during preflight.
	ErrDNSPreflightFailed = errors.New("DNS preflight resolution failed")
)

// IsDestructiveMethod reports whether the HTTP method is non-idempotent / potentially destructive (§12).
func IsDestructiveMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

// IsHostAllowed checks whether the candidate hostname matches any allowed host pattern (§12).
// Supports exact matching (e.g. "api.example.com") and wildcard prefix matching (e.g. "*.example.com").
func IsHostAllowed(host string, allowedHosts []string) bool {
	if len(allowedHosts) == 0 {
		return true
	}

	host = strings.ToLower(strings.TrimSpace(host))
	// Strip port if present
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		// Ensure it's not an unbracketed IPv6 address
		if !strings.Contains(host, "[") && strings.Count(host, ":") == 1 {
			host = host[:colonIdx]
		}
	}

	for _, pattern := range allowedHosts {
		pat := strings.ToLower(strings.TrimSpace(pattern))
		if pat == "" {
			continue
		}

		// Exact match
		if pat == host {
			return true
		}

		// Wildcard domain match: e.g. *.example.com matches sub.example.com
		if strings.HasPrefix(pat, "*.") {
			domain := pat[2:]
			if host == domain || strings.HasSuffix(host, pat[1:]) {
				return true
			}
		}
	}

	return false
}
