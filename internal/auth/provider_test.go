package auth_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/charleszardd/daegsa/internal/auth"
	"github.com/charleszardd/daegsa/internal/config"
)

func TestTokenPoolProvider_MinIntIsSafe(t *testing.T) {
	pool, err := auth.NewTokenPoolProvider([]string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("NewTokenPoolProvider() error = %v", err)
	}
	if got := pool.GetToken(math.MinInt); got == "" {
		t.Fatal("GetToken(math.MinInt) returned an empty token")
	}
}
func TestStaticTokenProvider(t *testing.T) {
	p := auth.NewStaticTokenProvider("secret-token-123")
	if p.TokenCount() != 1 {
		t.Errorf("expected TokenCount() = 1, got %d", p.TokenCount())
	}

	for workerID := -5; workerID <= 10; workerID++ {
		got := p.GetToken(workerID)
		if got != "secret-token-123" {
			t.Errorf("worker %d: expected 'secret-token-123', got %q", workerID, got)
		}
	}

	emptyP := auth.NewStaticTokenProvider("")
	if emptyP.TokenCount() != 0 {
		t.Errorf("expected TokenCount() = 0 for empty token, got %d", emptyP.TokenCount())
	}
	if emptyP.GetToken(0) != "" {
		t.Errorf("expected empty token, got %q", emptyP.GetToken(0))
	}
}

func TestTokenPoolProvider_ClosedAndOpenModulo(t *testing.T) {
	tokens := []string{"token-0", "token-1", "token-2"}
	pool, err := auth.NewTokenPoolProvider(tokens)
	if err != nil {
		t.Fatalf("failed to create TokenPoolProvider: %v", err)
	}

	if pool.TokenCount() != 3 {
		t.Errorf("expected TokenCount() = 3, got %d", pool.TokenCount())
	}

	// Test deterministic assignment across 10 VUs / worker lanes
	for i := 0; i < 10; i++ {
		expected := fmt.Sprintf("token-%d", i%3)
		got := pool.GetToken(i)
		if got != expected {
			t.Errorf("worker %d: expected %q, got %q", i, expected, got)
		}
	}

	// Test negative worker IDs (e.g. dispatcher worker -1)
	if got := pool.GetToken(-1); got != "token-1" {
		t.Errorf("worker -1: expected 'token-1', got %q", got)
	}
	if got := pool.GetToken(-3); got != "token-0" {
		t.Errorf("worker -3: expected 'token-0', got %q", got)
	}
}

func TestTokenPoolProvider_EdgeCases(t *testing.T) {
	// 1. Single token pool
	singlePool, err := auth.NewTokenPoolProvider([]string{"only-one"})
	if err != nil {
		t.Fatalf("unexpected error on single token pool: %v", err)
	}
	for i := -5; i <= 5; i++ {
		if got := singlePool.GetToken(i); got != "only-one" {
			t.Errorf("worker %d: expected 'only-one', got %q", i, got)
		}
	}

	// 2. Empty pool error
	_, err = auth.NewTokenPoolProvider([]string{})
	if err == nil {
		t.Errorf("expected error creating empty token pool, got nil")
	}

	// 3. Pool with empty token element
	_, err = auth.NewTokenPoolProvider([]string{"valid", "", "valid2"})
	if err == nil {
		t.Errorf("expected error creating pool with empty token element, got nil")
	}
}

func TestNewTokenProvider_Factory(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *config.AuthConfig
		wantCount int
	}{
		{
			name:      "nil config",
			cfg:       nil,
			wantCount: 0,
		},
		{
			name: "bearer auth",
			cfg: &config.AuthConfig{
				Type:  auth.AuthTypeBearer,
				Token: "tok-abc",
			},
			wantCount: 1,
		},
		{
			name: "token pool",
			cfg: &config.AuthConfig{
				Type:      auth.AuthTypeTokenPool,
				TokenPool: []string{"t1", "t2", "t3", "t4"},
			},
			wantCount: 4,
		},
		{
			name: "none auth",
			cfg: &config.AuthConfig{
				Type: auth.AuthTypeNone,
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := auth.NewTokenProvider(tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.TokenCount() != tt.wantCount {
				t.Errorf("expected TokenCount %d, got %d", tt.wantCount, p.TokenCount())
			}
		})
	}
}
