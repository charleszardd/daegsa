package plan

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/safety"
)

// Plan represents an immutable, validated, fully resolved execution manifest (§4, §6, §7).
type Plan struct {
	Name               string
	SchemaVersion      int
	Fingerprint        string
	TargetURL          *url.URL
	Method             string
	Headers            http.Header
	Body               []byte
	ExpectedStatuses   []int
	RequestTimeout     time.Duration
	ResponseBodyLimit  int64
	RedirectPolicy     string
	Model              core.WorkloadModel
	Rate               float64
	TimeUnit           time.Duration
	MaxInFlight        int64
	Duration           time.Duration
	GracefulStop       time.Duration
	Users              int64
	ThinkTime          time.Duration
	Treat429AsExpected bool
	AllowedHosts       []string
	AllowNonIdempotent bool
	ResolvedIPs        []net.IP
}

// BuildPlan constructs a deeply cloned, immutable Plan from a validated Config and PreflightResult (§4, §7).
func BuildPlan(cfg *config.Config, preflight *safety.PreflightResult) (*Plan, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cannot build plan from nil config")
	}
	if preflight == nil {
		return nil, fmt.Errorf("cannot build plan from nil preflight result")
	}

	fingerprint, err := config.ComputeFingerprint(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to compute configuration fingerprint: %w", err)
	}

	bodyLimitBytes, err := config.ParseByteSize(cfg.Request.ResponseBodyLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body limit: %w", err)
	}

	// Deep clone TargetURL
	var targetURL *url.URL
	if preflight.TargetURL != nil {
		u := *preflight.TargetURL
		targetURL = &u
	} else {
		parsed, parseErr := url.Parse(cfg.Request.URL)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid target URL: %w", parseErr)
		}
		targetURL = parsed
	}

	// Deep clone Headers
	headers := make(http.Header, len(cfg.Request.Headers))
	for k, v := range cfg.Request.Headers {
		headers.Set(k, v)
	}

	// Deep clone ExpectedStatuses
	expectedStatuses := make([]int, len(cfg.Request.ExpectedStatuses))
	copy(expectedStatuses, cfg.Request.ExpectedStatuses)

	// Deep clone AllowedHosts
	allowedHosts := make([]string, len(cfg.Safety.AllowedHosts))
	copy(allowedHosts, cfg.Safety.AllowedHosts)

	// Deep clone ResolvedIPs
	resolvedIPs := make([]net.IP, len(preflight.ResolvedIPs))
	for i, ip := range preflight.ResolvedIPs {
		clonedIP := make(net.IP, len(ip))
		copy(clonedIP, ip)
		resolvedIPs[i] = clonedIP
	}

	p := &Plan{
		Name:               cfg.Name,
		SchemaVersion:      cfg.SchemaVersion,
		Fingerprint:        fingerprint,
		TargetURL:          targetURL,
		Method:             cfg.Request.Method,
		Headers:            headers,
		ExpectedStatuses:   expectedStatuses,
		RequestTimeout:     cfg.Request.Timeout.Duration(),
		ResponseBodyLimit:  bodyLimitBytes,
		RedirectPolicy:     cfg.Request.Redirects,
		Model:              cfg.Load.Model,
		Rate:               cfg.Load.Rate,
		TimeUnit:           cfg.Load.TimeUnit.Duration(),
		MaxInFlight:        cfg.Load.MaxInFlight,
		Duration:           cfg.Load.Duration.Duration(),
		GracefulStop:       cfg.Load.GracefulStop.Duration(),
		Users:              cfg.Load.Users,
		ThinkTime:          cfg.Load.ThinkTime.Duration(),
		Treat429AsExpected: cfg.RateLimit.Treat429AsExpected,
		AllowedHosts:       allowedHosts,
		AllowNonIdempotent: cfg.Safety.AllowNonIdempotent,
		ResolvedIPs:        resolvedIPs,
	}

	return p, nil
}
