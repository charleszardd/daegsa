package config

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// RedactedPlaceholder is the canonical masked value for sensitive credentials (§6, §11).
const RedactedPlaceholder = "[REDACTED]"

// Standard sensitive header keys (case-insensitive substring and exact checks).
var sensitiveHeaderSubstrings = []string{
	"auth",
	"token",
	"secret",
	"cookie",
	"credential",
	"key",
	"password",
	"passwd",
	"session",
	"apikey",
	"api-key",
	"x-api-key",
	"x-auth-token",
	"x-token",
	"proxy-authorization",
	"set-cookie",
}

// Standard sensitive query parameter substrings.
var sensitiveQueryParamSubstrings = []string{
	"token",
	"secret",
	"key",
	"auth",
	"password",
	"passwd",
	"signature",
	"apikey",
	"api_key",
	"access_token",
	"session",
	"bearer",
	"refresh_token",
}

var (
	bearerTokenPattern    = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9_\-\.~+/]+=*`)
	basicAuthPattern      = regexp.MustCompile(`(?i)\bBasic\s+[A-Za-z0-9+/]+=*`)
	sensitiveParamPattern = regexp.MustCompile(`(?i)\b(token|key|secret|password|passwd|apikey|api_key|auth|signature|access_token|session|bearer|refresh_token)=([^& \t\r\n\x60"']+)`)
	urlQueryParamPattern  = regexp.MustCompile(`([?&])([^?&= \t\r\n\x60"']+)=([^& \t\r\n\x60"']*)`)
	urlUserInfoPattern    = regexp.MustCompile(`(?i)(https?://)([^/@\s:]+)(?::([^/@\s]+))?@`)
)

// IsSensitiveHeader reports whether a header key is considered sensitive.
func IsSensitiveHeader(headerName string) bool {
	lower := strings.ToLower(strings.TrimSpace(headerName))
	for _, sub := range sensitiveHeaderSubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// IsSensitiveQueryParam reports whether a query parameter name is considered sensitive.
func IsSensitiveQueryParam(paramName string) bool {
	lower := strings.ToLower(strings.TrimSpace(paramName))
	for _, sub := range sensitiveQueryParamSubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// RedactHeaders returns a copy of headers where sensitive header values are replaced by [REDACTED].
func RedactHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	redacted := make(map[string]string, len(headers))
	for k, v := range headers {
		if IsSensitiveHeader(k) {
			redacted[k] = RedactedPlaceholder
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

// RedactHTTPHeaders returns a cloned http.Header where sensitive header values are replaced by [REDACTED].
func RedactHTTPHeaders(headers http.Header) http.Header {
	if headers == nil {
		return nil
	}
	redacted := make(http.Header, len(headers))
	for k, vals := range headers {
		if IsSensitiveHeader(k) {
			redacted[k] = []string{RedactedPlaceholder}
		} else {
			copiedVals := make([]string, len(vals))
			copy(copiedVals, vals)
			redacted[k] = copiedVals
		}
	}
	return redacted
}

// RedactURL redacts userinfo credentials and sensitive query parameters from a raw URL string.
func RedactURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return RedactedPlaceholder
	}

	// Redact UserInfo
	if parsed.User != nil {
		parsed.User = url.UserPassword(RedactedPlaceholder, RedactedPlaceholder)
	}

	// Redact Query Parameters
	query := parsed.Query()
	if len(query) > 0 {
		modified := false
		for paramKey := range query {
			if IsSensitiveQueryParam(paramKey) {
				query.Set(paramKey, RedactedPlaceholder)
				modified = true
			}
		}
		if modified {
			parsed.RawQuery = query.Encode()
		}
	}

	return parsed.String()
}

func redactURLFragments(value string) string {
	redacted := urlUserInfoPattern.ReplaceAllString(value, "${1}"+RedactedPlaceholder+":"+RedactedPlaceholder+"@")
	return urlQueryParamPattern.ReplaceAllStringFunc(redacted, func(queryPair string) string {
		parts := urlQueryParamPattern.FindStringSubmatch(queryPair)
		if len(parts) != 4 || !IsSensitiveQueryParam(parts[2]) {
			return queryPair
		}
		return parts[1] + parts[2] + "=" + RedactedPlaceholder
	})
}

// RedactString scrubs known secrets, bearer tokens, basic auth credentials, and sensitive
// URL parameters from arbitrary text (such as error messages, log lines, and trace dumps) (§11, §12).
func RedactString(s string, knownSecrets []string) string {
	if s == "" {
		return ""
	}

	result := s

	// 1. Redact Bearer token patterns
	result = bearerTokenPattern.ReplaceAllString(result, "Bearer "+RedactedPlaceholder)

	// 2. Redact Basic auth patterns
	result = basicAuthPattern.ReplaceAllString(result, "Basic "+RedactedPlaceholder)

	// 3. Redact URL-shaped userinfo and query pairs. Compound query-key
	// sensitivity delegates to the same policy used by structured URL redaction.
	result = redactURLFragments(result)

	// 4. Redact standalone canonical sensitive parameters in error prose.
	result = sensitiveParamPattern.ReplaceAllString(result, "${1}="+RedactedPlaceholder)

	// 5. Scrub explicit known secret values.
	if len(knownSecrets) > 0 {
		// Deduplicate and filter empty
		unique := make(map[string]struct{})
		for _, sec := range knownSecrets {
			trimmed := strings.TrimSpace(sec)
			if trimmed != "" && trimmed != RedactedPlaceholder {
				unique[trimmed] = struct{}{}
			}
		}

		if len(unique) > 0 {
			secrets := make([]string, 0, len(unique))
			for sec := range unique {
				secrets = append(secrets, sec)
			}
			// Sort descending by length so longer substrings are matched first
			sort.Slice(secrets, func(i, j int) bool {
				return len(secrets[i]) > len(secrets[j])
			})

			for _, sec := range secrets {
				result = strings.ReplaceAll(result, sec, RedactedPlaceholder)
			}
		}
	}

	return result
}

// RedactError formats err by redacting sensitive secrets from err.Error() while
// preserving errors.Is and net.Error timeout classification without exposing the original error (§11).
func RedactError(err error, knownSecrets []string) error {
	if err == nil {
		return nil
	}
	redactedMsg := RedactString(err.Error(), knownSecrets)
	if redactedMsg == err.Error() {
		return err
	}
	return &redactedError{
		orig: err,
		msg:  redactedMsg,
	}
}

type redactedError struct {
	orig error
	msg  string
}

func (e *redactedError) Error() string {
	return e.msg
}

func (e *redactedError) Is(target error) bool {
	return errors.Is(e.orig, target)
}

func (e *redactedError) Timeout() bool {
	if netErr, ok := e.orig.(interface{ Timeout() bool }); ok {
		return netErr.Timeout()
	}
	return false
}

func (e *redactedError) Temporary() bool {
	if netErr, ok := e.orig.(interface{ Temporary() bool }); ok {
		return netErr.Temporary()
	}
	return false
}
