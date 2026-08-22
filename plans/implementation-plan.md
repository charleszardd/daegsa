# DAEGSA Execution Plan

Status: COMMITTED
Canonical phase: Phase 3 - Open Arrival-Rate Engine
Tranche: entire phase

## Objective

Implement the open arrival-rate workload generator and precision scheduler (`internal/scheduler/open.go`), strictly enforcing constant fractional interval spacing across the configured `time_unit`, atomic `max_in_flight` concurrency tracking and dropped-request semantics (`core.OutcomeDropped`), scheduler lag tracking with anti-catch-up burst progression, bounded worker execution and clean joining, separate start rate vs completed throughput measurements, generator health warnings for saturation/degradation, terminal and JSON report integration, CLI command wiring (`daegsa run` for open model), and rigorous deterministic virtual-time and real-clock tolerance testing.

## Requirements traceability

| Plan Section | Requirement Description | Implementation / Test Target |
| :--- | :--- | :--- |
| **§2, §6, §7** | Open arrival-rate model scheduling: constant arrival rate with fractional interval spacing across configured `time_unit` (`interval = time_unit / rate`) using monotonic clock abstraction (`internal/clock`) | `internal/scheduler/open.go`, `internal/scheduler/open_test.go` |
| **§2, §7, §9** | Strict `max_in_flight` limit enforcement: atomic concurrency tracking; when `in_flight >= max_in_flight`, immediately classify arrival as `core.OutcomeDropped`, record dropped metrics, and skip request dispatch without queuing | `internal/scheduler/open.go`, `internal/scheduler/open_test.go` |
| **§7, §9, §14** | Scheduler lag tracking: measure difference between intended dispatch time and actual dispatch time (`ScheduleLag = ScheduledAt - PlannedAt`); record max lag; if generator falls behind (`lag > interval`), advance schedule baseline to prevent catch-up bursts | `internal/scheduler/open.go`, `internal/metrics/health.go`, `internal/scheduler/open_test.go` |
| **§4, §7, §8, §9** | Bounded worker execution: worker pool or bounded worker goroutines with parent context cancellation, graceful stop draining within `graceful_stop` window, timeout aborts, and clean joining with zero goroutine leaks | `internal/scheduler/open.go`, `internal/scheduler/leak_test.go` |
| **§2, §4, §9, §13** | Separate accounting of Planned, Scheduled, Started, Completed, Canceled, and Dropped requests; target RPS, achieved start rate (`Started / DurationSec`), completed throughput (`Completed / DurationSec`), and dual latency distributions | `internal/scheduler/open.go`, `internal/metrics/aggregate.go`, `internal/scheduler/open_test.go` |
| **§9, §13, §14** | Generator health and self-diagnostics: track peak goroutines, max CPU %, max scheduler lag, and generate explicit warnings when scheduler lag > 50ms or `max_in_flight` drops occur due to target degradation | `internal/metrics/health.go`, `internal/scheduler/open.go`, `internal/scheduler/open_test.go` |
| **§13, §15** | Terminal and JSON v1 report delivery: display Target RPS, Achieved Start Rate, Completed Throughput, Max In-Flight, Dropped Requests, and Scheduler Lag in ANSI terminal report and schema v1 JSON output | `internal/report/terminal.go`, `internal/report/json.go`, `internal/report/terminal_test.go` |
| **§4, §10, §15** | CLI integration: wire `daegsa run` when `load.model == "open"` to instantiate and execute `OpenScheduler` with CLI flags, YAML configuration, and canonical exit codes | `internal/cli/run.go`, `internal/cli/cli_test.go` |
| **§9, §15, §18** | Deterministic and real-clock verification: `ControllableClock` virtual-time tests (exact interval spacing, exact drop count at saturation, anti-burst verification, graceful drain) and real-clock integration tests against `internal/testtarget` (100 RPS, 500 RPS, slow target overload) | `internal/scheduler/open_test.go`, `internal/scheduler/leak_test.go`, `benchmarks/scheduler_bench_test.go` |

## Current repository findings

- **Phase 0, Phase 1, and Phase 2 Foundations in Place**:
  - `internal/core`: canonical workload models (`WorkloadModelOpen`, `WorkloadModelClosed`), 12-state `Outcome` taxonomy including `OutcomeDropped`, timing boundaries (`Latency`, `TTFB`, `ScheduleLag`), `OpenModelParams` (`Rate`, `TimeUnit`, `MaxInFlight`, `Interval()`), `LifecycleStateMachine`, and exit codes (`ExitCodeSuccess`, `ExitCodeThresholdFailure`, `ExitCodeValidation`, `ExitCodeRuntimeFailure`, `ExitCodeSafetyRefusal`).
  - `internal/config`: strict YAML parser, environment variable expansion, CLI flag precedence, sanitized SHA256 configuration fingerprinting, and credential redaction.
  - `internal/safety`: preflight host allowlist checks, destructive method verification, and safety ceiling enforcement (`MaxAllowedInFlight = 100000`).
  - `internal/plan`: immutable execution `Plan` structure holding `Rate`, `TimeUnit`, `MaxInFlight`, `Duration`, `GracefulStop`, `Model`, and resolved target details.
  - `internal/executor`: reusable connection-pooled `http.Transport`, request builder, response body capper/drainer, rate-limit header parser, and single-request execution engine.
  - `internal/clock`: monotonic `RealClock` and virtual-time `ControllableClock` supporting deterministic timer and ticker control.
  - `internal/metrics`: bounded HDR histogram (1µs to 1h with 3 sig figs), lock-free `WorkerMetrics` with 12 outcome accumulators, central `MergeWorkers` computing throughput and mathematical reconciliation (`Planned == Started + Dropped`, `Started == Completed + Canceled`), and `GeneratorHealthSampler` tracking CPU %, peak goroutines, and scheduler lag.
  - `internal/scheduler`: `Scheduler` interface (`Run(ctx context.Context) (*metrics.AggregatedMetrics, *metrics.GeneratorHealth, error)`) and `ClosedScheduler` implemented for closed workload model.
  - `internal/report`: ANSI terminal report formatter (`FormatTerminalReport`), schema v1 JSON serializer (`WriteJSONReport`), and report builder (`BuildReport`).
  - `internal/cli`: Cobra commands (`run`, `validate`, `version`, `help`), flags (`--model`, `--rate`, `--time-unit`, `--max-in-flight`, `--output-json`), and exit code mappings.
  - `internal/testtarget`: 8-mode deterministic HTTP test target server (normal 200 OK, delay simulation, payload streaming, redirects, TCP drop, hanging timeout, cookies, 429 rate limiting).
- **Gaps to Address for Phase 3**:
  - `internal/scheduler/open.go` is not yet implemented. `internal/cli/run.go` currently executes a single fallback request when `p.Model == "open"`.
  - `OpenScheduler` must be created with constant arrival pacing, monotonic scheduling, atomic `max_in_flight` drop handling, scheduler lag tracking, anti-catch-up burst logic, and bounded worker pools.
  - `internal/cli/run.go` must be updated to instantiate and run `OpenScheduler` when `p.Model == core.WorkloadModelOpen`.
  - Terminal reporting in `internal/report/terminal.go` should clearly display Target RPS, Max In-Flight, and Scheduler Lag for open-model runs.
  - Deterministic virtual-time unit tests, real-clock integration tests, overload degradation tests, goroutine/connection leak checks, and performance benchmarks for open arrival rates must be written.

## Files expected to change

```text
daegsa/
├── internal/
│   ├── scheduler/
│   │   ├── open.go                          # New: Open-model arrival-rate scheduler with fractional pacing and max_in_flight drops
│   │   ├── open_test.go                     # New: Deterministic virtual-time and real-clock integration tests for open model
│   │   └── leak_test.go                     # Modified: Add goroutine leak, connection leak, and soak tests for OpenScheduler
│   ├── cli/
│   │   ├── run.go                           # Modified: Wire OpenScheduler into `daegsa run` when load.model == "open"
│   │   └── cli_test.go                      # Modified: Add end-to-end open-model execution and reporting tests
│   └── report/
│       ├── terminal.go                      # Modified: Enhance open-model summary display (Target RPS, Max In-Flight, Lag)
│       └── terminal_test.go                 # Modified: Verify open-model ANSI terminal report formatting
└── benchmarks/
    └── scheduler_bench_test.go              # New: Benchmarks for open arrival-rate scheduling and dispatch hot paths
```

## Implementation checklist

### 1. Open-Model Workload Scheduler Architecture (`internal/scheduler/open.go`)
- [x] Define `OpenScheduler` struct in `internal/scheduler/open.go` (§2, §4, §7):
  - `plan *plan.Plan`: immutable validated execution plan.
  - `executor *executor.HTTPExecutor`: HTTP request executor.
  - `clock clock.Clock`: monotonic time provider (`ControllableClock` or `RealClock`).
  - `healthSampler *metrics.GeneratorHealthSampler`: background diagnostics collector.
  - `stateMachine *core.LifecycleStateMachine`: lifecycle state tracker.
  - `inFlightCount atomic.Int64`: current number of requests actively executing.
  - `peakInFlight atomic.Int64`: peak concurrent in-flight requests observed.
  - `dispatcherMetrics *metrics.WorkerMetrics`: accumulator for planned, scheduled, and dropped requests.
  - `workerPool []*metrics.WorkerMetrics`: slice of worker-local metric accumulators for active execution lanes.
- [x] Implement `NewOpenScheduler(p *plan.Plan, exec *executor.HTTPExecutor, clk clock.Clock) (*OpenScheduler, error)`:
  - Validate `p != nil` (return `ErrInvalidPlan`).
  - Validate `exec != nil` (return `ErrInvalidExecutor`).
  - Validate `p.Model == core.WorkloadModelOpen` (return `ErrIncompatibleModel`).
  - Validate `p.Rate > 0`, `p.TimeUnit > 0`, `p.MaxInFlight > 0`, and `p.Duration > 0`.
  - Default `clk` to `clock.NewRealClock()` if nil; inject `clk` into `exec.SetClock(clk)`.
  - Allocate `dispatcherMetrics = metrics.NewWorkerMetrics(-1)` for scheduler-level accounting.
  - Determine worker pool size $W = \min(p.\text{MaxInFlight}, 1024)$ (or $p.\text{MaxInFlight}$) and allocate $W$ individual `WorkerMetrics` instances.
  - Initialize lifecycle state machine to `StateInitialized`.
- [x] Implement public inspection methods on `*OpenScheduler`:
  - `InFlight() int64`: returns `inFlightCount.Load()`.
  - `PeakInFlight() int64`: returns `peakInFlight.Load()`.
  - `LifecycleState() core.LifecycleState`: returns `stateMachine.Current()`.
- [x] Implement `Run(ctx context.Context) (*metrics.AggregatedMetrics, *metrics.GeneratorHealth, error)` on `*OpenScheduler` (§7):
  - Transition state machine from `StateInitialized` to `StateRunning`.
  - Start `GeneratorHealthSampler`.
  - Create run context with test duration deadline and cancellation support (`workerCtx, cancelWorkers := context.WithCancel(ctx)`).
  - Calculate inter-arrival interval: `interval := time.Duration(float64(s.plan.TimeUnit) / s.plan.Rate)`.
  - Initialize dispatch job channel `dispatchChan := make(chan *dispatchJob, workerPoolSize)`.
  - Launch $W$ worker goroutines in worker pool with `sync.WaitGroup`:
    - Each worker continuously reads `job <- dispatchChan`, executes `res, err := s.executor.ExecuteRequest(reqCtx)`, records results into worker-local `WorkerMetrics`, decrements `inFlightCount.Add(-1)`, and signals job completion.
  - **Arrival Scheduling Loop**:
    - Record `startTime := s.clock.Now()`.
    - Track `nextTargetTick := startTime`.
    - Loop while elapsed time < `s.plan.Duration` and context is not canceled:
      - Calculate wait duration: `waitDur := nextTargetTick.Sub(s.clock.Now())`.
      - If `waitDur > 0`: sleep/wait using `timer := s.clock.NewTimer(waitDur)` selecting on `timer.C()`, `durationTimer.C()`, and `ctx.Done()`.
      - When tick triggers:
        - Record intended tick time `intendedTime := nextTargetTick`.
        - Capture actual trigger time `actualTime := s.clock.Now()`.
        - Calculate scheduler lag: `lag := actualTime.Sub(intendedTime)`.
        - If `lag > 0`: record to `s.healthSampler.RecordSchedulerLag(lag)`.
        - **Anti-Catch-Up Burst Progression**: If `lag > interval` (generator fell behind), advance `nextTargetTick = actualTime.Add(interval)` rather than firing historical backlogged ticks in an instant burst. Otherwise, advance `nextTargetTick = nextTargetTick.Add(interval)`.
        - Increment `s.dispatcherMetrics.Planned++`.
        - **Strict `max_in_flight` Enforcement**:
          - Check current in-flight: `currentInFlight := s.inFlightCount.Load()`.
          - If `currentInFlight >= s.plan.MaxInFlight`:
            - Increment `s.dispatcherMetrics.Dropped++`.
            - Increment `s.dispatcherMetrics.Outcomes[core.OutcomeDropped]++`.
            - Record generator health warning: `"max_in_flight reached, dropped requests"`.
            - Continue to next schedule interval without dispatching request.
          - If `currentInFlight < s.plan.MaxInFlight`:
            - Increment `s.dispatcherMetrics.Scheduled++`.
            - Atomically increment `s.inFlightCount.Add(1)`.
            - Update `peakInFlight` if new value > peak.
            - Send `dispatchJob` containing `intendedTime`, `actualTime`, and request context to `dispatchChan`.
  - **Graceful Shutdown & Drain Strategy**:
    - When planned `Duration` expires or context is canceled:
      - Close `dispatchChan` to prevent any further dispatches.
      - If canceled: transition state machine to `StateCanceled`; otherwise transition to `StateGracefulStop`.
      - Start graceful stop timer: `graceTimer := s.clock.NewTimer(s.plan.GracefulStop)`.
      - In background goroutine, wait for `wg.Wait()` and close `workersDone` channel.
      - Select on `workersDone` (clean drain), `graceTimer.C()` (graceful stop timeout), and `ctx.Done()` (hard cancellation).
      - If graceful timeout expires or context canceled:
        - Invoke `cancelWorkers()` to abort remaining in-flight requests.
        - Wait for workers to cleanly join.
      - Transition state machine to `StateCompleted` (or `StateCanceled`).
  - **Metrics Finalization & Return**:
    - Stop `GeneratorHealthSampler`.
    - Collect all worker metrics: combine `s.dispatcherMetrics` and all `WorkerMetrics` from the worker pool.
    - Compute elapsed duration `elapsed := s.clock.Since(startTime)`.
    - Call `metrics.MergeWorkers(allWorkers, elapsed)` to produce `*metrics.AggregatedMetrics`.
    - Verify reconciliation invariants: `reqCounts.Planned == reqCounts.Started + reqCounts.Dropped`, `reqCounts.Started == reqCounts.Completed + reqCounts.Canceled`.
    - Collect `health := s.healthSampler.Collect()`.
    - If `s.dispatcherMetrics.Dropped > 0`: ensure health diagnostics include warning `"target degradation or low max_in_flight caused dropped requests"`.
    - Return aggregated metrics and generator health.

### 2. Metrics & Health Diagnostics Integration (`internal/metrics`, `internal/scheduler/open.go`)
- [x] Ensure `WorkerMetrics` and `MergeWorkers` correctly handle `core.OutcomeDropped` without skewing latency histograms (§9):
  - Verify that dropped requests increment `Dropped` and `Outcomes[core.OutcomeDropped]`, but do NOT record entries into `AllLatency` or `SuccessLatency` histograms.
  - Verify that `AchievedStartRPS = Started / DurationSec` and `CompletedThroughput = Completed / DurationSec` correctly separate start rate from finished throughput.
- [x] Ensure `GeneratorHealthSampler` warnings capture open-model saturation and target degradation (§14):
  - Scheduler lag > 50ms triggers `"scheduler lag exceeded 50ms"`.
  - CPU > 85% triggers `"client CPU saturation detected (> 85%)"`.
  - In-flight drops trigger `"max_in_flight reached, dropped requests"`.

### 3. Reporting and CLI Integration (`internal/report`, `internal/cli`)
- [x] Update `internal/report/terminal.go` to cleanly format open-model summaries (§13):
  - When `p.Model == core.WorkloadModelOpen`:
    - Display Target Rate: `Target Rate: %.2f req/s (Rate: %.1f/%s, Max In-Flight: %d)`.
    - Display Requests & Throughput table with Planned, Scheduled, Started, Completed, Canceled, Dropped.
    - Display Achieved Start Rate vs Completed Throughput rate.
    - Display Generator Health with Peak Goroutines, Max CPU %, Max Scheduler Lag, and Saturation Warnings.
- [x] Update `internal/cli/run.go` to wire `OpenScheduler` (§4, §10):
  - When `p.Model == core.WorkloadModelOpen`:
    - Construct `scheduler.NewOpenScheduler(p, exec, clock.NewRealClock())`.
    - Execute `sched.Run(cmd.Context())`.
    - Pass aggregated metrics and health diagnostics to `report.BuildReport`.
    - Print terminal report and write JSON report if `--output-json` specified.
    - Set canonical exit codes:
      - Exit code 0 (`ExitCodeSuccess`): all completed requests succeeded.
      - Exit code 1 (`ExitCodeThresholdFailure`): unexpected status codes or request failures detected.
      - Exit code 3 (`ExitCodeRuntimeFailure`): run canceled, timed out, or incomplete.

### 4. Benchmark Suite (`benchmarks/scheduler_bench_test.go`)
- [x] Create `benchmarks/scheduler_bench_test.go` to benchmark open scheduler scheduling throughput:
  - Benchmark arrival scheduling loop at 1,000 RPS, 10,000 RPS, and 50,000 RPS using a fast mock executor.
  - Assert zero memory allocations on the hot scheduling loop path.

## Test checklist

### 1. Deterministic Virtual-Time Unit Tests (`internal/scheduler/open_test.go`)
- [x] **Exact Interval Spacing Test**:
  - Configure open model: `rate: 10`, `time_unit: 1s` (`interval = 100ms`), `duration: 1s`, `max_in_flight: 50`.
  - Use `ControllableClock`. Advance clock in 100ms steps.
  - Assert exactly 10 requests planned, 10 scheduled, 10 started, 10 completed.
  - Verify dispatch timestamps occur at exact 100ms intervals ($T_0, T_0+100\text{ms}, \dots$).
- [x] **Fractional Spacing Test**:
  - Configure `rate: 3`, `time_unit: 1s` (`interval = 333.333ms`), `duration: 1s`.
  - Verify pacing advances correctly with fractional intervals without drift accumulation.
- [x] **Strict `max_in_flight` Saturation & Drop Test**:
  - Configure `rate: 100`, `time_unit: 1s` (`interval = 10ms`), `max_in_flight: 5`, `duration: 100ms` (10 total ticks).
  - Mock executor that holds requests in flight (does not complete until released).
  - Advance clock across 10 ticks.
  - Assert first 5 requests start (`in_flight = 5`), remaining 5 requests are immediately dropped (`OutcomeDropped`).
  - Assert: `Planned == 10`, `Started == 5`, `Dropped == 5`, `InFlight == 5`.
  - Assert `Outcomes[OutcomeDropped] == 5`.
  - Assert latency histograms have count == 0 (no completed requests yet).
- [x] **Anti-Catch-Up Burst Progression Test**:
  - Configure `rate: 10`, `time_unit: 1s` (`interval = 100ms`), `duration: 2s`.
  - Advance clock by 500ms in a single jump (simulating generator pause / freeze).
  - Verify scheduler advances `nextTargetTick` to current time without firing 5 instantaneous requests in a stampede.
  - Verify scheduler lag is recorded and captured in `GeneratorHealth`.
- [x] **Scheduler Lag Tracking & Warning Test**:
  - Simulate scheduling delay of 60ms.
  - Verify `healthSampler.Collect().SchedulerLagMaxMS >= 60.0`.
  - Verify warning `"scheduler lag exceeded 50ms"` is present.
- [x] **Graceful Drain Test**:
  - Schedule 10 requests; duration expires while 3 requests remain in flight.
  - Simulate in-flight requests completing within `GracefulStop` (1s).
  - Assert all 10 requests complete with `OutcomeSuccess`.
  - Assert `stateMachine.Current() == StateCompleted`.
- [x] **Graceful Stop Timeout Expiration Test**:
  - 2 requests remain hanging in flight past `GracefulStop`.
  - Assert worker context cancellation is triggered.
  - Assert hanging requests are terminated and counted as `OutcomeCanceled`.
- [x] **Hard Context Cancellation Test**:
  - Cancel parent `ctx` mid-run.
  - Assert immediate scheduler return, active requests canceled, state transitions to `StateCanceled`.
  - Assert `rep.Incomplete == true`.

### 2. Real-Clock Integration Tests with `internal/testtarget` (`internal/scheduler/open_test.go`)
- [x] **Fast 200 OK Target Test**:
  - Run against `testtarget` Mode 0 (Normal 200 OK): `rate: 100`, `time_unit: 1s`, `duration: 500ms`, `max_in_flight: 50`.
  - Assert ~50 planned, ~50 started, ~50 completed, 0 dropped.
  - Assert achieved start rate is within ±10% of target 100 RPS.
  - Assert 100% `OutcomeSuccess`.
- [x] **Higher Rate Target Test**:
  - Run against `testtarget` Mode 0: `rate: 500`, `time_unit: 1s`, `duration: 200ms`, `max_in_flight: 100`.
  - Assert ~100 requests executed, 0 dropped, clean metrics merge.
- [x] **Slow Target Overload & Dropped Requests Test**:
  - Run against `testtarget` Mode 2 (500ms delay): `rate: 100`, `time_unit: 1s`, `duration: 1s`, `max_in_flight: 10`.
  - Target latency is 500ms, so at 100 RPS, in-flight capacity (10) saturates within 100ms.
  - Assert `Dropped > 0` and `Started <= 30`.
  - Assert `Planned == Started + Dropped`.
  - Assert `OutcomeDropped` count equals `Dropped`.
  - Assert `AchievedStartRPS` separates from `CompletedThroughput`.
  - Assert generator health records saturation warning.
- [x] **429 Rate-Limited Target Test**:
  - Run against `testtarget` Mode 8 (Rate Limited): `rate: 50`, `time_unit: 1s`, `duration: 200ms`.
  - Assert `OutcomeRateLimited` is recorded and rate-limit headers are captured.
- [x] **Server Error & Disconnect Target Test**:
  - Run against `testtarget` Mode 1 (500 Internal Error) and Mode 5 (TCP Drop).
  - Assert errors are classified into `OutcomeUnexpectedStatus` and transport error classes.
  - Assert `inFlightCount` cleanly returns to 0 upon request completion/error.

### 3. Leak, Concurrency, and Bounded-Memory Tests (`internal/scheduler/leak_test.go`)
- [x] **Goroutine Leak Test**:
  - Capture runtime goroutines before `OpenScheduler.Run()`.
  - Run open scheduler with 100 RPS for 500ms against `testtarget`.
  - Assert active goroutines after run matches baseline (0 orphaned goroutines).
- [x] **Connection Leak Test**:
  - Run 1,000 open-model requests across connection-pooled executor.
  - Close executor; verify all idle TCP connections are closed and socket count returns to 0.
- [x] **Bounded-Memory Soak Test**:
  - Run 50,000 open-model request cycles with simulated saturation drops.
  - Measure heap allocation at 1,000, 10,000, and 50,000 iterations.
  - Assert heap memory remains strictly bounded with zero linear memory accumulation (< 15% variance).

### 4. Reporting & CLI Integration Tests (`internal/cli/cli_test.go`, `internal/report/terminal_test.go`)
- [x] **CLI Open-Model Execution Test**:
  - Execute `daegsa run --url <testtarget> --model open --rate 100 --time-unit 1s --max-in-flight 50 --duration 200ms`.
  - Assert exit code is 0 (`ExitCodeSuccess`).
  - Assert terminal output displays `Workload Model: open`, Target Rate, Achieved Start Rate, and Completed Throughput.
- [x] **CLI JSON Report Export Test**:
  - Execute `daegsa run --url <testtarget> --model open --rate 100 --duration 200ms --output-json <report.json>`.
  - Assert valid JSON file created on disk conforming to `report_schema_version: 1`.
  - Assert JSON contains `workload_model: "open"` and populated `request_counts` (Planned, Scheduled, Started, Completed, Dropped).
- [x] **CLI Slow-Target Overload Exit Code Test**:
  - Execute open-model run against slow target causing dropped requests and failures.
  - Assert exit code is 1 (`ExitCodeThresholdFailure`) when unexpected statuses/failures occur.

## Safety and failure behavior

- **Strict Concurrency Ceiling**: `max_in_flight` is hard-enforced atomically prior to dispatch. Requests never exceed configured concurrency limits, protecting both the target server and the generator from resource exhaustion.
- **No Unbounded Queues or Buffers**: When `in_flight >= max_in_flight`, arrivals are immediately dropped, recorded as `core.OutcomeDropped`, and discarded. No unbounded in-memory queues are maintained.
- **Anti-Catch-Up Burst Protection**: If scheduler execution is delayed due to system scheduling, GC pauses, or clock adjustments, the scheduler advances the scheduling baseline rather than firing an instantaneous stampede / catch-up burst of historical requests.
- **Bounded Goroutine Lifecycle**: All worker goroutines are owned by the scheduler, managed with `sync.WaitGroup`, cancellable via `context.Context`, and guaranteed to join cleanly on termination.
- **Deterministic Mathematical Reconciliation**: Metrics maintain strict invariant reconciliation: `Planned == Started + Dropped` and `Started == Completed + Canceled`.
- **Memory Bounding**: Error messages and rate-limit headers use fixed circular buffers (max 10 entries). HDR latency histograms maintain fixed bucket arrays (1µs to 1h with 3 sig figs).
- **Generator Saturation Warnings**: If CPU exceeds 85%, scheduler lag exceeds 50ms, or `max_in_flight` saturation drops occur, explicit warnings are emitted in both terminal and JSON reports.
- **Secret Redaction Invariance**: URLs, tokens, authorization headers, and cookies remain unconditionally redacted in all console logs, terminal reports, and JSON output.

## Acceptance gates

1. **Clean Compilation & Zero Diagnostics**: `go build ./...` and `go vet ./...` compile with 0 warnings, 0 diagnostics, and 0 errors.
2. **Deterministic Virtual-Time Accuracy**: Unit tests using `ControllableClock` verify exact arrival interval spacing, exact `max_in_flight` drop counts, anti-catch-up burst progression, graceful drain, and context cancellation.
3. **Real-Clock Target Integration**: Integration tests against `internal/testtarget` verify accurate open arrival rates (100 RPS, 500 RPS), start rate vs completed throughput separation, and dropped-request accounting on slow targets.
4. **Zero Goroutine & Connection Leaks**: Goroutine leak checks and connection pool teardown tests confirm 0 orphaned goroutines and 0 dangling TCP sockets after test completion.
5. **Bounded-Memory Soak Compliance**: 50,000-iteration open-model soak test confirms memory footprint remains strictly bounded with zero linear memory accumulation.
6. **Mathematical Reconciliation Invariance**: Metrics engine strictly preserves `Planned == Started + Dropped` and `Started == Completed + Canceled`.
7. **Full Terminal and JSON v1 Report Delivery**: Terminal ANSI reports and JSON reports (`report_schema_version: 1`) accurately report open-model metrics, target RPS, achieved start rate, completed throughput, max in-flight, dropped requests, and generator health.
8. **CLI Integration**: `daegsa run` executes open-model workloads from CLI flags and YAML manifests, producing correct console output, writing JSON reports, and returning canonical exit codes.
9. **Git & Code Hygiene**: `git diff --check` passes cleanly with zero whitespace or formatting errors.

## Explicit non-goals

- Implementing threshold DSL parsing and evaluation expressions (deferred to Phase 4).
- Implementing multi-token authentication pools or credential rotation (deferred to Phase 5).
- Implementing ramp, stress, spike, and soak profile compilers (deferred to Phase 6).
- Implementing multi-step scenarios or cookie session chains (deferred to Phase 7).
- Implementing Windows installer packaging, checksums, and SBOM generation (deferred to Phase 8).

## Open questions

*None. The open arrival-rate scheduling model, fractional interval spacing formula, monotonic clock integration, atomic `max_in_flight` drop semantics, scheduler lag calculation, anti-catch-up burst progression, terminal/JSON reporting fields, and exit code mappings are fully specified in `docs/DAEGSA_Implementation_Plan.md` and frozen for Phase 3.*

## Implementation handoff

### Changed files
- `internal/scheduler/open.go`: Created `OpenScheduler` with precision arrival pacing, atomic `max_in_flight` drops, scheduler lag tracking, anti-burst baseline advance, and bounded worker pool.
- `internal/scheduler/open_test.go`: Added deterministic virtual-time (`ControllableClock`) unit tests and real-clock `testtarget` integration tests.
- `internal/scheduler/leak_test.go`: Added goroutine leak, connection pool teardown, and bounded-memory soak tests for `OpenScheduler`.
- `internal/report/terminal.go`: Enhanced open-model summary display with Target Rate, Max In-Flight, and throughput metrics.
- `internal/report/terminal_test.go`: Added test verifying open-model terminal report formatting.
- `internal/cli/run.go`: Wired `OpenScheduler` instantiation and execution when `load.model == "open"`.
- `internal/cli/cli_test.go`: Added open-model CLI execution, JSON export, and error exit code tests.
- `benchmarks/scheduler_bench_test.go`: Created benchmarks for open arrival-rate scheduling at 1k, 10k, and 50k RPS.

### Behavior implemented
- **Open Arrival-Rate Scheduling**: Constant arrival rate with fractional interval spacing across configured `time_unit` (`interval = time_unit / rate`).
- **Strict `max_in_flight` Drops**: Atomic concurrency tracking; when `in_flight >= max_in_flight`, arrival is immediately dropped as `core.OutcomeDropped` without request dispatch or queuing.
- **Anti-Catch-Up Burst Progression**: Scheduler calculates lag (`actualTime - intendedTime`); if `lag > interval`, advances scheduling baseline to prevent catch-up stampedes.
- **Worker Pool Lifecycle**: Bounded worker execution, clean joins, graceful stop draining within `graceful_stop` window, cancellation aborts, zero goroutine leaks, and zero connection leaks.
- **Mathematical Invariant Reconciliation**: Strict adherence to `Planned == Started + Dropped` and `Started == Completed + Canceled`.
- **Generator Diagnostics**: Peak goroutine monitoring, CPU tracking, max scheduler lag recording, and saturation warnings for lag > 50ms and dropped requests.
- **Terminal & JSON v1 Reporting**: Formats open model metrics including Target Rate, Achieved Start Rate, Completed Throughput, Max In-Flight, and Dropped Requests.
- **CLI Integration**: `daegsa run` supports open model from CLI flags and YAML manifests with canonical exit codes.

### Commands run and results
- `go build ./...`: PASS (0 diagnostics, exit code 0)
- `go vet ./...`: PASS (0 diagnostics, exit code 0)
- `go test -v -count=1 ./...`: PASS (all packages pass, exit code 0)
- `go test -bench . -benchmem "github.com/charleszardd/daegsa/benchmarks"`: PASS (exit code 0)
- `git diff --check`: PASS (0 whitespace errors, exit code 0)

### Known limitations
- Threshold DSL evaluation expressions and threshold exit code rules are deferred to Phase 4.
- Multi-token authentication pools and credential rotation are deferred to Phase 5.
- Ramp, stress, and spike load profiles are deferred to Phase 6.

### Remaining unchecked test or acceptance items
- Acceptance gates and test checklist items are retained for independent verification by the tester agent.
