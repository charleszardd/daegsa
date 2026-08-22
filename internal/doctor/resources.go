package doctor

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

// CheckSystemResources inspects host CPU cores, GOMAXPROCS, and memory allocations (§14).
func CheckSystemResources(ctx context.Context) CheckResult {
	start := time.Now()

	numCPU := runtime.NumCPU()
	procs := runtime.GOMAXPROCS(0)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	elapsed := time.Since(start)

	allocMB := float64(m.Alloc) / (1024 * 1024)
	sysMB := float64(m.Sys) / (1024 * 1024)

	detail := fmt.Sprintf("Logical CPUs: %d, GOMAXPROCS: %d, Memory in-use: %.1f MB, System memory: %.1f MB, GC cycles: %d",
		numCPU, procs, allocMB, sysMB, m.NumGC)

	if numCPU < 2 {
		return CheckResult{
			Name:       "System Resources (CPU & Memory)",
			Category:   CategoryResources,
			Status:     StatusWarn,
			Summary:    fmt.Sprintf("Single-core host detected (CPUs: %d)", numCPU),
			Detail:     detail,
			Suggestion: "DAEGSA high-rate open arrival testing (>1000 RPS) benefits from 2+ CPU cores to ensure independent arrival-rate timer pacing and worker execution.",
			Duration:   elapsed,
		}
	}

	return CheckResult{
		Name:     "System Resources (CPU & Memory)",
		Category: CategoryResources,
		Status:   StatusPass,
		Summary:  fmt.Sprintf("%d logical CPUs (GOMAXPROCS: %d), %.1f MB in-use memory", numCPU, procs, allocMB),
		Detail:   detail,
		Duration: elapsed,
	}
}
