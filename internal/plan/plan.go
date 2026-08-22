package plan

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/charleszardd/daegsa/internal/auth"
	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/profile"
	"github.com/charleszardd/daegsa/internal/safety"
	"github.com/charleszardd/daegsa/internal/threshold"
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
	Thresholds         []*threshold.Threshold
	AllowedHosts       []string
	AllowNonIdempotent bool
	ResolvedIPs        []net.IP
	AuthType           string
	AuthHeaderName     string
	TokenProvider      auth.TokenProvider
	Authenticator      *auth.RequestAuthenticator
	JarManager         *auth.VUJarManager
	CookieJarEnabled   bool
	KnownSecrets       []string
	CompiledSegments   []profile.Segment
	PeakTargetRPS      float64
}

// BuildPlan constructs a deeply cloned, immutable Plan from a validated Config and PreflightResult (§4, §7).
func BuildPlan(cfg *config.Config, preflight *safety.PreflightResult) (*Plan, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cannot build plan from nil config")
	}
	if preflight == nil {
		return nil, fmt.Errorf("cannot build plan from nil preflight result")
	}

	var compiledProfile *profile.Compilation
	if cfg.Load.Model == core.WorkloadModelOpen {
		var compileErr error
		compiledProfile, compileErr = config.CompileLoadProfile(&cfg.Load)
		if compileErr != nil {
			return nil, fmt.Errorf("failed to compile load profile: %w", compileErr)
		}
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

	// Parse and deep clone Thresholds
	parsedThresholds, err := threshold.ParseThresholds(cfg.Thresholds)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan thresholds: %w", err)
	}
	clonedThresholds := make([]*threshold.Threshold, len(parsedThresholds))
	for i, t := range parsedThresholds {
		clonedThresholds[i] = t.Clone()
	}

	// Initialize authentication and per-VU session manager
	authenticator, err := auth.NewAuthenticator(&cfg.Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to build authenticator: %w", err)
	}

	tokenProvider, err := auth.NewTokenProvider(&cfg.Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to build token provider: %w", err)
	}

	numVUs := int(cfg.Load.Users)
	if cfg.Load.Model == core.WorkloadModelOpen {
		numVUs = int(cfg.Load.MaxInFlight)
	}
	jarManager, err := auth.NewVUJarManager(cfg.Auth.CookieJar, numVUs)
	if err != nil {
		return nil, fmt.Errorf("failed to build cookie jar manager: %w", err)
	}

	// Collect known secrets for redaction/scrubbing
	var knownSecrets []string
	if cfg.Auth.Token != "" {
		knownSecrets = append(knownSecrets, cfg.Auth.Token)
	}
	if cfg.Auth.Password != "" {
		knownSecrets = append(knownSecrets, cfg.Auth.Password)
	}
	for _, tok := range cfg.Auth.TokenPool {
		if tok != "" {
			knownSecrets = append(knownSecrets, tok)
		}
	}
	for k, v := range cfg.Request.Headers {
		if config.IsSensitiveHeader(k) && v != "" {
			knownSecrets = append(knownSecrets, v)
		}
	}

	var compiledSegments []profile.Segment
	var peakTargetRPS float64
	var planDuration = cfg.Load.Duration.Duration()
	if compiledProfile != nil {
		compiledSegments = append([]profile.Segment(nil), compiledProfile.Segments...)
		peakTargetRPS = compiledProfile.PeakTargetRPS
		planDuration = compiledProfile.TotalDuration
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
		Duration:           planDuration,
		GracefulStop:       cfg.Load.GracefulStop.Duration(),
		Users:              cfg.Load.Users,
		ThinkTime:          cfg.Load.ThinkTime.Duration(),
		Treat429AsExpected: cfg.RateLimit.Treat429AsExpected,
		Thresholds:         clonedThresholds,
		AllowedHosts:       allowedHosts,
		AllowNonIdempotent: cfg.Safety.AllowNonIdempotent,
		ResolvedIPs:        resolvedIPs,
		AuthType:           authenticator.AuthMode(),
		AuthHeaderName:     cfg.Auth.HeaderName,
		TokenProvider:      tokenProvider,
		Authenticator:      authenticator,
		JarManager:         jarManager,
		CookieJarEnabled:   cfg.Auth.CookieJar,
		KnownSecrets:       knownSecrets,
		CompiledSegments:   compiledSegments,
		PeakTargetRPS:      peakTargetRPS,
	}

	return p, nil
}

// TargetRPS returns the computed target arrival rate per second for open model plans (§7).
func (p *Plan) TargetRPS() float64 {
	if p == nil {
		return 0.0
	}
	if p.PeakTargetRPS > 0 {
		return p.PeakTargetRPS
	}
	if p.TimeUnit <= 0 {
		return 0.0
	}
	return p.Rate / p.TimeUnit.Seconds()
}

// ToEvaluationContext constructs the threshold.EvaluationContext for evaluating thresholds against this Plan (§10).
func (p *Plan) ToEvaluationContext() threshold.EvaluationContext {
	if p == nil {
		return threshold.EvaluationContext{}
	}
	return threshold.EvaluationContext{
		TargetRPS:   p.TargetRPS(),
		MaxInFlight: p.MaxInFlight,
	}
}
