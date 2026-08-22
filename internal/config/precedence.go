package config

import (
	"fmt"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
)

// CLIFlags holds parsed command-line flag values for execution and configuration overrides (§6).
type CLIFlags struct {
	ConfigFile        string
	URL               string
	Method            string
	Model             core.WorkloadModel
	Rate              float64
	TimeUnit          time.Duration
	Users             int64
	Duration          time.Duration
	Timeout           time.Duration
	MaxInFlight       int64
	ResponseBodyLimit string
	Redirects         string
	AllowDestructive  bool
	DryRun            bool
	NonInteractive    bool
}

// ApplyCLIOverrides applies command-line flag overrides onto a Config struct with canonical
// precedence (CLI flag > Environment variable > YAML document > Default) and re-validates the result.
func ApplyCLIOverrides(cfg *Config, flags *CLIFlags) error {
	if cfg == nil {
		return fmt.Errorf("%w: config cannot be nil", ErrConfigValidation)
	}
	if flags == nil {
		return ValidateConfig(cfg)
	}

	if flags.URL != "" {
		cfg.Request.URL = flags.URL
	}
	if flags.Method != "" {
		cfg.Request.Method = flags.Method
	}

	// Model overrides
	if flags.Model != "" {
		cfg.Load.Model = flags.Model
		if flags.Model == core.WorkloadModelOpen {
			// Clear closed-model fields to avoid mixed-model validation errors
			cfg.Load.Users = 0
			cfg.Load.ThinkTime = 0
		} else if flags.Model == core.WorkloadModelClosed {
			// Clear open-model fields
			cfg.Load.Rate = 0
			cfg.Load.TimeUnit = 0
			cfg.Load.MaxInFlight = 0
		}
	}

	if flags.Rate > 0 {
		cfg.Load.Rate = flags.Rate
	}
	if flags.TimeUnit > 0 {
		cfg.Load.TimeUnit = Duration(flags.TimeUnit)
	}
	if flags.Users > 0 {
		cfg.Load.Users = flags.Users
	}
	if flags.Duration > 0 {
		cfg.Load.Duration = Duration(flags.Duration)
	}
	if flags.Timeout > 0 {
		cfg.Request.Timeout = Duration(flags.Timeout)
	}
	if flags.MaxInFlight > 0 {
		cfg.Load.MaxInFlight = flags.MaxInFlight
	}
	if flags.ResponseBodyLimit != "" {
		cfg.Request.ResponseBodyLimit = flags.ResponseBodyLimit
	}
	if flags.Redirects != "" {
		cfg.Request.Redirects = flags.Redirects
	}
	if flags.AllowDestructive {
		cfg.Safety.AllowNonIdempotent = true
	}

	// Provide sensible defaults for pure CLI executions without config file
	if cfg.SchemaVersion == 0 {
		cfg.SchemaVersion = ExpectedSchemaVersion
	}
	if cfg.Name == "" {
		cfg.Name = "cli-execution"
	}
	if cfg.Request.Method == "" {
		cfg.Request.Method = "GET"
	}
	if cfg.Load.Model == "" {
		if cfg.Load.Users > 0 {
			cfg.Load.Model = core.WorkloadModelClosed
		} else {
			cfg.Load.Model = core.WorkloadModelOpen
			if cfg.Load.Rate <= 0 {
				cfg.Load.Rate = 1
			}
			if cfg.Load.MaxInFlight <= 0 {
				cfg.Load.MaxInFlight = 10
			}
		}
	}
	if cfg.Load.Duration.Duration() <= 0 {
		cfg.Load.Duration = Duration(10 * time.Second)
	}

	return ValidateConfig(cfg)
}
