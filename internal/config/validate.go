package config

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/charleszardd/daegsa/internal/core"
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
	if cfg.SchemaVersion != ExpectedSchemaVersion {
		return fmt.Errorf("%w: expected schema_version %d, got %d", ErrInvalidSchemaVersion, ExpectedSchemaVersion, cfg.SchemaVersion)
	}

	// 2. Name validation
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrConfigValidation)
	}

	// 3. Request validation & normalization
	if err := validateRequestConfig(&cfg.Request); err != nil {
		return err
	}

	// 4. Load validation & normalization
	if err := validateLoadConfig(&cfg.Load); err != nil {
		return err
	}

	// 5. Thresholds validation
	if err := validateThresholds(cfg.Thresholds); err != nil {
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
		return fmt.Errorf("%w: request.url must be a valid absolute http/https URL, got %q", ErrConfigValidation, req.URL)
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

func validateLoadConfig(load *LoadConfig) error {
	if !load.Model.IsValid() {
		return fmt.Errorf("%w: invalid load.model %q, must be 'open' or 'closed'", ErrConfigValidation, load.Model)
	}

	if load.Duration.Duration() <= 0 {
		return fmt.Errorf("%w: load.duration must be > 0, got %v", ErrConfigValidation, load.Duration)
	}

	if load.GracefulStop.Duration() < 0 {
		return fmt.Errorf("%w: load.graceful_stop cannot be negative", ErrConfigValidation)
	}
	if load.GracefulStop.Duration() == 0 {
		load.GracefulStop = Duration(DefaultGracefulStop)
	}

	switch load.Model {
	case core.WorkloadModelOpen:
		if load.Rate <= 0 {
			return fmt.Errorf("%w: open model requires load.rate > 0, got %v", ErrConfigValidation, load.Rate)
		}
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

	case core.WorkloadModelClosed:
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
