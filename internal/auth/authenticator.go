package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/charleszardd/daegsa/internal/config"
)

// RequestAuthenticator injects authentication credentials into outgoing HTTP requests (§4, §11).
type RequestAuthenticator struct {
	authType      AuthType
	headerName    string
	tokenProvider TokenProvider
	basicUsername string
	basicPassword string
}

// NewAuthenticator constructs a RequestAuthenticator from an AuthConfig.
func NewAuthenticator(cfg *config.AuthConfig) (*RequestAuthenticator, error) {
	if cfg == nil {
		return &RequestAuthenticator{
			authType:      AuthTypeNone,
			tokenProvider: NewStaticTokenProvider(""),
		}, nil
	}

	authType := strings.ToLower(strings.TrimSpace(cfg.Type))
	if authType == "" {
		authType = AuthTypeNone
	}

	provider, err := NewTokenProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create token provider: %w", err)
	}

	headerName := cfg.HeaderName
	if headerName == "" {
		headerName = "Authorization"
	}

	return &RequestAuthenticator{
		authType:      authType,
		headerName:    headerName,
		tokenProvider: provider,
		basicUsername: cfg.Username,
		basicPassword: cfg.Password,
	}, nil
}

// AuthenticateRequest applies the appropriate credentials to req according to workerID.
func (a *RequestAuthenticator) AuthenticateRequest(req *http.Request, workerID int) {
	if a == nil || req == nil {
		return
	}

	switch a.authType {
	case AuthTypeBearer:
		token := a.tokenProvider.GetToken(workerID)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	case AuthTypeCustomHeader:
		token := a.tokenProvider.GetToken(workerID)
		if token != "" && a.headerName != "" {
			req.Header.Set(a.headerName, token)
		}
	case AuthTypeBasic:
		req.SetBasicAuth(a.basicUsername, a.basicPassword)
	case AuthTypeTokenPool:
		token := a.tokenProvider.GetToken(workerID)
		if token != "" {
			if strings.EqualFold(a.headerName, "Authorization") || a.headerName == "" {
				req.Header.Set("Authorization", "Bearer "+token)
			} else {
				req.Header.Set(a.headerName, token)
			}
		}
	case AuthTypeNone:
		// No authentication applied
	}
}

// AuthMode returns the configured authentication mode string.
func (a *RequestAuthenticator) AuthMode() string {
	if a == nil {
		return AuthTypeNone
	}
	return a.authType
}

// TokenCount returns the number of tokens managed by the authenticator.
func (a *RequestAuthenticator) TokenCount() int {
	if a == nil || a.tokenProvider == nil {
		return 0
	}
	return a.tokenProvider.TokenCount()
}
