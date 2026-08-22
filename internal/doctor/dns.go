package doctor

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	warnDNSLatency = 500 * time.Millisecond
)

// CheckDNSResolution verifies local loopback name resolution and latency (§14).
func CheckDNSResolution(ctx context.Context) CheckResult {
	start := time.Now()

	// 1. Resolve localhost
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, "localhost")
	lookupDuration := time.Since(start)

	if err != nil {
		return CheckResult{
			Name:       "DNS Resolution (Loopback & Local)",
			Category:   CategoryDNS,
			Status:     StatusFail,
			Summary:    "Failed to resolve 'localhost'",
			Detail:     fmt.Sprintf("Lookup error: %v", err),
			Suggestion: "Check system hosts file (e.g. C:\\Windows\\System32\\drivers\\etc\\hosts or /etc/hosts) to ensure '127.0.0.1 localhost' and '::1 localhost' are present.",
			Duration:   lookupDuration,
		}
	}

	if len(addrs) == 0 {
		return CheckResult{
			Name:       "DNS Resolution (Loopback & Local)",
			Category:   CategoryDNS,
			Status:     StatusFail,
			Summary:    "Resolved 'localhost' to 0 IP addresses",
			Detail:     "Resolver returned empty address list for localhost",
			Suggestion: "Ensure loopback interfaces are configured properly on the host.",
			Duration:   lookupDuration,
		}
	}

	// Verify at least one loopback IP is present (127.0.0.1 or ::1)
	var hasLoopback bool
	var addrStrs []string
	for _, a := range addrs {
		addrStrs = append(addrStrs, a.IP.String())
		if a.IP.IsLoopback() {
			hasLoopback = true
		}
	}

	if !hasLoopback {
		return CheckResult{
			Name:       "DNS Resolution (Loopback & Local)",
			Category:   CategoryDNS,
			Status:     StatusWarn,
			Summary:    fmt.Sprintf("Localhost resolved to non-loopback IPs: %s", strings.Join(addrStrs, ", ")),
			Detail:     fmt.Sprintf("Resolved IPs: %s in %v", strings.Join(addrStrs, ", "), lookupDuration),
			Suggestion: "Verify DNS search domains and local hosts file to prevent DNS hijacking of localhost.",
			Duration:   lookupDuration,
		}
	}

	if lookupDuration > warnDNSLatency {
		return CheckResult{
			Name:       "DNS Resolution (Loopback & Local)",
			Category:   CategoryDNS,
			Status:     StatusWarn,
			Summary:    fmt.Sprintf("High DNS lookup latency for localhost (%v)", lookupDuration),
			Detail:     fmt.Sprintf("Resolved IPs: %s in %v", strings.Join(addrStrs, ", "), lookupDuration),
			Suggestion: "Local DNS resolver latency is elevated. Ensure local DNS proxy or VPN software is not intercepting loopback lookups.",
			Duration:   lookupDuration,
		}
	}

	return CheckResult{
		Name:     "DNS Resolution (Loopback & Local)",
		Category: CategoryDNS,
		Status:   StatusPass,
		Summary:  fmt.Sprintf("Resolved 'localhost' -> %s in %v", strings.Join(addrStrs, ", "), lookupDuration.Truncate(time.Microsecond)),
		Detail:   fmt.Sprintf("Resolved IP addresses: %s", strings.Join(addrStrs, ", ")),
		Duration: lookupDuration,
	}
}
