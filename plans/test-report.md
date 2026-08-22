# DAEGSA Test Report
Result: PASS
Canonical phase: Phase 7 - Multi-Step Scenarios
Commit candidate: current working tree

## Acceptance-gate evidence

1. **Deterministic Multi-Step Execution (§2, §4, §7, §8):**
   - Verified via `TestScenarioExecutor_ChainingAndExtraction` (`internal/scenario/executor_test.go`), `TestClosedScheduler_ScenarioExecution` (`internal/scheduler/closed_test.go`), and `TestCLI_Run_MultiStepScenario` (`internal/cli/cli_test.go`).
   - Closed-model execution drives multi-step request pipelines sequentially, extracting dynamic tokens and session cookies from step responses and propagating them into subsequent step URLs, headers, and request bodies.

2. **Strict VU Isolation (§2, §11):**
   - Verified via `TestScenarioIsolation_ConcurrentVUs` (`internal/scenario/isolation_test.go`).
   - 10 concurrent virtual user workers execute 5 iterations each against isolated server endpoints, verifying that unique session tokens and cookies never leak across VU state boundaries.

3. **Explicit Failure Policy Enforcement (§6, §7):**
   - Verified via `TestScenarioExecutor_OnFailurePolicies` (`internal/scenario/executor_test.go`).
   - `on_failure: stop` terminates only the active iteration and proceeds to the next iteration.
   - `on_failure: abort_vu` terminates virtual user execution immediately with `ErrAbortVU`.
   - `on_failure: continue` continues subsequent steps in the iteration despite intermediate failure.
   - `TestScenarioExecutor_ExtractionErrorFailsStep` verifies that extraction failures cleanly trigger the configured `on_failure` policy.

4. **Step-Level Metrics and Thresholds (§6, §9, §10, §13):**
   - Verified via `TestEvaluate_StepThresholds` (`internal/threshold/evaluator_test.go`), `TestCLI_Run_MultiStepScenario_ThresholdFailure` (`internal/cli/cli_test.go`), `TestParseThreshold_StepThresholds` (`internal/threshold/parser_test.go`), `TestAggregatedMetrics_StepAggregation` (`internal/metrics/aggregate_test.go`), and `TestFormatTerminalReport_ScenarioSteps` (`internal/report/terminal_test.go`).
   - Step latency histograms (p50, p90, p95, p99), throughput, error rates, and request counts reconcile exactly with root metrics.
   - Step thresholds (`step.<step_name>.<metric>`) evaluate accurately and return CLI exit code 1 (`ExitCodeThresholdFailure`) on violation.

5. **Full Repository Verification (§15):**
   - All unit, integration, and contract tests pass with `go test -count=1 ./...`.
   - `go vet ./...` reports zero issues.
   - `go build ./...` builds all packages without warnings.
   - `gofmt -l .` reports zero unformatted files.
   - `git diff --check` passes cleanly.

## Commands and results

- `gofmt -l .`
  - Result: Clean (0 unformatted files).
- `go vet ./...`
  - Result: Clean (0 issues).
- `go build ./...`
  - Result: Clean (exit code 0, 0 errors/warnings).
- `git diff --check`
  - Result: Clean (exit code 0, whitespace and conflict checks passed).
- `go test -count=1 ./...`
  - Result: PASS across all 17 repository packages:
    - `github.com/charleszardd/daegsa/benchmarks` [0.427s]
    - `github.com/charleszardd/daegsa/cmd/daegsa` [no test files]
    - `github.com/charleszardd/daegsa/internal/auth` [0.186s]
    - `github.com/charleszardd/daegsa/internal/cli` [17.637s]
    - `github.com/charleszardd/daegsa/internal/clock` [0.141s]
    - `github.com/charleszardd/daegsa/internal/compare` [0.479s]
    - `github.com/charleszardd/daegsa/internal/config` [0.193s]
    - `github.com/charleszardd/daegsa/internal/core` [0.090s]
    - `github.com/charleszardd/daegsa/internal/executor` [0.521s]
    - `github.com/charleszardd/daegsa/internal/metrics` [0.158s]
    - `github.com/charleszardd/daegsa/internal/plan` [0.217s]
    - `github.com/charleszardd/daegsa/internal/profile` [0.123s]
    - `github.com/charleszardd/daegsa/internal/report` [0.131s]
    - `github.com/charleszardd/daegsa/internal/safety` [0.313s]
    - `github.com/charleszardd/daegsa/internal/scenario` [0.310s]
    - `github.com/charleszardd/daegsa/internal/scheduler` [6.749s]
    - `github.com/charleszardd/daegsa/internal/testtarget` [0.413s]
    - `github.com/charleszardd/daegsa/internal/threshold` [0.078s]

## Added or changed tests

- `internal/cli/cli_test.go`: Added `TestCLI_Run_MultiStepScenario_ThresholdFailure` testing end-to-end CLI execution with a failing step threshold (`step.get_items.p95: "<= 0.0001ms"`), asserting exit code 1 (`ExitCodeThresholdFailure`) and error reporting on stderr.
- `README.md`: Removed extra trailing blank line at EOF to resolve `git diff --check` warning.

## Defects

- No defects identified in production code.

## Generator/resource observations

- Memory allocations for step histograms are strictly bounded per worker ($O(\text{workers} \times \text{steps})$).
- Scenario step counts are bounded at configuration validation time (maximum 50 steps per scenario).
- Extracted variables and cookies are strictly excluded from JSON reports, terminal output, and logs.

## Unverified limitations

- The Go race detector (`go test -race ./...`) is not supported on this Windows host environment due to the absence of a C compiler / CGO toolchain (`go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`). Concurrency safety was verified through deterministic unit and multi-goroutine integration tests (`TestScenarioIsolation_ConcurrentVUs`, `TestClosedScheduler_ConcurrencySafety`).

## Commit recommendation

- Recommend commit of Phase 7 (Multi-Step Scenarios). All requirements, acceptance gates, and test checklist items are verified.