# DAEGSA Execution Plan

Status: COMMITTED
Canonical phase: Phase 2 - Metrics and Closed-Model Baseline
Tranche: entire phase

## Objective

Implement the high-performance, bounded-memory metrics engine (`internal/metrics`), the closed-model virtual-user workload scheduler (`internal/scheduler`), the terminal and JSON v1 report generation subsystem (`internal/report`), and CLI wiring in `internal/cli` for closed-model load testing. Deliver a complete, robust, and deterministic vertical slice capable of executing $N$ concurrent Virtual User (VU) loops with think time, graceful drain, bounded latency tracking (spanning 1µs to 1h with 3 significant figures), 12-state outcome rollups, generator health monitoring, and ANSI/JSON reporting with zero goroutine leaks, zero connection leaks, and strictly bounded memory.

## Requirements traceability

| Plan Section | Requirement Description | Implementation / Test Target |
| :--- | :--- | :--- |
| **§3, §9, §15** | Bounded high-precision histogram interface and implementation spanning 1µs to 1h with 3 significant figures, fixed memory footprint, and percentile calculations (min, max, mean, p50, p90, p95, p99, p99.9) | `internal/metrics/histogram.go`, `internal/metrics/histogram_test.go` |
| **§4, §9, §15** | Lock-free worker-local metrics aggregator (`WorkerMetrics`) capturing latency samples, outcome counts (all 12 states), HTTP status distribution, bytes sent/received, bounded error samples, and rate-limit header observations | `internal/metrics/worker.go`, `internal/metrics/worker_test.go` |
| **§4, §9, §13** | Central metrics snapshot and deterministic merge engine (`Snapshot`, `Merge(workers []WorkerMetrics) *AggregatedMetrics`) computing throughput, start rate, error rates, and separate latency distributions (all completed vs expected success) | `internal/metrics/aggregate.go`, `internal/metrics/aggregate_test.go` |
| **§9, §13, §14** | Generator resource and self-diagnostic sampler (`GeneratorHealth`: peak goroutines, memory allocation, CPU usage estimate, scheduler lag, client saturation warnings) | `internal/metrics/health.go`, `internal/metrics/health_test.go` |
| **§2, §4, §7** | Closed Workload Controller running exactly $N$ concurrent Virtual User (VU) worker loops with per-VU state, independent worker metric accumulators, request-wait-think iterations, and monotonic clock integration | `internal/scheduler/closed.go`, `internal/scheduler/closed_test.go` |
| **§7, §8, §15** | Test lifecycle state machine integration: warm-up phase (excluded from threshold metrics), running for duration, graceful stop draining in-flight requests, and context cancellation abort | `internal/scheduler/closed.go`, `internal/scheduler/lifecycle_test.go` |
| **§7, §15** | Deterministic clock abstraction integration (`internal/clock`), supporting `ControllableClock` for virtual-time simulation and `RealClock` for production load runs | `internal/scheduler/closed.go`, `internal/scheduler/closed_test.go` |
| **§13, §15** | Terminal report generator: ANSI-formatted console summary with header metadata, target vs achieved RPS, throughput, 12-state outcome distribution, status codes, latency comparison tables, rate-limit observations, and generator health | `internal/report/terminal.go`, `internal/report/terminal_test.go` |
| **§13, §15** | JSON report v1 generator: serialization conforming to JSON schema v1 (`report_schema_version: 1`), UTC timestamps, duration_ms, sanitized config fingerprint, generator health, and incomplete flag | `internal/report/json.go`, `internal/report/json_test.go`, `internal/report/schema_test.go` |
| **§4, §10, §15** | CLI integration: connect closed-model scheduler and reporting into `daegsa run` when `load.model == "closed"` (or default), with `--output-json` flag and canonical exit codes | `internal/cli/run.go`, `internal/cli/flags.go`, `internal/cli/cli_test.go` |
| **§9, §15, §18** | Concurrency, zero-leak, and bounded-memory verification: exhaustive tests against `internal/testtarget` with zero goroutine leaks, zero connection leaks, and constant memory footprint over 100,000+ iterations | `internal/scheduler/closed_test.go`, `benchmarks/metrics_test.go` |

## Current repository findings

- **Phase 0 & Phase 1 Foundations Completed**:
  - `internal/core`: canonical workload models (`open`, `closed`), 12-state `Outcome` taxonomy & `OutcomeClassifier`, 5 `ExitCode` constants, timing boundaries (`Latency`, `TTFB`, `ScheduleLag`), and lifecycle states (`StateInitialized`, `StateWarmup`, `StateRunning`, `StateGracefulStop`, `StateCanceled`, `StateCompleted`).
  - `internal/config`: strict YAML parsing, environment variable resolution (`${VAR}` / `$${VAR}`), CLI flag precedence, sanitized SHA256 configuration fingerprinting, and header/URL credential redaction.
  - `internal/safety`: safety preflight engine (host allowlist, destructive method authorization, safety ceilings, DNS preflight lookup).
  - `internal/plan`: immutable execution `Plan` structure and sanitized console formatter.
  - `internal/executor`: reusable connection-pooled `http.Transport`, request builder, response body capper/drainer, rate-limit parser (`Retry-After`, `RateLimit-*`, `X-RateLimit-*`), and single-request execution engine.
  - `internal/clock`: `RealClock` (monotonic) and `ControllableClock` (priority-queue virtual monotonic time with timer/ticker support).
  - `internal/testtarget`: 8-mode deterministic HTTP simulation server (status codes, delays, payload streaming, redirects, TCP drops, timeout hangs, cookies, 429 rate limiting).
  - `internal/report`: JSON report schema v1 structs (`Report`, `RequestCounts`, `LatencySummary`, `RateLimitObservations`, `GeneratorHealth`, `ThresholdResult`) and schema serialization contract tests (`internal/report/schema_test.go`).
  - `cmd/daegsa` & `internal/cli`: Cobra CLI commands (`run`, `validate`, `version`, `help`) and canonical exit code mapping.
- **Dependencies Present**: Go 1.22, `gopkg.in/yaml.v3`, `github.com/spf13/cobra`, `github.com/spf13/pflag`, `github.com/HdrHistogram/hdrhistogram-go` (v1.1.2).
- **Components Implemented in Phase 2**:
  - `internal/metrics`: `histogram.go`, `worker.go`, `aggregate.go`, `health.go` and comprehensive unit tests.
  - `internal/scheduler`: `scheduler.go`, `closed.go` and comprehensive deterministic / integration tests.
  - `internal/report`: `terminal.go`, `json.go`, `builder.go` and formatting / golden output tests.
  - `internal/cli`: updated `run.go` and `flags.go` to execute the closed-model scheduler, collect metrics, format reports, and support `--output-json`.
  - `benchmarks`: added `metrics_bench_test.go` for histogram, worker, and merge benchmarks.

## Files expected to change

```text
daegsa/
├── go.mod                                   # Modified: Add github.com/HdrHistogram/hdrhistogram-go
├── go.sum                                   # Modified: Checksums for new dependencies
├── internal/
│   ├── cli/
│   │   ├── flags.go                         # Modified: Add --output-json (-o) flag
│   │   ├── run.go                           # Modified: Wire closed scheduler, metrics aggregator, and reporters
│   │   └── cli_test.go                      # Modified/Added: End-to-end closed-model execution and reporting tests
│   ├── metrics/
│   │   ├── histogram.go                     # New: Histogram interface and HDR/bounded implementation (1µs - 1h)
│   │   ├── histogram_test.go                # New: Unit tests for precision, percentiles, min/max/mean, and merge
│   │   ├── worker.go                        # New: Lock-free WorkerMetrics struct with 12 outcomes, status, bytes, errors
│   │   ├── worker_test.go                   # New: Unit tests for worker-local metric recording and bounds
│   │   ├── aggregate.go                     # New: Central metrics merge engine and percentile/throughput calculator
│   │   ├── aggregate_test.go                # New: Unit tests for multi-worker merging and math reconciliation
│   │   ├── health.go                        # New: Generator health sampler (CPU, memory, goroutines, scheduler lag)
│   │   └── health_test.go                   # New: Unit tests for health monitoring and saturation warnings
│   ├── scheduler/
│   │   ├── scheduler.go                     # New: Common Scheduler interface and execution options
│   │   ├── closed.go                        # New: Closed-model VU controller, worker loops, think time, lifecycle
│   │   ├── closed_test.go                   # New: Deterministic virtual-time tests and testtarget integration tests
│   │   └── leak_test.go                     # New: Goroutine leak, connection leak, and bounded-memory tests
│   └── report/
│       ├── builder.go                       # New: Report builder constructing report.Report from execution state
│       ├── terminal.go                      # New: ANSI-formatted console report generator
│       ├── terminal_test.go                 # New: Tests for ANSI layout, tables, percentages, and redaction
│       ├── json.go                          # New: JSON report generator and file persistence
│       └── json_test.go                     # New: Tests for JSON report formatting and schema compliance
└── benchmarks/
    └── metrics_bench_test.go                # New: Benchmarks for histogram recording, worker aggregation, and merge
```

## Implementation checklist

### 1. Dependencies & Histogram Abstraction (`go.mod`, `internal/metrics/histogram.go`)
- [x] Add `github.com/HdrHistogram/hdrhistogram-go` (v1.1.2) to `go.mod` and run `go mod tidy`.
- [x] Define the internal `Histogram` interface in `internal/metrics/histogram.go` (§3, §9):
  - `Record(value int64) error` (values in microseconds: 1µs to 3,600,000,000µs = 1 hour).
  - `ValueAtQuantile(q float64) int64` (quantile in [0.0, 100.0]).
  - `Min() int64`
  - `Max() int64`
  - `Mean() float64`
  - `Count() int64`
  - `Merge(other Histogram) error`
  - `Reset()`
  - `Copy() Histogram`
- [x] Implement `HDRHistogram` struct wrapping `hdrhistogram.Histogram`:
  - Configure bounded parameters: `minValue = 1` (1µs), `maxValue = 3600000000` (1 hour in µs), `sigFigs = 3`.
  - Clamp recorded values below 1µs to 1µs; clamp values exceeding 1h to 1h with overflow tracking.
  - Implement zero-allocation `Record` operations on the hot path.
  - Implement `NewLatencyHistogram() Histogram` constructor.

### 2. Worker-Local Metrics Aggregator (`internal/metrics/worker.go`)
- [x] Define `WorkerMetrics` struct in `internal/metrics/worker.go` (§4, §9):
  - `WorkerID int`
  - `Planned int64`, `Scheduled int64`, `Started int64`, `Completed int64`, `Canceled int64`, `Dropped int64`
  - `Outcomes [12]int64` (indexed by canonical `core.Outcome` enum / mapped to 12 states)
  - `StatusCodes map[int]int64` (bounded HTTP status code distribution)
  - `AllLatency Histogram` (all completed responses)
  - `SuccessLatency Histogram` (expected-status responses only)
  - `TTFBSumMicroseconds int64`, `TTFBCount int64`
  - `BytesSent int64`, `BytesReceived int64`
  - `ErrorSamples []ErrorSample` (bounded circular buffer / slice, max `MaxErrorSamples = 10`)
  - `RateLimitObservations RateLimitObservations` (count of 429s, bounded slice of max 10 `RateLimitHeaderSample`)
- [x] Implement `NewWorkerMetrics(workerID int) *WorkerMetrics`.
- [x] Implement `RecordResult(res *executor.Result)` on `*WorkerMetrics`:
  - Completely lock-free execution (isolated to single VU goroutine).
  - Record outcome count in `Outcomes`.
  - If `res.StatusCode > 0`: increment `StatusCodes[res.StatusCode]`.
  - Convert `res.Latency` to microseconds and record into `AllLatency`.
  - If `res.Outcome.IsSuccess()`: record into `SuccessLatency`.
  - Accumulate `BytesSent` and `BytesReceived`.
  - Accumulate TTFB.
  - If `res.Err != nil`: record normalized error message into bounded `ErrorSamples` (track message, error class, and occurrence count).
  - If `res.RateLimitInfo != nil`: record rate-limit header observations.
- [x] Implement `Snapshot() *WorkerMetrics` performing a safe copy for mid-test progress polling.

### 3. Central Metrics Merge & Aggregation Engine (`internal/metrics/aggregate.go`)
- [x] Define `AggregatedMetrics` struct in `internal/metrics/aggregate.go` (§4, §9, §13):
  - `RequestCounts report.RequestCounts`
  - `Outcomes map[core.Outcome]int64`
  - `StatusCodes map[string]int64`
  - `Latency report.LatencySummary` (min, max, mean, p50, p90, p95, p99, p99.9 in milliseconds)
  - `RateLimits report.RateLimitObservations`
  - `TotalBytesSent int64`, `TotalBytesReceived int64`
  - `Duration time.Duration`
  - `AchievedStartRPS float64` (started / duration seconds)
  - `CompletedThroughput float64` (completed / duration seconds)
  - `ErrorRate float64` ((completed - success) / completed)
  - `RateLimitedRate float64` (429 count / completed)
  - `ErrorSamples []ErrorSample`
- [x] Implement `MergeWorkers(workers []*WorkerMetrics, duration time.Duration) (*AggregatedMetrics, error)`:
  - Initialize empty combined histograms for all completed and expected success latencies.
  - Loop over all `WorkerMetrics` and merge histograms using `Histogram.Merge()`.
  - Sum `Planned`, `Scheduled`, `Started`, `Completed`, `Canceled`, `Dropped`.
  - Aggregate outcome counts across all 12 states.
  - Aggregate status code counts.
  - Aggregate byte counts.
  - Calculate `LatencySummary` percentiles: convert microseconds to milliseconds (`float64(µs) / 1000.0`).
  - Calculate throughput rates and error percentages.
  - Reconcile math: `Planned == Started + Dropped`, `Started == Completed + Canceled + InFlight`.

### 4. Generator Health Sampler (`internal/metrics/health.go`)
- [x] Implement `GeneratorHealthSampler` in `internal/metrics/health.go` (§9, §13, §14):
  - Track `GoroutinesPeak int64` using `runtime.NumGoroutine()`.
  - Track `CPUMaxPercent float64` via interval sampling.
  - Track `SchedulerLagMaxMS float64`.
  - Track memory allocation and GC stats via `runtime.ReadMemStats`.
  - Maintain active background sampling loop during test execution with configurable sample interval (e.g. 250ms).
- [x] Implement `Collect() report.GeneratorHealth`:
  - Evaluate saturation conditions and generate explicit warnings:
    - Peak CPU > 85%: `"client CPU saturation detected (> 85%)"`.
    - Peak goroutines > threshold: `"high goroutine count detected"`.
    - Max scheduler lag > 50ms: `"scheduler lag exceeded 50ms"`.
  - Return populated `report.GeneratorHealth`.

### 5. Closed-Model Workload Scheduler (`internal/scheduler/closed.go`)
- [x] Define `ClosedScheduler` struct in `internal/scheduler/closed.go` (§2, §4, §7):
  - `plan *plan.Plan`
  - `executor *executor.HTTPExecutor`
  - `clock clock.Clock`
  - `healthSampler *metrics.GeneratorHealthSampler`
  - `stateMachine *core.LifecycleStateMachine`
  - `workers []*metrics.WorkerMetrics`
  - `inFlightCount atomic.Int64`
- [x] Implement `NewClosedScheduler(p *plan.Plan, exec *executor.HTTPExecutor, clk clock.Clock) (*ClosedScheduler, error)`:
  - Validate `p.Model == core.WorkloadModelClosed` and `p.Users > 0`.
  - Initialize lifecycle state machine to `StateInitialized`.
  - Initialize `p.Users` individual `WorkerMetrics` instances.
- [x] Implement `Run(ctx context.Context) (*metrics.AggregatedMetrics, *report.GeneratorHealth, error)`:
  - Start `GeneratorHealthSampler`.
  - Transition state machine: `StateInitialized` -> `StateRunning` (or `StateWarmup` if configured).
  - Create run context with test duration deadline or start duration timer: `durationTimer := s.clock.NewTimer(s.plan.Duration)`.
  - Launch $N$ worker goroutines using `sync.WaitGroup`:
    - Each worker executes `runVU(ctx, workerID, workerMetrics, stopChan)`.
  - **Worker Loop Algorithm (`runVU`)**:
    1. Check if stop signaled or `ctx.Err() != nil`. If so, exit.
    2. Atomically increment `inFlightCount`.
    3. Worker records `Started++`.
    4. Call `executor.ExecuteRequest(reqCtx)`.
    5. Atomically decrement `inFlightCount`.
    6. Worker records `Completed++` and calls `workerMetrics.RecordResult(result)`.
    7. If `plan.ThinkTime > 0`:
       - Wait for think time using `thinkTimer := s.clock.NewTimer(s.plan.ThinkTime)`.
       - Select on `thinkTimer.C()`, `stopChan`, or `ctx.Done()`.
    8. Loop back to step 1.
  - **Shutdown & Drain Handling**:
    - When `durationTimer.C()` fires:
      - Transition state machine to `StateGracefulStop`.
      - Signal all workers to stop starting new iterations (close `stopChan`).
      - Start `gracefulTimer := s.clock.NewTimer(s.plan.GracefulStop)`.
      - In a separate goroutine, wait for `wg.Wait()` and close `doneChan`.
      - Select between `doneChan` (clean drain) and `gracefulTimer.C()` (graceful stop timeout expired) and `ctx.Done()` (hard interrupt).
      - If graceful timeout expires or `ctx.Done()` occurs:
        - Cancel active request contexts.
        - Transition state machine to `StateCanceled` if interrupted, or `StateCompleted`.
      - Transition state machine to `StateCompleted`.
  - Stop `GeneratorHealthSampler`.
  - Collect health metrics and merge all `WorkerMetrics` into `*metrics.AggregatedMetrics`.
  - Return aggregated metrics and generator health.

### 6. Terminal Report Formatter (`internal/report/terminal.go`)
- [x] Implement `FormatTerminalReport(rep *Report, p *plan.Plan) string` in `internal/report/terminal.go` (§13):
  - Format test header: Test Name, Target URL (redacted), Method, Model (`closed`), Users, Planned vs Actual Duration.
  - Format Request & Throughput summary table:
    - Target: $N$ Virtual Users.
    - Achieved Start Rate: `X.XX req/s`.
    - Completed Throughput: `Y.YY req/s`.
    - Total Counts: Planned, Started, Completed, Canceled, Dropped.
  - Format 12-State Outcome Distribution table:
    - Display all outcomes with count and percentage (highlight non-zero errors).
  - Format HTTP Status Code table:
    - Display status codes (e.g. 200, 404, 500) with counts and percentages.
  - Format Latency Distribution table:
    - Columns: `Metric`, `All Completed (ms)`, `Expected Success (ms)`.
    - Rows: `Min`, `p50`, `p90`, `p95`, `p99`, `Max`, `Mean`.
  - Format Rate Limiting Observations (if non-zero 429s or headers present):
    - 429 count, sample `Retry-After` values, `RateLimit-*` headers.
  - Format Generator Health & Saturation:
    - Peak Goroutines, Max CPU %, Max Scheduler Lag, Saturation Warnings.
  - Format Test Result banner: `PASS` / `FAIL` (based on outcome/incomplete status).

### 7. JSON Report Generator & File Writer (`internal/report/json.go`, `internal/report/builder.go`)
- [x] Implement `BuildReport(...) *Report` in `internal/report/builder.go`:
  - Assemble `Report` with `ReportSchemaVersion = 1`, Daegsa version/commit/build date, OS, Arch.
  - Populate sanitized configuration fingerprint (`p.Fingerprint`).
  - Set `StartTimeUTC`, `EndTimeUTC`, `DurationMS`.
  - Set `WorkloadModel = p.Model`.
  - Populate `RequestCounts`, `Outcomes`, `StatusCodes`, `Latency`, `RateLimits`, `GeneratorHealth`.
  - Set `Incomplete: true` if canceled or graceful stop timed out.
- [x] Implement `WriteJSONReport(filename string, rep *Report) error` in `internal/report/json.go`:
  - Marshal using `rep.ToJSON()`.
  - Write atomically to target file path.

### 8. CLI Integration (`internal/cli/run.go`, `internal/cli/flags.go`)
- [x] Add `--output-json` (shorthand `-o`) flag to CLI flags in `internal/cli/flags.go`.
- [x] Update `internal/cli/run.go` to execute the full closed-model load test:
  - If `--dry-run`: print plan summary and exit 0.
  - If `p.Model == core.WorkloadModelClosed` (or default):
    - Construct `executor.NewHTTPExecutor(p)`.
    - Construct `scheduler.NewClosedScheduler(p, exec, clock.NewRealClock())`.
    - Run scheduler with `cmd.Context()`.
    - Construct `Report` via `report.BuildReport`.
    - Print ANSI terminal report to stdout.
    - If `--output-json` provided: write JSON report to file.
    - Determine exit code:
      - If `rep.Incomplete`: exit code 3 (`ExitCodeRuntimeFailure`).
      - If any requests failed or unexpected status occurred (when no thresholds defined): exit code 1 (`ExitCodeThresholdFailure`).
      - If all requests succeeded: exit code 0 (`ExitCodeSuccess`).

## Test checklist

### 1. Histogram & Worker Metrics Unit Tests (`internal/metrics`)
- [x] `internal/metrics/histogram_test.go`:
  - Test recording microsecond values (1µs to 3,600,000,000µs).
  - Test precision at boundaries (1µs, 1ms, 100ms, 1s, 1m, 1h).
  - Test percentile retrieval (`ValueAtQuantile`: 50, 90, 95, 99, 99.9).
  - Test `Min()`, `Max()`, `Mean()`, `Count()`.
  - Test `Merge()` of two histograms and verify percentiles and count reconcile.
  - Test zero-sample histogram behavior (returns 0 for min/max/percentiles, no panic).
  - Test value clamping for values < 1µs and > 1h.
- [x] `internal/metrics/worker_test.go`:
  - Test recording single `executor.Result` into `WorkerMetrics`.
  - Test recording across all 12 canonical `core.Outcome` states.
  - Test HTTP status code distribution counting (200, 204, 404, 429, 500).
  - Test byte counts accumulation (`BytesSent`, `BytesReceived`).
  - Test error sample bounding: record 50 errors and verify `ErrorSamples` is capped at `MaxErrorSamples` (10) without unbounded growth.
  - Test rate-limit header observations recording.
- [x] `internal/metrics/aggregate_test.go`:
  - Test merging $N=10$ `WorkerMetrics` instances into `AggregatedMetrics`.
  - Test math reconciliation: `Total Planned == Started + Dropped`, `Total Started == Completed + Canceled`.
  - Test throughput and start rate calculations.
  - Test latency milliseconds conversion.
- [x] `internal/metrics/health_test.go`:
  - Test background sampling captures non-zero goroutine counts and memory stats.
  - Test CPU saturation warning generation when simulated CPU > 85%.
  - Test scheduler lag warning generation when lag > 50ms.

### 2. Closed-Model Scheduler Tests (`internal/scheduler`)
- [x] `internal/scheduler/closed_test.go` (Deterministic Virtual-Time Tests with `ControllableClock`):
  - Test concurrency invariant: verify exactly $N$ concurrent VU worker loops run.
  - Test think time: verify each iteration pauses for exact `think_time` duration between requests.
  - Test duration expiration: verify workers stop starting new requests exactly when `duration` expires.
  - Test graceful stop:
    - Simulate in-flight request finishing during graceful stop window -> completed successfully.
    - Simulate in-flight request hanging past `graceful_stop` window -> canceled with `OutcomeCanceled`.
  - Test hard context cancellation:
    - Cancel `ctx` mid-run -> immediate worker termination, in-flight requests canceled, `Incomplete: true`.
- [x] `internal/scheduler/closed_test.go` (Integration Tests with `internal/testtarget`):
  - **200 OK Fast Target**: 10 VUs, 500ms duration, 10ms think time -> 100% `OutcomeSuccess`.
  - **Delayed Target (Mode 2)**: 5 VUs, 50ms delay, 200ms duration -> latencies >= 50ms, throughput correctly measured.
  - **429 Rate-Limited Target (Mode 8)**: 5 VUs against rate-limited endpoint -> `OutcomeRateLimited` counted, rate limit headers extracted.
  - **500 Server Error Target (Mode 1)**: 5 VUs against error endpoint -> `OutcomeUnexpectedStatus` counted.
  - **Disconnect & Timeout Targets (Mode 5 & 6)**: 5 VUs against dropping/hanging endpoints -> transport errors / timeouts accurately categorized.
- [x] `internal/scheduler/leak_test.go` (Zero-Leak & Bounded Memory Assertions):
  - **Goroutine Leak Test**: count active goroutines before scheduler run; assert active goroutines after `scheduler.Run()` matches baseline (0 leaked goroutines).
  - **Connection Leak Test**: execute 1,000 requests across 10 VUs; verify transport idle connections are cleanly closed and socket count returns to 0.
  - **Bounded-Memory Soak Test**: run 100,000 closed-model iterations; sample heap allocations at iteration 1,000, 10,000, and 100,000; assert heap growth is bounded (< 15% variance, zero linear memory accumulation).

### 3. Reporting Tests (`internal/report`)
- [x] `internal/report/terminal_test.go`:
  - Test `FormatTerminalReport` produces clean ANSI output with all required sections.
  - Test URL credential and token redaction in terminal report header.
  - Test zero-count outcomes and status codes formatting.
  - Test latency percentile display accuracy.
  - Test saturation warning banner formatting.
- [x] `internal/report/json_test.go`:
  - Test `BuildReport` populates all fields matching `report_schema_version: 1`.
  - Test JSON serialization validity against `TestReport_Serialization` contract.
  - Test `WriteJSONReport` writes valid UTF-8 formatted JSON file to disk.
  - Test `Incomplete` flag serialization on canceled run.

### 4. CLI End-to-End Tests (`internal/cli`)
- [x] `internal/cli/cli_test.go`:
  - Test `daegsa run --url <testtarget> --users 5 --duration 200ms`: runs closed model, prints ANSI report, exits 0.
  - Test `daegsa run --url <testtarget> --users 5 --duration 200ms --output-json <path>`: generates valid JSON report file on disk.
  - Test `daegsa run --url <testtarget_500> --users 2 --duration 100ms`: exits with code 1 (`ExitCodeThresholdFailure`) due to unexpected status codes.
  - Test `daegsa run --config <closed_config.yaml>`: loads YAML configuration, runs closed model, prints report, exits 0.

## Safety and failure behavior

- **Lock-Free Hot Path**: Worker VUs must never contend on a global metrics lock during request execution or recording. All request outcome and latency recording must be worker-local, with central merging occurring only during periodic snapshots or post-test finalization.
- **Strict Memory Bounding**: Error messages and rate-limit header samples must use fixed-size circular buffers / slices (capped at 10 entries) to prevent unbounded memory growth during long soak tests. Histograms must use fixed bucket structures spanning 1µs to 1h.
- **Graceful Shutdown Bounds**: When the load test duration expires, no new request iterations may be started. In-flight requests are allowed up to `GracefulStop` (default 5s, configurable) to drain. If requests remain in flight after `GracefulStop`, their execution contexts must be forcefully canceled and classified as `OutcomeCanceled`.
- **Interrupt / Context Cancellation**: If the process receives SIGINT/SIGTERM or parent context is canceled, all running VU worker loops and active HTTP requests must immediately abort, join cleanly, and mark the report with `Incomplete: true` with exit code 3 (`ExitCodeRuntimeFailure`).
- **Generator Saturation Warnings**: If the client generator detects high CPU (> 85%), excessive GC pauses, or high scheduler lag (> 50ms), it must record explicit warnings in both terminal and JSON reports to ensure operators do not mistake client saturation for server degradation.
- **Secret Redaction Invariance**: Target URLs, credentials, authorization headers, cookies, and secret query parameters must remain unconditionally redacted in terminal reports, logs, and JSON reports.

## Acceptance gates

1. **Clean Compilation & Zero Diagnostics**: `go build ./...` and `go vet ./...` compile with 0 warnings, 0 diagnostics, and 0 errors.
2. **Deterministic & Race-Free Concurrency**: All unit, scheduler, report, and CLI tests pass cleanly without race conditions or deadlocks (`go test -v -count=1 ./...`).
3. **Repeatable Closed Workload Execution**: `ClosedScheduler` runs exactly $N$ concurrent VU loops, correctly honors `think_time`, stops new iterations on duration expiration, drains in-flight requests within `graceful_stop`, and merges worker metrics without data loss.
4. **Zero Goroutine & Connection Leaks**: Goroutine leak checks and connection pool teardown tests confirm 0 orphaned goroutines and 0 dangling TCP sockets after test completion.
5. **Bounded-Memory Soak Compliance**: 100,000-iteration closed workload test confirms memory footprint remains strictly bounded with zero linear memory accumulation.
6. **Full Report Delivery**: ANSI terminal report and JSON v1 report (`report_schema_version: 1`) are generated with complete request counts, 12-state outcome distributions, HTTP status tables, dual latency percentiles (all completed vs expected success), rate-limit observations, and generator health.
7. **CLI Closed-Model Integration**: `daegsa run` successfully runs closed-model tests from CLI flags and YAML configurations, prints terminal summaries, exports `--output-json` files, and exits with canonical exit codes.
8. **Git & Code Hygiene**: `git diff --check` passes with zero formatting or whitespace issues; no temporary artifacts or binaries committed.

## Explicit non-goals

- Implementing the open arrival-rate scheduler, Poisson arrivals, or `max_in_flight` drop semantics (deferred to Phase 3).
- Implementing threshold DSL parsing and evaluation expressions (deferred to Phase 4).
- Implementing multi-token authentication pools or credential rotation (deferred to Phase 5).
- Implementing ramp/spike/soak profile compilers or report comparison diffing (deferred to Phase 6 & Phase 7).

## Open questions

*None. The metrics architecture, HDR histogram boundaries, worker-local accumulation strategy, closed workload state machine, lifecycle transitions, terminal/JSON report formats, and exit code mappings are fully specified in `docs/DAEGSA_Implementation_Plan.md` and frozen for Phase 2.*

## Implementation handoff

### Changed files
- `go.mod` / `go.sum`: Added `github.com/HdrHistogram/hdrhistogram-go` v1.1.2 dependency.
- `internal/metrics/histogram.go`: Defined `Histogram` interface and implemented `HDRHistogram` with bounded 1µs to 1h range at 3 significant figures.
- `internal/metrics/histogram_test.go`: Unit tests for precision, percentiles, min/max/mean, clamping, merging, and zero-sample behavior.
- `internal/metrics/worker.go`: Lock-free `WorkerMetrics` struct for per-VU outcome (12 states), status code, byte, TTFB, bounded error (max 10), and rate-limit header tracking.
- `internal/metrics/worker_test.go`: Unit tests for worker-local recording across all 12 outcomes, status codes, error sample bounding, and rate-limit observations.
- `internal/metrics/aggregate.go`: Central `MergeWorkers` aggregation engine computing start rates, completed throughput, error rates, and dual latency percentiles (all completed vs expected success).
- `internal/metrics/aggregate_test.go`: Unit tests for multi-worker merging, mathematical reconciliation, and rate calculations.
- `internal/metrics/health.go`: `GeneratorHealthSampler` tracking peak goroutines, memory stats, max CPU %, and scheduler lag with saturation warning rules.
- `internal/metrics/health_test.go`: Unit tests for health sampling lifecycle and saturation warnings.
- `internal/scheduler/scheduler.go`: Common `Scheduler` execution interface.
- `internal/scheduler/closed.go`: `ClosedScheduler` running $N$ concurrent VU worker loops with think time, duration timer, graceful drain, in-flight accounting, and lifecycle state management.
- `internal/scheduler/closed_test.go`: Deterministic lifecycle, think time, concurrency invariant, cancellation, and integration tests against `internal/testtarget`.
- `internal/scheduler/leak_test.go`: Zero goroutine leak tests, zero connection leak tests, and bounded-memory soak tests.
- `internal/report/types.go`: Updated report schema types with aliases to metrics types.
- `internal/report/builder.go`: `BuildReport` constructing canonical `Report` (schema v1) from execution plan, metrics, and health diagnostics.
- `internal/report/terminal.go`: ANSI-formatted terminal summary with header metadata, credentials redaction, requests & throughput, 12-state outcomes, status codes, latency comparison tables, rate limits, and health diagnostics.
- `internal/report/terminal_test.go`: Unit tests for terminal formatting, table alignment, secret redaction, and outcome banners.
- `internal/report/json.go`: `WriteJSONReport` serializing and atomically writing formatted JSON report files.
- `internal/report/json_test.go`: Unit tests for JSON report generation and file persistence.
- `internal/cli/flags.go`: Added `--output-json` (`-o`) flag.
- `internal/cli/run.go`: Connected closed-model scheduler, metrics aggregator, and terminal/JSON reporters to `daegsa run`.
- `internal/cli/cli_test.go`: End-to-end integration tests for closed-model runs, `--output-json` export, YAML config runs, and exit code mappings.
- `benchmarks/metrics_bench_test.go`: Allocation and throughput benchmarks for histogram recording, quantile evaluation, worker-local recording, and central worker merging.

### Behavior implemented
- Full Phase 2 vertical slice: closed-model virtual-user workload generation, lock-free metrics recording, bounded HDR histogram tracking (1µs to 1h with 3 sig figs), 12-state outcome rollups, generator health diagnostics, and ANSI/JSON v1 report output.
- All requests adhere strictly to bounded memory limits, zero goroutine leaks, and zero connection leaks.
- Credential redaction is preserved across all terminal and JSON report outputs.

### Commands run and results
- `go test -v -count=1 .\...`: All tests in all packages passed (PASS).
- `go vet .\...`: 0 warnings, 0 diagnostics, 0 errors.
- `go build .\...`: Clean compilation across all packages.
- `git diff --check`: Clean formatting and whitespace.

### Known limitations
- Open arrival-rate model scheduling and `max_in_flight` drop semantics are non-goals for Phase 2 and deferred to Phase 3.
- Threshold DSL evaluation expressions are deferred to Phase 4.

### Remaining unchecked test or acceptance items
- None. All acceptance gates and verification items in `## Test checklist` and `## Acceptance gates` have been independently validated and verified.
