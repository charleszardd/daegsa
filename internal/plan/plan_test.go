package plan

import (
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/safety"
)

func TestBuildPlan_Immutability(t *testing.T) {
	cfg := &config.Config{
		SchemaVersion: 1,
		Name:          "immutable-plan-test",
		Request: config.RequestConfig{
			URL:    "https://api.example.com/items",
			Method: "GET",
			Headers: map[string]string{
				"Authorization": "Bearer secret123",
				"X-Custom":      "val1",
			},
			ExpectedStatuses:  []int{200, 201},
			Timeout:           config.Duration(5 * time.Second),
			ResponseBodyLimit: "1MiB",
			Redirects:         "same-origin",
		},
		Load: config.LoadConfig{
			Model:        core.WorkloadModelOpen,
			Rate:         100,
			TimeUnit:     config.Duration(1 * time.Second),
			MaxInFlight:  500,
			Duration:     config.Duration(30 * time.Second),
			GracefulStop: config.Duration(10 * time.Second),
		},
		Safety: config.SafetyConfig{
			AllowedHosts:       []string{"api.example.com"},
			AllowNonIdempotent: false,
		},
	}

	targetURL, _ := url.Parse("https://api.example.com/items")
	preflight := &safety.PreflightResult{
		TargetURL:   targetURL,
		Method:      "GET",
		ResolvedIPs: []net.IP{net.ParseIP("192.0.2.1")},
		Authorized:  true,
	}

	p, err := BuildPlan(cfg, preflight)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	// Mutate source config and preflight
	cfg.Request.URL = "https://mutated.example.com"
	cfg.Request.Headers["X-Custom"] = "mutated"
	cfg.Request.ExpectedStatuses[0] = 999
	cfg.Safety.AllowedHosts[0] = "mutated.com"
	preflight.ResolvedIPs[0] = net.ParseIP("10.0.0.1")

	// Verify plan was unaffected
	if p.TargetURL.String() != "https://api.example.com/items" {
		t.Errorf("Plan TargetURL mutated: %s", p.TargetURL.String())
	}
	if p.Headers.Get("X-Custom") != "val1" {
		t.Errorf("Plan Headers mutated: %s", p.Headers.Get("X-Custom"))
	}
	if p.ExpectedStatuses[0] != 200 {
		t.Errorf("Plan ExpectedStatuses mutated: %d", p.ExpectedStatuses[0])
	}
	if p.AllowedHosts[0] != "api.example.com" {
		t.Errorf("Plan AllowedHosts mutated: %s", p.AllowedHosts[0])
	}
	if p.ResolvedIPs[0].String() != "192.0.2.1" {
		t.Errorf("Plan ResolvedIPs mutated: %s", p.ResolvedIPs[0].String())
	}
}

func TestFormatPlanSummary_RedactsSecrets(t *testing.T) {
	parsedURL, _ := url.Parse("https://api.example.com/v1/items?token=my_secret_token")
	p := &Plan{
		Name:              "summary-test",
		SchemaVersion:     1,
		Fingerprint:       "a1b2c3d4e5f60718293a4b5c6d7e8f90",
		TargetURL:         parsedURL,
		Method:            "GET",
		Headers:           map[string][]string{"Authorization": {"Bearer supersecret"}, "Accept": {"application/json"}},
		ExpectedStatuses:  []int{200},
		RequestTimeout:    5 * time.Second,
		ResponseBodyLimit: 1048576,
		RedirectPolicy:    "same-origin",
		Model:             core.WorkloadModelOpen,
		Rate:              50,
		TimeUnit:          time.Second,
		MaxInFlight:       200,
		Duration:          30 * time.Second,
		GracefulStop:      5 * time.Second,
		AllowedHosts:      []string{"api.example.com"},
		ResolvedIPs:       []net.IP{net.ParseIP("127.0.0.1")},
	}

	summary := FormatPlanSummary(p)

	if strings.Contains(summary, "supersecret") {
		t.Errorf("secret token leaked in summary: %s", summary)
	}
	if strings.Contains(summary, "my_secret_token") {
		t.Errorf("secret query param leaked in summary: %s", summary)
	}
	if !strings.Contains(summary, "[REDACTED]") && !strings.Contains(summary, "%5BREDACTED%5D") {
		t.Errorf("expected [REDACTED] in summary, got:\n%s", summary)
	}
	if !strings.Contains(summary, "DAEGSA EXECUTION PLAN") {
		t.Errorf("missing headline in summary")
	}
}
