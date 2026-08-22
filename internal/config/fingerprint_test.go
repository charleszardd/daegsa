package config

import (
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
)

func TestComputeFingerprint_Deterministic(t *testing.T) {
	cfg1 := &Config{
		SchemaVersion: 1,
		Name:          "bench",
		Request: RequestConfig{
			URL:     "https://api.example.com/items",
			Method:  "GET",
			Timeout: Duration(5 * time.Second),
		},
		Load: LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        100,
			TimeUnit:    Duration(1 * time.Second),
			MaxInFlight: 500,
			Duration:    Duration(30 * time.Second),
		},
	}

	cfg2 := &Config{
		SchemaVersion: 1,
		Name:          "bench",
		Request: RequestConfig{
			URL:     "https://api.example.com/items",
			Method:  "GET",
			Timeout: Duration(5 * time.Second),
		},
		Load: LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        100,
			TimeUnit:    Duration(1 * time.Second),
			MaxInFlight: 500,
			Duration:    Duration(30 * time.Second),
		},
	}

	fp1, err1 := ComputeFingerprint(cfg1)
	if err1 != nil {
		t.Fatalf("failed to compute fp1: %v", err1)
	}

	fp2, err2 := ComputeFingerprint(cfg2)
	if err2 != nil {
		t.Fatalf("failed to compute fp2: %v", err2)
	}

	if fp1 == "" {
		t.Errorf("fingerprint is empty")
	}
	if fp1 != fp2 {
		t.Errorf("expected deterministic fingerprints, got %q vs %q", fp1, fp2)
	}
}

func TestComputeFingerprint_SecretIndependence(t *testing.T) {
	cfgA := &Config{
		SchemaVersion: 1,
		Name:          "bench",
		Request: RequestConfig{
			URL:    "https://api.example.com/items?token=secret111",
			Method: "GET",
			Headers: map[string]string{
				"Authorization": "Bearer secret_token_AAA",
				"X-Api-Key":     "key_aaa",
			},
		},
		Load: LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        100,
			Duration:    Duration(30 * time.Second),
			MaxInFlight: 500,
		},
	}

	cfgB := &Config{
		SchemaVersion: 1,
		Name:          "bench",
		Request: RequestConfig{
			URL:    "https://api.example.com/items?token=secret222",
			Method: "GET",
			Headers: map[string]string{
				"Authorization": "Bearer secret_token_BBB",
				"X-Api-Key":     "key_bbb",
			},
		},
		Load: LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        100,
			Duration:    Duration(30 * time.Second),
			MaxInFlight: 500,
		},
	}

	fpA, errA := ComputeFingerprint(cfgA)
	if errA != nil {
		t.Fatalf("failed to compute fpA: %v", errA)
	}

	fpB, errB := ComputeFingerprint(cfgB)
	if errB != nil {
		t.Fatalf("failed to compute fpB: %v", errB)
	}

	if fpA != fpB {
		t.Errorf("fingerprints differ across secret rotations: %q vs %q", fpA, fpB)
	}
}

func TestComputeFingerprint_AuthSecretInvariance(t *testing.T) {
	cfg1 := &Config{
		SchemaVersion: 1,
		Name:          "auth-bench",
		Request: RequestConfig{
			URL:    "https://api.example.com/data",
			Method: "GET",
		},
		Load: LoadConfig{
			Model:    core.WorkloadModelClosed,
			Users:    10,
			Duration: Duration(10 * time.Second),
		},
		Auth: AuthConfig{
			Type:       AuthTypeBearer,
			Token:      "initial_secret_token_12345",
			HeaderName: "Authorization",
			CookieJar:  true,
		},
	}

	cfg2 := &Config{
		SchemaVersion: 1,
		Name:          "auth-bench",
		Request: RequestConfig{
			URL:    "https://api.example.com/data",
			Method: "GET",
		},
		Load: LoadConfig{
			Model:    core.WorkloadModelClosed,
			Users:    10,
			Duration: Duration(10 * time.Second),
		},
		Auth: AuthConfig{
			Type:       AuthTypeBearer,
			Token:      "rotated_secret_token_98765",
			HeaderName: "Authorization",
			CookieJar:  true,
		},
	}

	fp1, err1 := ComputeFingerprint(cfg1)
	if err1 != nil {
		t.Fatalf("failed to compute fp1: %v", err1)
	}

	fp2, err2 := ComputeFingerprint(cfg2)
	if err2 != nil {
		t.Fatalf("failed to compute fp2: %v", err2)
	}

	if fp1 != fp2 {
		t.Errorf("fingerprints differ after bearer token rotation: %q vs %q", fp1, fp2)
	}

	// Also test TokenPool rotation
	cfgPool1 := &Config{
		SchemaVersion: 1,
		Name:          "pool-bench",
		Request: RequestConfig{
			URL:    "https://api.example.com/data",
			Method: "GET",
		},
		Load: LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        50,
			MaxInFlight: 100,
			Duration:    Duration(10 * time.Second),
		},
		Auth: AuthConfig{
			Type:      AuthTypeTokenPool,
			TokenPool: []string{"tok_AAA_111", "tok_BBB_222", "tok_CCC_333"},
		},
	}

	cfgPool2 := &Config{
		SchemaVersion: 1,
		Name:          "pool-bench",
		Request: RequestConfig{
			URL:    "https://api.example.com/data",
			Method: "GET",
		},
		Load: LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        50,
			MaxInFlight: 100,
			Duration:    Duration(10 * time.Second),
		},
		Auth: AuthConfig{
			Type:      AuthTypeTokenPool,
			TokenPool: []string{"tok_XXX_777", "tok_YYY_888", "tok_ZZZ_999"},
		},
	}

	fpPool1, errP1 := ComputeFingerprint(cfgPool1)
	if errP1 != nil {
		t.Fatalf("failed to compute fpPool1: %v", errP1)
	}

	fpPool2, errP2 := ComputeFingerprint(cfgPool2)
	if errP2 != nil {
		t.Fatalf("failed to compute fpPool2: %v", errP2)
	}

	if fpPool1 != fpPool2 {
		t.Errorf("fingerprints differ after token pool rotation: %q vs %q", fpPool1, fpPool2)
	}
}
