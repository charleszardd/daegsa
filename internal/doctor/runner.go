package doctor

import (
	"context"
	"runtime"
	"time"
)

// RunDiagnostics executes the full suite of doctor diagnostic checks (§14, §15).
func RunDiagnostics(ctx context.Context, opts Options) *DiagnosticReport {
	start := time.Now()

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	checks := make([]CheckResult, 0, 5)

	// Execute checks sequentially for predictable, isolated diagnostic measurements
	checks = append(checks, CheckClockPrecision(ctx))
	checks = append(checks, CheckDNSResolution(ctx))
	checks = append(checks, CheckTLSConfiguration(ctx))
	checks = append(checks, CheckSocketLimits(ctx))
	checks = append(checks, CheckSystemResources(ctx))

	// Aggregate overall status
	overall := StatusPass
	for _, c := range checks {
		if c.Status == StatusFail {
			overall = StatusFail
			break
		}
		if c.Status == StatusWarn && overall != StatusFail {
			overall = StatusWarn
		}
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	sysInfo := SystemDiagnostics{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		NumCPU:      runtime.NumCPU(),
		GOMAXPROCS:  runtime.GOMAXPROCS(0),
		GoVersion:   runtime.Version(),
		MemoryAlloc: m.Alloc,
		MemorySys:   m.Sys,
	}

	return &DiagnosticReport{
		Timestamp:     start.UTC(),
		OverallStatus: overall,
		Checks:        checks,
		System:        sysInfo,
		TotalDuration: time.Since(start),
	}
}
