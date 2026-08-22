package auth

import (
	"net/http"

	"github.com/charleszardd/daegsa/internal/config"
)

// RedactedPlaceholder is the canonical masked value for sensitive credentials (§6, §11).
const RedactedPlaceholder = config.RedactedPlaceholder

// IsSensitiveHeader reports whether a header key is considered sensitive.
func IsSensitiveHeader(headerName string) bool {
	return config.IsSensitiveHeader(headerName)
}

// IsSensitiveQueryParam reports whether a query parameter name is considered sensitive.
func IsSensitiveQueryParam(paramName string) bool {
	return config.IsSensitiveQueryParam(paramName)
}

// RedactHeaders returns a copy of headers where sensitive header values are replaced by [REDACTED].
func RedactHeaders(headers map[string]string) map[string]string {
	return config.RedactHeaders(headers)
}

// RedactHTTPHeaders returns a cloned http.Header where sensitive header values are replaced by [REDACTED].
func RedactHTTPHeaders(headers http.Header) http.Header {
	return config.RedactHTTPHeaders(headers)
}

// RedactURL redacts userinfo credentials and sensitive query parameters from a raw URL string.
func RedactURL(rawURL string) string {
	return config.RedactURL(rawURL)
}

// RedactString scrubs known secrets, bearer tokens, basic auth credentials, and sensitive
// URL parameters from arbitrary text (such as error messages, log lines, and trace dumps) (§11, §12).
func RedactString(s string, knownSecrets []string) string {
	return config.RedactString(s, knownSecrets)
}

// RedactError formats err by redacting sensitive secrets from err.Error() (§11).
func RedactError(err error, knownSecrets []string) error {
	return config.RedactError(err, knownSecrets)
}
