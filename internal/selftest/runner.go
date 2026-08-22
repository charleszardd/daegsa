package selftest

import (
	"context"
	"time"

	"github.com/charleszardd/daegsa/internal/testtarget"
)

const (
	defaultSelfTestTimeout = 30 * time.Second
)

// RunSelfTests executes the complete in-process self-test suite against an embedded testtarget (§14, §15).
func RunSelfTests(ctx context.Context, opts Options, onProgress func(result SubTestResult)) *SelfTestReport {
	start := time.Now()

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultSelfTestTimeout
	}

	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Spin up embedded loopback target server
	srv := testtarget.NewServer()
	defer srv.Close()

	tests := make([]SubTestResult, 0, 5)
	allPassed := true

	runSubTest := func(fn func(context.Context, *testtarget.TargetServer) SubTestResult) {
		res := fn(testCtx, srv)
		if res.Status != StatusPass {
			allPassed = false
		}
		tests = append(tests, res)
		if onProgress != nil {
			onProgress(res)
		}
	}

	// 1. Closed Workload Loop
	runSubTest(runClosedWorkloadSubTest)

	// 2. Open Arrival-Rate Pacing & Max-In-Flight
	runSubTest(runOpenArrivalRateSubTest)

	// 3. Multi-Step Scenario & State Chaining
	runSubTest(runMultiStepScenarioSubTest)

	// 4. Threshold Evaluation Engine
	runSubTest(runThresholdEvaluationSubTest)

	// 5. Report Serialization & Schema
	runSubTest(runReportGenerationSubTest)

	return &SelfTestReport{
		Timestamp:     start.UTC(),
		Passed:        allPassed,
		Tests:         tests,
		TotalDuration: time.Since(start),
	}
}
