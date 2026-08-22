package auth

import (
	"fmt"

	"github.com/charleszardd/daegsa/internal/config"
)

// StaticTokenProvider returns a static token regardless of workerID (§11).
type StaticTokenProvider struct {
	token string
}

// NewStaticTokenProvider constructs a StaticTokenProvider.
func NewStaticTokenProvider(token string) *StaticTokenProvider {
	return &StaticTokenProvider{token: token}
}

// GetToken returns the static token for any worker index.
func (p *StaticTokenProvider) GetToken(workerID int) string {
	if p == nil {
		return ""
	}
	return p.token
}

// TokenCount returns 1 if a token is configured, or 0 otherwise.
func (p *StaticTokenProvider) TokenCount() int {
	if p == nil || p.token == "" {
		return 0
	}
	return 1
}

// TokenPoolProvider deterministically maps worker and virtual user IDs to tokens using modulo arithmetic (§7, §11).
type TokenPoolProvider struct {
	tokens []string
}

// NewTokenPoolProvider constructs a TokenPoolProvider with deep-copied tokens.
func NewTokenPoolProvider(tokens []string) (*TokenPoolProvider, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("token pool cannot be empty")
	}
	cloned := make([]string, len(tokens))
	for i, t := range tokens {
		if t == "" {
			return nil, fmt.Errorf("token at index %d cannot be empty", i)
		}
		cloned[i] = t
	}
	return &TokenPoolProvider{tokens: cloned}, nil
}

// GetToken returns the token assigned to workerID via deterministic modulo indexing.
// Negative worker IDs (such as dispatcher internal index -1) are handled defensively.
func (p *TokenPoolProvider) GetToken(workerID int) string {
	if p == nil || len(p.tokens) == 0 {
		return ""
	}
	idx := workerID % len(p.tokens)
	if idx < 0 {
		idx = -idx
	}
	return p.tokens[idx]
}

// TokenCount returns the number of tokens in the pool.
func (p *TokenPoolProvider) TokenCount() int {
	if p == nil {
		return 0
	}
	return len(p.tokens)
}

// NewTokenProvider constructs an appropriate TokenProvider based on the supplied AuthConfig.
func NewTokenProvider(cfg *config.AuthConfig) (TokenProvider, error) {
	if cfg == nil {
		return NewStaticTokenProvider(""), nil
	}
	switch cfg.Type {
	case AuthTypeTokenPool:
		return NewTokenPoolProvider(cfg.TokenPool)
	case AuthTypeBearer, AuthTypeCustomHeader:
		return NewStaticTokenProvider(cfg.Token), nil
	default:
		return NewStaticTokenProvider(""), nil
	}
}
