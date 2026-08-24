package config

import (
	"fmt"
	"net"
	"strings"
)

const (
	wildcardHostPrefix = "*."
	localhostHost      = "localhost"
	maxDNSNameLength   = 253
	maxDNSLabelLength  = 63
)

// NormalizeAllowedHosts validates and canonicalizes host-only allowlist entries.
// Wildcard entries are accepted only for declarative configuration; CLI authorization
// intentionally requires an exact hostname or IP address.
func NormalizeAllowedHosts(hosts []string, allowWildcard bool) ([]string, error) {
	if hosts == nil {
		return nil, nil
	}
	normalizedHosts := make([]string, 0, len(hosts))
	seenHosts := make(map[string]struct{}, len(hosts))
	for index, host := range hosts {
		normalizedHost, err := NormalizeAllowedHost(host, allowWildcard)
		if err != nil {
			return nil, fmt.Errorf("allowed_hosts[%d]: %w", index, err)
		}
		if _, exists := seenHosts[normalizedHost]; exists {
			continue
		}
		seenHosts[normalizedHost] = struct{}{}
		normalizedHosts = append(normalizedHosts, normalizedHost)
	}
	return normalizedHosts, nil
}

// NormalizeAllowedHost validates and canonicalizes a single DNS name or IP
// address. Entries must be host-only: schemes, credentials, ports, paths,
// queries, and fragments are rejected.
func NormalizeAllowedHost(host string, allowWildcard bool) (string, error) {
	normalizedHost := strings.ToLower(strings.TrimSpace(host))
	if normalizedHost == "" {
		return "", fmt.Errorf("host cannot be empty")
	}
	isWildcard := strings.HasPrefix(normalizedHost, wildcardHostPrefix)
	if isWildcard {
		if !allowWildcard {
			return "", fmt.Errorf("wildcard host %q is not permitted for CLI authorization", host)
		}
		normalizedHost = strings.TrimPrefix(normalizedHost, wildcardHostPrefix)
	}
	if strings.Contains(normalizedHost, "://") || strings.ContainsAny(normalizedHost, "/?#@") {
		return "", fmt.Errorf("host %q must not include a scheme, credentials, path, query, or fragment", host)
	}
	if strings.HasSuffix(normalizedHost, ".") {
		normalizedHost = strings.TrimSuffix(normalizedHost, ".")
	}
	if normalizedHost == "" {
		return "", fmt.Errorf("host %q is invalid", host)
	}
	if parsedIP := net.ParseIP(normalizedHost); parsedIP != nil {
		if isWildcard {
			return "", fmt.Errorf("IP address %q cannot use a wildcard", host)
		}
		return parsedIP.String(), nil
	}
	if strings.Contains(normalizedHost, ":") {
		return "", fmt.Errorf("host %q must not include a port", host)
	}
	if err := validateDNSName(normalizedHost); err != nil {
		return "", fmt.Errorf("invalid DNS host %q: %w", host, err)
	}
	if isWildcard {
		return wildcardHostPrefix + normalizedHost, nil
	}
	return normalizedHost, nil
}

// IsLoopbackHost reports whether a canonical host is an explicit loopback DNS
// name or IP literal. It deliberately does not perform DNS resolution.
func IsLoopbackHost(host string) bool {
	normalizedHost, err := NormalizeAllowedHost(host, false)
	if err != nil {
		return false
	}
	if normalizedHost == localhostHost {
		return true
	}
	parsedIP := net.ParseIP(normalizedHost)
	return parsedIP != nil && parsedIP.IsLoopback()
}

func validateDNSName(host string) error {
	if len(host) > maxDNSNameLength {
		return fmt.Errorf("name exceeds %d characters", maxDNSNameLength)
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" {
			return fmt.Errorf("name contains an empty label")
		}
		if len(label) > maxDNSLabelLength {
			return fmt.Errorf("label %q exceeds %d characters", label, maxDNSLabelLength)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("label %q starts or ends with a hyphen", label)
		}
		for _, character := range label {
			isLetter := character >= 'a' && character <= 'z'
			isDigit := character >= '0' && character <= '9'
			if !isLetter && !isDigit && character != '-' {
				return fmt.Errorf("label %q contains an invalid character", label)
			}
		}
	}
	return nil
}
