package config

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/profile"
	"github.com/charleszardd/daegsa/internal/threshold"
	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidSchemaVersion indicates unsupported or missing schema_version.
	ErrInvalidSchemaVersion = errors.New("invalid schema_version")

	// ErrConfigValidation indicates a structural or semantic validation failure.
	ErrConfigValidation = errors.New("config validation error")

	// ErrDuplicateYAMLKey indicates a duplicate key was encountered in YAML.
	ErrDuplicateYAMLKey = errors.New("duplicate YAML key")
)

// ParseAndValidateYAML parses raw YAML data into a Config, enforcing strict unknown field
// rejection, duplicate key checks, and canonical semantic validation rules.
func ParseAndValidateYAML(data []byte) (*Config, error) {
	// First: check for duplicate keys in YAML AST
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return nil, fmt.Errorf("%w: failed to parse YAML structure: %w", ErrConfigValidation, err)
	}

	if err := checkDuplicateKeys(&rootNode); err != nil {
		return nil, err
	}

	// Second: decode strictly into Config struct with KnownFields(true)
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("%w: strict YAML decode failed: %w", ErrConfigValidation, err)
	}

	// Third: apply semantic validations and defaults
	if err := ValidateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ValidateConfig validates all invariants on a parsed Config struct and applies documented defaults.
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: config cannot be nil", ErrConfigValidation)
	}

	// 1. Schema version check
	if cfg.SchemaVersion != LegacySchemaVersion && cfg.SchemaVersion != ExpectedSchemaVersion {
		return fmt.Errorf("%w: supported schema_version values are %d and %d, got %d", ErrInvalidSchemaVersion, LegacySchemaVersion, ExpectedSchemaVersion, cfg.SchemaVersion)
	}

	if cfg.SchemaVersion == ExpectedSchemaVersion && len(cfg.Load.Segments) == 0 {
		return fmt.Errorf("%w: schema_version %d requires load.segments", ErrConfigValidation, ExpectedSchemaVersion)
	}

	// 2. Name validation
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrConfigValidation)
	}

	// 3. Target validation (mutual exclusivity: either request or scenario)
	hasRequest := strings.TrimSpace(cfg.Request.URL) != ""
	hasScenario := cfg.Scenario != nil

	if hasRequest && hasScenario {
		return fmt.Errorf("%w: cannot specify both request and scenario", ErrConfigValidation)
	}
	if !hasRequest && !hasScenario {
		return fmt.Errorf("%w: must specify either request or scenario", ErrConfigValidation)
	}

	if hasRequest {
		if err := validateRequestConfig(&cfg.Request); err != nil {
			return err
		}
	}

	if hasScenario {
		if err := validateScenarioConfig(cfg.Scenario); err != nil {
			return err
		}
		if cfg.Load.Model != core.WorkloadModelClosed {
			return fmt.Errorf("%w: scenarios require load.model: closed", ErrConfigValidation)
		}
	}

	// 4. Load validation & normalization
	if err := validateLoadConfig(&cfg.Load, cfg.SchemaVersion); err != nil {
		return err
	}

	// 5. Thresholds validation
	if err := validateThresholds(cfg.Thresholds); err != nil {
		return err
	}

	// 6. Auth validation & normalization
	if err := validateAuth(&cfg.Auth); err != nil {
		return err
	}

	return nil
}

func validateRequestConfig(req *RequestConfig) error {
	trimmedURL := strings.TrimSpace(req.URL)
	if trimmedURL == "" {
		return fmt.Errorf("%w: request.url cannot be empty", ErrConfigValidation)
	}

	parsedURL, err := url.Parse(trimmedURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return fmt.Errorf("%w: request.url must be a valid absolute http/https URL, got %q", ErrConfigValidation, RedactURL(req.URL))
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		return fmt.Errorf("%w: request.method cannot be empty", ErrConfigValidation)
	}
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		req.Method = method
	default:
		return fmt.Errorf("%w: unsupported request.method %q", ErrConfigValidation, req.Method)
	}

	// Timeout
	if req.Timeout.Duration() < 0 {
		return fmt.Errorf("%w: request.timeout cannot be negative", ErrConfigValidation)
	}
	if req.Timeout.Duration() == 0 {
		req.Timeout = Duration(DefaultRequestTimeout)
	}

	// Expected statuses
	if len(req.ExpectedStatuses) == 0 {
		req.ExpectedStatuses = []int{200}
	} else {
		for _, s := range req.ExpectedStatuses {
			if s < 100 || s > 599 {
				return fmt.Errorf("%w: invalid expected_status %d, must be between 100 and 599", ErrConfigValidation, s)
			}
		}
	}

	// Response body limit
	if req.ResponseBodyLimit == "" {
		req.ResponseBodyLimit = DefaultResponseBodyLimitStr
	}
	limitBytes, err := ParseByteSize(req.ResponseBodyLimit)
	if err != nil {
		return fmt.Errorf("%w: invalid request.response_body_limit: %w", ErrConfigValidation, err)
	}
	if limitBytes <= 0 {
		return fmt.Errorf("%w: request.response_body_limit must be > 0", ErrConfigValidation)
	}
	if limitBytes > MaxResponseBodyLimitBytes {
		return fmt.Errorf("%w: request.response_body_limit %d exceeds hard safety limit %d", ErrConfigValidation, limitBytes, MaxResponseBodyLimitBytes)
	}

	// Redirects
	if req.Redirects == "" {
		req.Redirects = DefaultRedirects
	}
	switch req.Redirects {
	case RedirectPolicySameOrigin, RedirectPolicyNone, RedirectPolicyAll:
		// Valid
	default:
		return fmt.Errorf("%w: invalid request.redirects %q, must be 'same-origin', 'none', or 'all'", ErrConfigValidation, req.Redirects)
	}

	return nil
}

func validateLoadConfig(load *LoadConfig, schemaVersion int) error {
	if !load.Model.IsValid() {
		return fmt.Errorf("%w: invalid load.model %q, must be 'open' or 'closed'", ErrConfigValidation, load.Model)
	}

	if load.GracefulStop.Duration() < 0 {
		return fmt.Errorf("%w: load.graceful_stop cannot be negative", ErrConfigValidation)
	}
	if load.GracefulStop.Duration() == 0 {
		load.GracefulStop = Duration(DefaultGracefulStop)
	}

	switch load.Model {
	case core.WorkloadModelOpen:
		if load.TimeUnit.Duration() <= 0 {
			load.TimeUnit = Duration(DefaultTimeUnit)
		}
		if load.MaxInFlight <= 0 {
			return fmt.Errorf("%w: open model requires load.max_in_flight > 0, got %d", ErrConfigValidation, load.MaxInFlight)
		}
		// Rejection of closed-model fields in open model
		if load.Users != 0 {
			return fmt.Errorf("%w: open model cannot specify load.users", ErrConfigValidation)
		}
		if load.ThinkTime.Duration() != 0 {
			return fmt.Errorf("%w: open model cannot specify load.think_time", ErrConfigValidation)
		}
		if len(load.Segments) > 0 {
			if schemaVersion != ExpectedSchemaVersion {
				return fmt.Errorf("%w: load.segments requires schema_version %d", ErrConfigValidation, ExpectedSchemaVersion)
			}
			if load.Rate != 0 || load.Duration.Duration() != 0 {
				return fmt.Errorf("%w: profile mode cannot specify load.rate or load.duration", ErrConfigValidation)
			}
			if err := validateProfileSegments(load.Segments); err != nil {
				return err
			}
		} else {
			if load.Rate <= 0 {
				return fmt.Errorf("%w: open model requires load.rate > 0, got %v", ErrConfigValidation, load.Rate)
			}
			if load.Duration.Duration() <= 0 {
				return fmt.Errorf("%w: load.duration must be > 0, got %v", ErrConfigValidation, load.Duration)
			}
		}
		if _, err := CompileLoadProfile(load); err != nil {
			return err
		}

	case core.WorkloadModelClosed:
		if load.Duration.Duration() <= 0 {
			return fmt.Errorf("%w: load.duration must be > 0, got %v", ErrConfigValidation, load.Duration)
		}
		if load.Users <= 0 {
			return fmt.Errorf("%w: closed model requires load.users > 0, got %d", ErrConfigValidation, load.Users)
		}
		if load.ThinkTime.Duration() < 0 {
			return fmt.Errorf("%w: closed model think_time cannot be negative", ErrConfigValidation)
		}
		// Rejection of open-model fields in closed model
		if load.Rate != 0 {
			return fmt.Errorf("%w: closed model cannot specify load.rate", ErrConfigValidation)
		}
		if load.TimeUnit.Duration() != 0 {
			return fmt.Errorf("%w: closed model cannot specify load.time_unit", ErrConfigValidation)
		}
		if load.MaxInFlight != 0 {
			return fmt.Errorf("%w: closed model cannot specify load.max_in_flight", ErrConfigValidation)
		}
		if len(load.Segments) != 0 {
			return fmt.Errorf("%w: closed model cannot specify load.segments", ErrConfigValidation)
		}
	}

	return nil
}

func validateProfileSegments(segments []ProfileSegmentConfig) error {
	if len(segments) == 0 || len(segments) > profile.MaxSourceSegments {
		return fmt.Errorf("%w: load.segments count must be between 1 and %d", ErrConfigValidation, profile.MaxSourceSegments)
	}
	seenNames := make(map[string]struct{}, len(segments))
	stageRank := map[string]int{profile.StageWarmup: 0, profile.StageMeasured: 1, profile.StageCooldown: 2}
	previousRank := 0
	hasMeasured := false
	for index, segment := range segments {
		name := strings.TrimSpace(segment.Name)
		if name == "" || len(name) > profile.MaxSegmentNameLength {
			return fmt.Errorf("%w: load.segments[%d].name must contain 1-%d characters", ErrConfigValidation, index, profile.MaxSegmentNameLength)
		}
		if _, exists := seenNames[name]; exists {
			return fmt.Errorf("%w: duplicate profile segment name %q", ErrConfigValidation, name)
		}
		seenNames[name] = struct{}{}
		rank, validStage := stageRank[segment.Stage]
		if !validStage || rank < previousRank {
			return fmt.Errorf("%w: invalid or out-of-order stage %q in segment %q", ErrConfigValidation, segment.Stage, name)
		}
		previousRank = rank
		hasMeasured = hasMeasured || segment.Stage == profile.StageMeasured
		if segment.Duration.Duration() <= 0 {
			return fmt.Errorf("%w: segment %q duration must be positive", ErrConfigValidation, name)
		}
		isConstant := segment.Rate > 0 && segment.StartRate == 0 && segment.EndRate == 0 && segment.Steps == 0
		isRamp := segment.Rate == 0 && segment.StartRate > 0 && segment.EndRate > 0 && segment.Steps >= 2
		if !isConstant && !isRamp {
			return fmt.Errorf("%w: segment %q must define either rate or start_rate/end_rate/steps>=2", ErrConfigValidation, name)
		}
	}
	if !hasMeasured {
		return fmt.Errorf("%w: profile requires at least one measured segment", ErrConfigValidation)
	}
	return nil
}

func validateThresholds(thresholds map[string]string) error {
	for name, expr := range thresholds {
		if _, err := threshold.ParseThreshold(name, expr); err != nil {
			return fmt.Errorf("%w: %w", ErrConfigValidation, err)
		}
	}
	return nil
}

func validateAuth(auth *AuthConfig) error {
	if auth == nil {
		return nil
	}

	auth.Type = strings.ToLower(strings.TrimSpace(auth.Type))
	if auth.Type == "" {
		auth.Type = AuthTypeNone
	}

	switch auth.Type {
	case AuthTypeNone:
		// No credentials required.
	case AuthTypeBearer:
		if strings.TrimSpace(auth.Token) == "" {
			return fmt.Errorf("%w: auth.type 'bearer' requires non-empty auth.token", ErrConfigValidation)
		}
		if strings.TrimSpace(auth.HeaderName) == "" {
			auth.HeaderName = "Authorization"
		} else if !strings.EqualFold(strings.TrimSpace(auth.HeaderName), "Authorization") {
			return fmt.Errorf("%w: auth.type 'bearer' only supports the Authorization header", ErrConfigValidation)
		} else {
			auth.HeaderName = "Authorization"
		}
	case AuthTypeCustomHeader:
		if strings.TrimSpace(auth.Token) == "" {
			return fmt.Errorf("%w: auth.type 'custom_header' requires non-empty auth.token", ErrConfigValidation)
		}
		if strings.TrimSpace(auth.HeaderName) == "" {
			return fmt.Errorf("%w: auth.type 'custom_header' requires non-empty auth.header_name", ErrConfigValidation)
		}
	case AuthTypeBasic:
		if strings.TrimSpace(auth.Username) == "" {
			return fmt.Errorf("%w: auth.type 'basic' requires non-empty auth.username", ErrConfigValidation)
		}
		if strings.TrimSpace(auth.HeaderName) == "" {
			auth.HeaderName = "Authorization"
		}
	case AuthTypeTokenPool:
		if len(auth.TokenPool) == 0 {
			return fmt.Errorf("%w: auth.type 'token_pool' requires non-empty auth.token_pool", ErrConfigValidation)
		}
		for i, tok := range auth.TokenPool {
			if strings.TrimSpace(tok) == "" {
				return fmt.Errorf("%w: auth.token_pool[%d] cannot be empty", ErrConfigValidation, i)
			}
		}
		if strings.TrimSpace(auth.HeaderName) == "" {
			auth.HeaderName = "Authorization"
		}
	default:
		return fmt.Errorf("%w: unsupported auth.type %q, must be 'none', 'bearer', 'custom_header', 'token_pool', or 'basic'", ErrConfigValidation, auth.Type)
	}

	return nil
}

// checkDuplicateKeys recursively traverses a YAML Node tree to detect duplicate keys in mapping nodes.
func checkDuplicateKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := checkDuplicateKeys(child); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		seenKeys := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]

			if seenKeys[keyNode.Value] {
				return fmt.Errorf("%w: duplicate key %q found at line %d column %d", ErrDuplicateYAMLKey, keyNode.Value, keyNode.Line, keyNode.Column)
			}
			seenKeys[keyNode.Value] = true

			if err := checkDuplicateKeys(valNode); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if err := checkDuplicateKeys(item); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateScenarioConfig(scenario *ScenarioConfig) error {
	if scenario == nil {
		return fmt.Errorf("%w: scenario cannot be nil", ErrConfigValidation)
	}
	if strings.TrimSpace(scenario.Name) == "" {
		return fmt.Errorf("%w: scenario.name cannot be empty", ErrConfigValidation)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%w: scenario must define at least one step", ErrConfigValidation)
	}
	if len(scenario.Steps) > MaxScenarioSteps {
		return fmt.Errorf("%w: scenario cannot exceed %d steps, got %d", ErrConfigValidation, MaxScenarioSteps, len(scenario.Steps))
	}

	seenNames := make(map[string]struct{}, len(scenario.Steps))
	for i := range scenario.Steps {
		step := &scenario.Steps[i]
		stepName := strings.TrimSpace(step.Name)
		if stepName == "" {
			return fmt.Errorf("%w: scenario.steps[%d].name cannot be empty", ErrConfigValidation, i)
		}
		if _, exists := seenNames[stepName]; exists {
			return fmt.Errorf("%w: duplicate step name %q in scenario", ErrConfigValidation, stepName)
		}
		seenNames[stepName] = struct{}{}
		step.Name = stepName

		if err := validateStepConfig(step); err != nil {
			return fmt.Errorf("step %q: %w", stepName, err)
		}
	}
	return nil
}

func validateStepConfig(step *StepConfig) error {
	trimmedURL := strings.TrimSpace(step.URL)
	if trimmedURL == "" {
		return fmt.Errorf("%w: step url cannot be empty", ErrConfigValidation)
	}
	if !strings.HasPrefix(trimmedURL, "http://") && !strings.HasPrefix(trimmedURL, "https://") {
		return fmt.Errorf("%w: step url must be a valid absolute http/https URL, got %q", ErrConfigValidation, RedactURL(step.URL))
	}

	method := strings.ToUpper(strings.TrimSpace(step.Method))
	if method == "" {
		method = "GET"
	}
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		step.Method = method
	default:
		return fmt.Errorf("%w: unsupported step method %q", ErrConfigValidation, step.Method)
	}

	if step.Timeout.Duration() < 0 {
		return fmt.Errorf("%w: step timeout cannot be negative", ErrConfigValidation)
	}
	if step.Timeout.Duration() == 0 {
		step.Timeout = Duration(DefaultRequestTimeout)
	}

	if len(step.ExpectedStatuses) == 0 {
		step.ExpectedStatuses = []int{200}
	} else {
		for _, s := range step.ExpectedStatuses {
			if s < 100 || s > 599 {
				return fmt.Errorf("%w: invalid expected_status %d, must be between 100 and 599", ErrConfigValidation, s)
			}
		}
	}

	if step.ResponseBodyLimit == "" {
		step.ResponseBodyLimit = DefaultResponseBodyLimitStr
	}
	limitBytes, err := ParseByteSize(step.ResponseBodyLimit)
	if err != nil {
		return fmt.Errorf("%w: invalid step response_body_limit: %w", ErrConfigValidation, err)
	}
	if limitBytes <= 0 {
		return fmt.Errorf("%w: step response_body_limit must be > 0", ErrConfigValidation)
	}
	if limitBytes > MaxResponseBodyLimitBytes {
		return fmt.Errorf("%w: step response_body_limit %d exceeds hard safety limit %d", ErrConfigValidation, limitBytes, MaxResponseBodyLimitBytes)
	}

	if step.Redirects == "" {
		step.Redirects = DefaultRedirects
	}
	switch step.Redirects {
	case RedirectPolicySameOrigin, RedirectPolicyNone, RedirectPolicyAll:
		// Valid
	default:
		return fmt.Errorf("%w: invalid step redirects %q, must be 'same-origin', 'none', or 'all'", ErrConfigValidation, step.Redirects)
	}

	if step.ThinkTime.Duration() < 0 {
		return fmt.Errorf("%w: step think_time cannot be negative", ErrConfigValidation)
	}

	onFailure := strings.ToLower(strings.TrimSpace(step.OnFailure))
	if onFailure == "" {
		onFailure = OnFailureStop
	}
	switch onFailure {
	case OnFailureStop, OnFailureAbortVU, OnFailureContinue:
		step.OnFailure = onFailure
	default:
		return fmt.Errorf("%w: invalid on_failure policy %q, must be 'stop', 'abort_vu', or 'continue'", ErrConfigValidation, step.OnFailure)
	}

	for varName, rule := range step.Extract {
		trimmedVar := strings.TrimSpace(varName)
		if trimmedVar == "" {
			return fmt.Errorf("%w: extraction variable name cannot be empty", ErrConfigValidation)
		}
		from := strings.ToLower(strings.TrimSpace(rule.From))
		switch from {
		case ExtractSourceJSON, ExtractSourceJSONPath, ExtractSourceHeader, ExtractSourceCookie, ExtractSourceRegex:
			// Valid
		default:
			return fmt.Errorf("%w: invalid extraction source %q for variable %q, must be 'json', 'jsonpath', 'header', 'cookie', or 'regex'", ErrConfigValidation, rule.From, varName)
		}
		expr := strings.TrimSpace(rule.Expression)
		if expr == "" {
			return fmt.Errorf("%w: extraction expression for variable %q cannot be empty", ErrConfigValidation, varName)
		}
		if from == ExtractSourceRegex {
			if _, rErr := regexp.Compile(expr); rErr != nil {
				return fmt.Errorf("%w: invalid regex expression %q for variable %q: %w", ErrConfigValidation, expr, varName, rErr)
			}
		}
	}

	return nil
}
