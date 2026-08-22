package cli

import (
	"time"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/spf13/pflag"
)

type flagValues struct {
	configFile        string
	url               string
	method            string
	model             string
	rate              float64
	timeUnit          time.Duration
	users             int64
	duration          time.Duration
	timeout           time.Duration
	maxInFlight       int64
	responseBodyLimit string
	redirects         string
	allowDestructive  bool
	dryRun            bool
	nonInteractive    bool
}

func addCommonFlags(fs *pflag.FlagSet, f *flagValues) {
	fs.StringVarP(&f.configFile, "config", "c", "", "Path to YAML test configuration file")
	fs.StringVarP(&f.url, "url", "u", "", "Target URL")
	fs.StringVarP(&f.method, "method", "m", "", "HTTP method (GET, POST, PUT, DELETE, PATCH, etc.)")
	fs.StringVar(&f.model, "model", "", "Workload model ('open' or 'closed')")
	fs.Float64Var(&f.rate, "rate", 0, "Target request arrival rate (open model)")
	fs.DurationVar(&f.timeUnit, "time-unit", 0, "Time unit for arrival rate (e.g. 1s, 1m)")
	fs.Int64Var(&f.users, "users", 0, "Number of concurrent virtual users (closed model)")
	fs.DurationVarP(&f.duration, "duration", "d", 0, "Test execution duration (e.g. 30s, 5m)")
	fs.DurationVarP(&f.timeout, "timeout", "t", 0, "Per-request timeout (e.g. 5s, 500ms)")
	fs.Int64Var(&f.maxInFlight, "max-in-flight", 0, "Maximum concurrent in-flight requests (open model)")
	fs.StringVar(&f.responseBodyLimit, "response-body-limit", "", "Response body read limit (e.g. 1MiB, 500KB)")
	fs.StringVar(&f.redirects, "redirects", "", "Redirect policy ('same-origin', 'none', 'all')")
	fs.BoolVar(&f.dryRun, "dry-run", false, "Print sanitized execution plan without sending test traffic")
	fs.BoolVar(&f.nonInteractive, "non-interactive", false, "Disable interactive prompts (CI mode)")
	fs.BoolVar(&f.allowDestructive, "allow-destructive", false, "Authorize non-idempotent HTTP methods (POST, PUT, PATCH, DELETE)")
}

func (f *flagValues) toCLIFlags() *config.CLIFlags {
	return &config.CLIFlags{
		ConfigFile:        f.configFile,
		URL:               f.url,
		Method:            f.method,
		Model:             core.WorkloadModel(f.model),
		Rate:              f.rate,
		TimeUnit:          f.timeUnit,
		Users:             f.users,
		Duration:          f.duration,
		Timeout:           f.timeout,
		MaxInFlight:       f.maxInFlight,
		ResponseBodyLimit: f.responseBodyLimit,
		Redirects:         f.redirects,
		AllowDestructive:  f.allowDestructive,
		DryRun:            f.dryRun,
		NonInteractive:    f.nonInteractive,
	}
}
