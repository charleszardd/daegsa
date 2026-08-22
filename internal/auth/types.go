package auth

import (
	"net/http"

	"github.com/charleszardd/daegsa/internal/config"
)

// AuthType aliases canonical config auth types (§6, §11).
type AuthType = string

const (
	AuthTypeNone         AuthType = config.AuthTypeNone
	AuthTypeBearer       AuthType = config.AuthTypeBearer
	AuthTypeCustomHeader AuthType = config.AuthTypeCustomHeader
	AuthTypeTokenPool    AuthType = config.AuthTypeTokenPool
	AuthTypeBasic        AuthType = config.AuthTypeBasic
)

// TokenProvider defines an interface for deterministic token assignment (§7, §11).
type TokenProvider interface {
	GetToken(workerID int) string
	TokenCount() int
}

// CookieJarManager defines an interface for managing per-VU cookie jars (§8, §11).
type CookieJarManager interface {
	GetJar(vuID int) http.CookieJar
	Enabled() bool
}

// Authenticator defines an interface for injecting credentials into HTTP requests (§4, §11).
type Authenticator interface {
	AuthenticateRequest(req *http.Request, workerID int)
	AuthMode() string
	TokenCount() int
}
