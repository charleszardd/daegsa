package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/profile"
	"gopkg.in/yaml.v3"
)

const (
	LegacySchemaVersion   = 1
	ExpectedSchemaVersion = 2
)

// Default configuration constants.
const (
	DefaultRequestTimeout       = 10 * time.Second
	DefaultGracefulStop         = 10 * time.Second
	DefaultTimeUnit             = 1 * time.Second
	DefaultResponseBodyLimit    = 1024 * 1024 // 1 MiB
	DefaultResponseBodyLimitStr = "1MiB"
	DefaultRedirects            = "same-origin"
	MaxResponseBodyLimitBytes   = 50 * 1024 * 1024 // 50 MiB hard safety ceiling
)

// Allowed redirect policies (§6, §8).
const (
	RedirectPolicySameOrigin = core.RedirectPolicySameOrigin
	RedirectPolicyNone       = core.RedirectPolicyNone
	RedirectPolicyAll        = core.RedirectPolicyAll
)

// Supported authentication types (§6, §11).
const (
	AuthTypeNone         = "none"
	AuthTypeBearer       = "bearer"
	AuthTypeCustomHeader = "custom_header"
	AuthTypeTokenPool    = "token_pool"
	AuthTypeBasic        = "basic"
)

// Duration wraps time.Duration to provide string parsing in YAML and JSON (e.g. "5s", "250ms").
type Duration time.Duration

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// String returns formatted duration string.
func (d Duration) String() string {
	return time.Duration(d).String()
}

// UnmarshalYAML unmarshals string or numeric duration.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected duration scalar, got kind %d", value.Kind)
	}
	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		// try integer nanoseconds or seconds
		num, numErr := strconv.ParseInt(value.Value, 10, 64)
		if numErr == nil {
			*d = Duration(time.Duration(num) * time.Millisecond)
			return nil
		}
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	*d = Duration(parsed)
	return nil
}

// MarshalYAML marshals duration as string.
func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

// UnmarshalJSON unmarshals JSON string or number.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch val := v.(type) {
	case string:
		parsed, err := time.ParseDuration(val)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", val, err)
		}
		*d = Duration(parsed)
		return nil
	case float64:
		*d = Duration(time.Duration(val) * time.Millisecond)
		return nil
	default:
		return errors.New("invalid type for duration")
	}
}

// MarshalJSON marshals duration as JSON string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// Scenario and extraction constants.
const (
	MaxScenarioSteps = 50

	OnFailureStop     = "stop"
	OnFailureAbortVU  = "abort_vu"
	OnFailureContinue = "continue"

	ExtractSourceJSON     = "json"
	ExtractSourceJSONPath = "jsonpath"
	ExtractSourceHeader   = "header"
	ExtractSourceCookie   = "cookie"
	ExtractSourceRegex    = "regex"
)

// Config represents the top-level v1 configuration document (§6).
type Config struct {
	SchemaVersion int               `yaml:"schema_version" json:"schema_version"`
	Name          string            `yaml:"name" json:"name"`
	Request       RequestConfig     `yaml:"request,omitempty" json:"request,omitempty"`
	Scenario      *ScenarioConfig   `yaml:"scenario,omitempty" json:"scenario,omitempty"`
	Load          LoadConfig        `yaml:"load" json:"load"`
	RateLimit     RateLimitConfig   `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`
	Auth          AuthConfig        `yaml:"auth,omitempty" json:"auth,omitempty"`
	Thresholds    map[string]string `yaml:"thresholds,omitempty" json:"thresholds,omitempty"`
	Safety        SafetyConfig      `yaml:"safety,omitempty" json:"safety,omitempty"`
}

// ScenarioConfig defines a multi-step workflow scenario (§2, §6, §11).
type ScenarioConfig struct {
	Name  string       `yaml:"name" json:"name"`
	Steps []StepConfig `yaml:"steps" json:"steps"`
}

// ExtractRuleConfig defines a response extraction rule (§6, §11).
type ExtractRuleConfig struct {
	From       string `yaml:"from" json:"from"`
	Expression string `yaml:"expression" json:"expression"`
}

// StepConfig defines an individual HTTP step within a multi-step scenario (§6).
type StepConfig struct {
	Name              string                       `yaml:"name" json:"name"`
	URL               string                       `yaml:"url" json:"url"`
	Method            string                       `yaml:"method" json:"method"`
	Headers           map[string]string            `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body              string                       `yaml:"body,omitempty" json:"body,omitempty"`
	ExpectedStatuses  []int                        `yaml:"expected_statuses,omitempty" json:"expected_statuses,omitempty"`
	Timeout           Duration                     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	ResponseBodyLimit string                       `yaml:"response_body_limit,omitempty" json:"response_body_limit,omitempty"`
	Redirects         string                       `yaml:"redirects,omitempty" json:"redirects,omitempty"`
	ThinkTime         Duration                     `yaml:"think_time,omitempty" json:"think_time,omitempty"`
	Extract           map[string]ExtractRuleConfig `yaml:"extract,omitempty" json:"extract,omitempty"`
	OnFailure         string                       `yaml:"on_failure,omitempty" json:"on_failure,omitempty"`
}

// AuthConfig defines static authentication and credential configuration (§6, §11).
type AuthConfig struct {
	Type       string   `yaml:"type,omitempty" json:"type,omitempty"`
	Token      string   `yaml:"token,omitempty" json:"token,omitempty"`
	HeaderName string   `yaml:"header_name,omitempty" json:"header_name,omitempty"`
	Username   string   `yaml:"username,omitempty" json:"username,omitempty"`
	Password   string   `yaml:"password,omitempty" json:"password,omitempty"`
	TokenPool  []string `yaml:"token_pool,omitempty" json:"token_pool,omitempty"`
	CookieJar  bool     `yaml:"cookie_jar,omitempty" json:"cookie_jar,omitempty"`
}

// RequestConfig defines HTTP target and execution options (§6).
type RequestConfig struct {
	URL               string            `yaml:"url,omitempty" json:"url,omitempty"`
	Method            string            `yaml:"method,omitempty" json:"method,omitempty"`
	Headers           map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	ExpectedStatuses  []int             `yaml:"expected_statuses,omitempty" json:"expected_statuses,omitempty"`
	Timeout           Duration          `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	ResponseBodyLimit string            `yaml:"response_body_limit,omitempty" json:"response_body_limit,omitempty"`
	Redirects         string            `yaml:"redirects,omitempty" json:"redirects,omitempty"`
}

// LoadConfig defines the workload model parameters (§2, §6).
type LoadConfig struct {
	Model        core.WorkloadModel     `yaml:"model" json:"model"`
	Rate         float64                `yaml:"rate,omitempty" json:"rate,omitempty"`
	TimeUnit     Duration               `yaml:"time_unit,omitempty" json:"time_unit,omitempty"`
	MaxInFlight  int64                  `yaml:"max_in_flight,omitempty" json:"max_in_flight,omitempty"`
	Duration     Duration               `yaml:"duration,omitempty" json:"duration,omitempty"`
	GracefulStop Duration               `yaml:"graceful_stop,omitempty" json:"graceful_stop,omitempty"`
	Users        int64                  `yaml:"users,omitempty" json:"users,omitempty"`
	ThinkTime    Duration               `yaml:"think_time,omitempty" json:"think_time,omitempty"`
	Segments     []ProfileSegmentConfig `yaml:"segments,omitempty" json:"segments,omitempty"`
}

// ProfileSegmentConfig is one source segment in a schema-v2 open workload profile.
type ProfileSegmentConfig struct {
	Name      string   `yaml:"name" json:"name"`
	Stage     string   `yaml:"stage" json:"stage"`
	Duration  Duration `yaml:"duration" json:"duration"`
	Rate      float64  `yaml:"rate,omitempty" json:"rate,omitempty"`
	StartRate float64  `yaml:"start_rate,omitempty" json:"start_rate,omitempty"`
	EndRate   float64  `yaml:"end_rate,omitempty" json:"end_rate,omitempty"`
	Steps     int      `yaml:"steps,omitempty" json:"steps,omitempty"`
}

// CompileLoadProfile returns the immutable constant-rate profile for an open load.
func CompileLoadProfile(load *LoadConfig) (*profile.Compilation, error) {
	if load == nil {
		return nil, fmt.Errorf("%w: load cannot be nil", ErrConfigValidation)
	}
	source := make([]profile.SourceSegment, 0, len(load.Segments))
	if len(load.Segments) == 0 {
		source = append(source, profile.SourceSegment{Name: "measured", Stage: profile.StageMeasured, Duration: load.Duration.Duration(), Rate: load.Rate})
	} else {
		for _, segment := range load.Segments {
			source = append(source, profile.SourceSegment{
				Name: segment.Name, Stage: segment.Stage, Duration: segment.Duration.Duration(),
				Rate: segment.Rate, StartRate: segment.StartRate, EndRate: segment.EndRate, Steps: segment.Steps,
			})
		}
	}
	compiled, err := profile.Compile(source, load.TimeUnit.Duration())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfigValidation, err)
	}
	return compiled, nil
}

// RateLimitConfig defines 429 rate-limiting analysis rules (§6, §14).
type RateLimitConfig struct {
	Treat429AsExpected bool `yaml:"treat_429_as_expected,omitempty" json:"treat_429_as_expected,omitempty"`
}

// SafetyConfig defines preflight host allowlists and execution safeguards (§6, §12).
type SafetyConfig struct {
	AllowedHosts       []string `yaml:"allowed_hosts,omitempty" json:"allowed_hosts,omitempty"`
	AllowNonIdempotent bool     `yaml:"allow_non_idempotent,omitempty" json:"allow_non_idempotent,omitempty"`
}

// ParseByteSize parses a human-readable byte string (e.g. "1MiB", "500KB", "1024") into int64 bytes.
func ParseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty byte size")
	}

	// Separate numbers from unit
	idx := 0
	for idx < len(s) && (unicode.IsDigit(rune(s[idx])) || s[idx] == '.') {
		idx++
	}
	if idx == 0 {
		return 0, fmt.Errorf("invalid byte size format: %q", s)
	}

	numStr := s[:idx]
	unit := strings.TrimSpace(s[idx:])

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil || val < 0 {
		return 0, fmt.Errorf("invalid byte size number %q: %w", numStr, err)
	}

	var multiplier float64 = 1
	switch strings.ToLower(unit) {
	case "", "b", "bytes":
		multiplier = 1
	case "k", "kb":
		multiplier = 1000
	case "kib":
		multiplier = 1024
	case "m", "mb":
		multiplier = 1000 * 1000
	case "mib":
		multiplier = 1024 * 1024
	case "g", "gb":
		multiplier = 1000 * 1000 * 1000
	case "gib":
		multiplier = 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown byte unit %q in %q", unit, s)
	}

	total := int64(val * multiplier)
	return total, nil
}
