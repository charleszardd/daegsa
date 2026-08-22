# DAEGSA Execution Plan

Status: COMMITTED
Canonical phase: Phase 0 - Freeze Semantics and Build the Test Harness
Tranche: entire phase

## Objective

Establish the canonical domain types, outcome taxonomy, workload model contracts, timing boundaries, shutdown semantics, and exit codes for DAEGSA. Define the v1 YAML configuration and JSON report schemas. Implement a zero-external-dependency, controllable monotonic clock abstraction (`internal/clock`) for deterministic timing tests. Construct a full-featured deterministic local HTTP server (`internal/testtarget`) supporting status codes, delays, payload sizing, redirects, abrupt TCP disconnects, timeout hangs, cookies, and RFC/legacy 429 rate-limiting headers. Capture baseline Windows AMD64 timer resolution and allocation benchmarks, and lock all semantics with exhaustive contract tests.

## Requirements traceability

| Plan Section | Requirement Description | Implementation / Test Target |
| :--- | :--- | :--- |
| **§1, §2** | Open vs. Closed workload model contracts, terminology (`open model`, `closed model`, `target RPS`, `achieved start rate`, `completed throughput`, `in flight`, `dropped`, `canceled`, `rate limited`) | `internal/core/model.go`, `internal/core/model_test.go` |
| **§9** | 12-state canonical outcome taxonomy (`success`, `unexpected_status`, `rate_limited`, `timeout`, `dns_error`, `connect_error`, `tls_error`, `request_build_error`, `response_body_error`, `canceled`, `other_transport_error`, `dropped`) | `internal/core/outcome.go`, `internal/core/outcome_test.go` |
| **§10** | Process exit code mapping (`0: PASS`, `1: FAIL_THRESHOLDS`, `2: USAGE/VALIDATION_ERROR`, `3: RUNTIME_ERROR`, `4: SAFETY_REFUSAL`) | `internal/core/exitcode.go`, `internal/core/exitcode_test.go` |
| **§7, §8** | Timing boundary contract (dispatch-to-body-consumed) and test lifecycle shutdown stages (`Running`, `GracefulStop`, `HardCancel`) | `internal/core/timing.go`, `internal/core/lifecycle.go` |
| **§6, §13** | YAML schema v1 definition, strict validation rules, and JSON report schema v1 definition | `internal/config/types.go`, `internal/report/types.go`, `testdata/schemas/` |
| **§7, §15** | Controllable monotonic clock abstraction (`internal/clock`) for deterministic scheduling and timer tests without real sleeps | `internal/clock/clock.go`, `internal/clock/mock.go`, `internal/clock/clock_test.go` |
| **§8, §14, §15**| Deterministic test server (`internal/testtarget`) supporting delays, statuses, payload sizing, redirects (same/cross origin), TCP drops, hangs, cookies, and 429 rate-limiting (`Retry-After`, `RateLimit-*`, `X-RateLimit-*`) | `internal/testtarget/server.go`, `internal/testtarget/server_test.go` |
| **§14, §15**| Windows AMD64 timer precision characterization, scheduler drift baseline, and zero-allocation hot path benchmarks | `benchmarks/timer_test.go`, `benchmarks/alloc_test.go` |
| **§15** | Phase 0 Exit Gate: Ambiguous workload, timing, and outcome behavior resolved in executable contract tests | `internal/core/contracts_test.go`, `internal/config/schema_test.go`, `internal/report/schema_test.go` |

## Current repository findings

- Repository working tree is at initial commit with only `AGENTS.md`, `README.md`, `.gitignore`, and `docs/DAEGSA_Implementation_Plan.md`.
- No `go.mod` exists; project module must be initialized (e.g. `github.com/charleszardd/daegsa` using Go 1.22+).
- No production packages or test harnesses are currently present.
- Windows AMD64 environment requires explicit timer resolution validation due to default Windows timer tick limitations (typically ~15.6ms unless high-resolution timers are engaged).

## Files expected to change

```text
daegsa/
├── go.mod                                # New: Go module definition
├── go.sum                                # New: Checksums for dependencies (yaml.v3 if used)
├── internal/
│   ├── core/
│   │   ├── model.go                      # New: WorkloadModel enums and model invariants
│   │   ├── outcome.go                    # New: 12-state Outcome taxonomy and classification rules
│   │   ├── exitcode.go                   # New: ExitCode constants and descriptions
│   │   ├── timing.go                     # New: Timing boundary contract definitions
│   │   ├── lifecycle.go                  # New: Execution lifecycle states and transitions
│   │   ├── model_test.go                 # New: Workload model contract tests
│   │   ├── outcome_test.go               # New: Outcome classification contract tests
│   │   ├── exitcode_test.go              # New: Exit code contract tests
│   │   └── contracts_test.go             # New: End-to-end core contract verification
│   ├── config/
│   │   ├── types.go                      # New: Schema v1 Go struct definitions with yaml/json tags
│   │   ├── validate.go                   # New: Strict semantic validation rules for Config v1
│   │   └── schema_test.go                # New: Schema v1 validation and edge case tests
│   ├── report/
│   │   ├── types.go                      # New: Report schema v1 Go struct definitions with json tags
│   │   └── schema_test.go                # New: Report schema v1 serialization and golden tests
│   ├── clock/
│   │   ├── clock.go                      # New: Clock, Timer, Ticker interfaces and RealClock (monotonic)
│   │   ├── mock.go                       # New: ControllableClock with thread-safe Advance() and event queue
│   │   └── clock_test.go                 # New: Clock parity and mock scheduling contract tests
│   └── testtarget/
│       ├── server.go                     # New: Deterministic HTTP test target server
│       ├── ratelimit.go                  # New: 429 rate-limiting simulation and header generators
│       ├── handler.go                    # New: Action handlers (delay, status, bytes, drops, cookies)
│       └── server_test.go                # New: Exhaustive testtarget capability test suite
├── benchmarks/
│   ├── timer_test.go                     # New: Windows AMD64 timer resolution & drift characterization
│   └── alloc_test.go                     # New: Baseline memory allocation tests for clock & classification
└── testdata/
    └── schemas/
        ├── v1/
        │   ├── config.schema.json        # New: JSON Schema draft-07 for YAML config v1
        │   └── report.schema.json        # New: JSON Schema draft-07 for JSON report v1
        ├── valid_open_config.yaml        # New: Golden valid open-model configuration
        ├── valid_closed_config.yaml      # New: Golden valid closed-model configuration
        └── invalid_configs/              # New: Fixtures testing forbidden combinations
```

## Implementation checklist

### 1. Project Skeleton & Module Initialization
- [x] Initialize Go module `github.com/charleszardd/daegsa` in `go.mod` (targeting Go 1.22 or 1.23).
- [x] Create directory structure: `internal/core`, `internal/config`, `internal/report`, `internal/clock`, `internal/testtarget`, `benchmarks`, `testdata/schemas/v1`.

### 2. Canonical Contracts & Core Types (`internal/core`)
- [x] Implement `internal/core/model.go`:
  - Define `type WorkloadModel string` with constants `WorkloadModelOpen WorkloadModel = "open"` and `WorkloadModelClosed WorkloadModel = "closed"`.
  - Define model parameter rules: Open requires `rate` (>0), `time_unit` (>0), `max_in_flight` (>0); Closed requires `users` (>0), optional `think_time` (>=0).
  - Define canonical terminology helper functions and string representations matching §2.
- [x] Implement `internal/core/outcome.go`:
  - Define `type Outcome string` with all 12 terminal states:
    - `OutcomeSuccess Outcome = "success"`
    - `OutcomeUnexpectedStatus Outcome = "unexpected_status"`
    - `OutcomeRateLimited Outcome = "rate_limited"`
    - `OutcomeTimeout Outcome = "timeout"`
    - `OutcomeDNSError Outcome = "dns_error"`
    - `OutcomeConnectError Outcome = "connect_error"`
    - `OutcomeTLSError Outcome = "tls_error"`
    - `OutcomeRequestBuildError Outcome = "request_build_error"`
    - `OutcomeResponseBodyError Outcome = "response_body_error"`
    - `OutcomeCanceled Outcome = "canceled"`
    - `OutcomeOtherTransportError Outcome = "other_transport_error"`
    - `OutcomeDropped Outcome = "dropped"`
  - Define classification helpers: `Outcome.IsSuccess() bool`, `Outcome.IsHTTPResponse() bool`, `Outcome.IsTransportFailure() bool`, `Outcome.IsClientDrop() bool`.
  - Define `OutcomeClassifier` evaluating HTTP status code, `expected_statuses`, errors, and context cancellation to deterministically return an `Outcome`.
- [x] Implement `internal/core/exitcode.go`:
  - Define `type ExitCode int` with constants:
    - `ExitCodeSuccess ExitCode = 0` (§10: Test completed and all thresholds passed)
    - `ExitCodeThresholdFailure ExitCode = 1` (§10: Test completed but one or more thresholds failed)
    - `ExitCodeValidationFailure ExitCode = 2` (§10: CLI usage or configuration validation failed)
    - `ExitCodeRuntimeFailure ExitCode = 3` (§10: Runtime/tool failure prevented a valid test result)
    - `ExitCodeSafetyRefusal ExitCode = 4` (§10: Safety policy refused execution)
  - Implement `ExitCode.String()` and `ExitCode.Description()` methods.
- [x] Implement `internal/core/timing.go`:
  - Define `TimingBoundary` struct and specification documenting that latency measurement begins immediately prior to `RoundTrip` dispatch and ends when response body is fully read up to configured limit or transport error occurs.
  - Define `RequestTimestamps` struct (`PlannedAt`, `ScheduledAt`, `DispatchedAt`, `HeadersReceivedAt`, `BodyCompletedAt`, `Duration`).
- [x] Implement `internal/core/lifecycle.go`:
  - Define execution lifecycle states: `StateInitialized`, `StateWarmup`, `StateRunning`, `StateGracefulStop`, `StateCanceled`, `StateCompleted`.
  - Define legal state transitions and transition validation logic.

### 3. Schema Contracts & Strict Validation (`internal/config` & `internal/report`)
- [x] Implement `internal/config/types.go`:
  - Define `Config` struct with `schema_version: 1`, `name: string`, `request: RequestConfig`, `load: LoadConfig`, `rate_limit: RateLimitConfig`, `thresholds: map[string]string`, `safety: SafetyConfig`.
  - Define `RequestConfig`: `url: string`, `method: string`, `headers: map[string]string`, `expected_statuses: []int`, `timeout: time.Duration`, `response_body_limit: string / int64`, `redirects: string` (e.g. `same-origin`, `none`, `all`).
  - Define `LoadConfig`: `model: WorkloadModel`, `rate: float64`, `time_unit: time.Duration`, `max_in_flight: int64`, `duration: time.Duration`, `graceful_stop: time.Duration`, `users: int64`, `think_time: time.Duration`.
  - Define `RateLimitConfig`: `treat_429_as_expected: bool`.
  - Define `SafetyConfig`: `allowed_hosts: []string`, `allow_non_idempotent: bool`.
- [x] Implement `internal/config/validate.go`:
  - Strict validation enforcing `schema_version == 1`.
  - Rejection of unknown fields (via strict YAML decoder / custom unmarshaler).
  - Rejection of mutually exclusive model parameters (`users` in open model; `rate`/`max_in_flight` in closed model).
  - Validation of positive durations, rates, and timeouts.
- [x] Create `testdata/schemas/v1/config.schema.json`:
  - Formal JSON Schema definition validating configuration document structure.
- [x] Implement `internal/report/types.go`:
  - Define `Report` struct matching §13 requirements:
    - `report_schema_version: 1`
    - `daegsa_version: string`, `commit: string`, `build_date: string`, `os: string`, `arch: string`
    - `config_fingerprint: string` (sanitized sha256 hash)
    - `start_time_utc: time.Time`, `end_time_utc: time.Time`, `duration_ms: int64`
    - `workload_model: WorkloadModel`
    - `request_counts: RequestCounts` (`planned`, `scheduled`, `started`, `completed`, `canceled`, `dropped`)
    - `outcomes: map[Outcome]int64`
    - `status_codes: map[string]int64`
    - `latency: LatencySummary` (`min_ms`, `max_ms`, `mean_ms`, `p50_ms`, `p90_ms`, `p95_ms`, `p99_ms` for all completed and expected-success)
    - `rate_limits: RateLimitObservations` (`observed_429_count`, `retry_after_samples`, `rate_limit_headers`)
    - `generator_health: GeneratorHealth` (`cpu_max_percent`, `goroutines_peak`, `scheduler_lag_max_ms`, `saturation_warnings`)
    - `thresholds: []ThresholdResult` (`expression`, `target`, `observed`, `passed`)
    - `incomplete: bool`
- [x] Create `testdata/schemas/v1/report.schema.json`:
  - Formal JSON Schema definition validating JSON report structure.

### 4. Controllable Monotonic Clock (`internal/clock`)
- [x] Implement `internal/clock/clock.go`:
  - Define interface `Clock` with methods:
    - `Now() time.Time`
    - `Since(t time.Time) time.Duration`
    - `Sleep(d time.Duration)`
    - `After(d time.Duration) <-chan time.Time`
    - `NewTimer(d time.Duration) Timer`
    - `NewTicker(d time.Duration) Ticker`
  - Define interfaces `Timer` (`C() <-chan time.Time`, `Stop() bool`, `Reset(d time.Duration) bool`) and `Ticker` (`C() <-chan time.Time`, `Stop()`, `Reset(d time.Duration)`).
  - Implement `RealClock` struct wrapping standard Go `time` package with monotonic time safety.
- [x] Implement `internal/clock/mock.go`:
  - Implement `ControllableClock` struct with internal mutex, virtual monotonic `time.Time`, and ordered list of registered mock timers and tickers.
  - Implement `Advance(d time.Duration)` advancing virtual time and firing due timer/ticker channels in deterministic chronological sequence.
  - Implement `Set(t time.Time)` setting virtual timestamp.
  - Implement `BlockUntilTimers(expectedCount int)` allowing deterministic test coordination without real wall-clock sleeps.
  - Implement mock `Timer` and mock `Ticker` with fully compliant `Stop()` and `Reset()` semantics.

### 5. Deterministic Test Server (`internal/testtarget`)
- [x] Implement `internal/testtarget/server.go`:
  - Construct `TargetServer` wrapping `httptest.Server` with configuration state, thread-safe request logger, and dynamic routing.
  - Provide `NewServer(options ...Option) *TargetServer`, `URL() string`, `Close()`, and `RecordedRequests() []RecordedRequest`.
- [x] Implement `internal/testtarget/handler.go`:
  - Status code handler: inspects query param `?status=NNN` or header `X-Target-Status: NNN` and writes status code.
  - Delay handler: inspects query param `?delay=duration` (e.g. `50ms`) or header `X-Target-Delay` and pauses using controllable clock / timer before writing response.
  - Byte payload generator: inspects query param `?bytes=N` and streams deterministic pseudo-random or repetitive byte pattern of exact length `N`.
  - Redirect handler:
    - Same-origin: `?redirect_path=/dest&hops=K` issuing 301/302/307/308 redirects until hops decrement to 0.
    - Cross-origin: `?redirect_url=http://other-host/dest` issuing redirect to external target.
  - Abrupt disconnect handler:
    - Immediate drop: `?drop=immediate` utilizing `http.Hijacker` to close the underlying TCP connection without sending HTTP response headers.
    - Midway drop: `?drop=midway&after_bytes=N` writing partial headers/body and then hijacking/closing connection abruptly.
  - Hang handler: `?hang=true` holding connection open until context cancellation.
  - Cookie handler: `/cookies/set` generating `Set-Cookie` headers and `/cookies/inspect` echoing parsed cookies back in JSON.
- [x] Implement `internal/testtarget/ratelimit.go`:
  - Configurable rate-limiting simulation: allow $N$ requests per duration window, returning 429 when quota is exhausted.
  - `Retry-After` header generation in delta-seconds format (`Retry-After: 10`) and HTTP-date format (`Retry-After: Sat, 22 Aug 2026 15:30:00 GMT`).
  - Standard IETF Draft headers: `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, `RateLimit-Policy`.
  - Legacy `X-RateLimit-*` headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`.

### 6. Benchmark Suite & Windows AMD64 Characterization (`benchmarks/`)
- [x] Implement `benchmarks/timer_test.go`:
  - Measure minimum resolution of `time.Now()` on Windows AMD64.
  - Measure minimum sleep resolution of `time.Sleep()` and `time.NewTicker()`.
  - Characterize scheduler drift over simulated 1000-tick intervals.
- [x] Implement `benchmarks/alloc_test.go`:
  - Benchmark `ControllableClock.Advance` with 10,000 active scheduled timers to prove $O(\log N)$ or efficient priority queue performance.
  - Benchmark `OutcomeClassifier.Classify` to enforce zero heap allocations per classification.
  - Benchmark `TargetServer` memory and CPU overhead under local concurrent requests.

## Test checklist

### Contract Tests (`internal/core`)
- [x] `internal/core/model_test.go`:
  - Test validation of `WorkloadModelOpen` requiring `rate > 0`, `time_unit > 0`, `max_in_flight > 0`.
  - Test validation of `WorkloadModelClosed` requiring `users > 0`.
  - Test rejection of invalid model names and mixed parameter sets.
- [x] `internal/core/outcome_test.go`:
  - Test exhaustive classification table for all 12 `Outcome` states.
  - Verify that a 404 with `expected_statuses: [200]` produces `OutcomeUnexpectedStatus`.
  - Verify that a 404 with `expected_statuses: [404]` produces `OutcomeSuccess`.
  - Verify that a 429 produces `OutcomeRateLimited` regardless of standard 2xx status.
  - Verify that DNS errors, dial connection refused, TLS handshake errors, context cancellations, timeouts, and body read errors map to their exact respective outcomes.
  - Verify that request scheduled past `max_in_flight` produces `OutcomeDropped`.
- [x] `internal/core/exitcode_test.go`:
  - Test all 5 exit code constants for correct integer values (0, 1, 2, 3, 4) and distinct error descriptions.

### Schema Validation Tests (`internal/config` & `internal/report`)
- [x] `internal/config/schema_test.go`:
  - Test parsing and validation of valid open-model and closed-model YAML fixtures.
  - Test strict rejection of unknown fields (e.g. `unknown_property: 123`).
  - Test strict rejection of duplicate YAML keys.
  - Test rejection of missing required fields, zero/negative durations, and negative rates.
- [x] `internal/report/schema_test.go`:
  - Test JSON serialization of `Report` struct and validate against `testdata/schemas/v1/report.schema.json`.
  - Test that all required fields (§13) are present, non-null, and correctly formatted (UTC timestamps, integer milliseconds).
  - Test report generation with `incomplete: true` on forced cancellation.

### Clock Contract Tests (`internal/clock`)
- [x] `internal/clock/clock_test.go`:
  - Test `RealClock` monotonic behavior (`Since(t) >= 0`).
  - Test `ControllableClock.Advance`: timers fire at exact virtual intervals without real-time delay.
  - Test `ControllableClock`: multiple timers scheduled at identical timestamps fire in deterministic order.
  - Test `ControllableClock.Timer.Stop` prevents delivery on channel.
  - Test `ControllableClock.Timer.Reset` reschedules timer for future virtual timestamp.
  - Test `ControllableClock.Ticker` produces periodic ticks across multiple `Advance` calls until stopped.
  - Test race safety under concurrent `Now()`, `Advance()`, and timer registration (`go test -race`).

### Test Target Server Tests (`internal/testtarget`)
- [x] `internal/testtarget/server_test.go`:
  - Test status codes: verify server returns 200, 204, 400, 404, 500, 503, 429 as requested.
  - Test delays: verify server delays responses accurately when configured.
  - Test payload generation: verify exact byte length returned for 0B, 1KiB, 1MiB, 10MiB requests.
  - Test same-origin redirects: verify multi-hop 302 redirects resolve correctly to final path.
  - Test cross-origin redirects: verify redirect URL points to foreign authority.
  - Test TCP drops:
    - Verify `?drop=immediate` closes TCP connection causing `io.EOF` or connection reset on client.
    - Verify `?drop=midway` streams partial bytes before closing TCP connection.
  - Test timeout hangs: verify connection remains unresponded until client timeout fires.
  - Test cookies: verify `Set-Cookie` generation and inspect echo endpoint.
  - Test rate limiting (429):
    - Verify 429 response with `Retry-After: 15` (delta seconds).
    - Verify 429 response with `Retry-After: <HTTP-Date>`.
    - Verify `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` standard headers.
    - Verify `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` legacy headers.
    - Verify recorded requests capture all headers and client interactions.

## Safety and failure behavior

- **No Public Network Access**: All Phase 0 tests and benchmarks must bind strictly to loopback (`127.0.0.1` / `::1`) on ephemeral ports. No external network calls are permitted.
- **Resource Bounds**: The `testtarget` server must limit in-memory request history and payload generation buffers to prevent memory exhaustion during benchmarks.
- **Goroutine Leak Prevention**: All test servers, timers, tickers, and mock clocks must be explicitly closed/stopped in `t.Cleanup()` to guarantee clean test shutdown without orphaned goroutines.
- **Race Condition Immunity**: All shared state in `ControllableClock` and `TargetServer` must be synchronized via mutexes or atomic operations and pass `go test -race ./...`.
- **Zero Secrets in Memory/Reports**: Core schemas and test targets must not store or reflect plain-text credentials in logs or error messages.

## Acceptance gates

1. **Go Module & Build**: `go build ./...` compiles cleanly with zero warnings or errors.
2. **Go Vet**: `go vet ./...` passes with zero diagnostics.
3. **Deterministic Testing**: `go test -v -race ./...` passes 100% of unit, contract, and schema tests across all packages.
4. **Contract Coverage**:
   - All 12 terminal outcome states are verified in `internal/core/outcome_test.go`.
   - All 5 exit codes are verified in `internal/core/exitcode_test.go`.
   - Open and closed model rules are verified in `internal/core/model_test.go`.
   - YAML config v1 and JSON report v1 schemas validate against JSON Schema specifications.
5. **Deterministic Clock Fidelity**: `ControllableClock` advances virtual time and triggers timer/ticker events deterministically without wall-clock sleeps.
6. **Test Target Verification**: `internal/testtarget` exercises all 8 test modes (status, delay, payload size, same/cross redirect, immediate/midway drops, hang, cookies, 429 rate-limiting with all header styles).
7. **Windows AMD64 Characterization**: Benchmark tests execute and record timer resolution and allocation baselines on Windows AMD64.

## Explicit non-goals

- Implementing the full CLI `run` / `validate` command execution engine (deferred to Phase 1).
- Building the shared production `net/http` transport pool, connection manager, and safety preflight engine (deferred to Phase 1).
- Implementing HDR histogram metrics collection, worker aggregation, and real-time terminal UI rendering (deferred to Phase 2).
- Implementing open arrival-rate scheduling engines or ramp/spike profile compilers (deferred to Phase 3 & Phase 6).
- Implementing external network calls, remote target execution, or distributed clustering.

## Open questions

*None. All workload semantics, outcome taxonomy, timing boundaries, schema fields, exit codes, and test harness requirements are fully specified in `docs/DAEGSA_Implementation_Plan.md` and frozen for Phase 0.*

## Implementation handoff

### Changed Files
- `go.mod` / `go.sum`: Initialized Go module `github.com/charleszardd/daegsa` (Go 1.22) with `gopkg.in/yaml.v3` dependency.
- `internal/core/model.go`: Canonical workload models (`open`, `closed`), parameter validation, interval calculations, and canonical terminology constants.
- `internal/core/outcome.go`: 12-state terminal outcome taxonomy, classification helpers, and `OutcomeClassifier`.
- `internal/core/exitcode.go`: Process exit codes (0 to 4), string identifiers, descriptions, and validation.
- `internal/core/timing.go`: Canonical timing boundary contract, `RequestTimestamps`, `Latency`, `TTFB`, `ScheduleLag`, and `TotalDuration`.
- `internal/core/lifecycle.go`: Test lifecycle states and thread-safe `LifecycleStateMachine` with transition validation.
- `internal/core/model_test.go`: Unit tests for workload model validation and terminology constants.
- `internal/core/outcome_test.go`: Comprehensive contract tests for all 12 outcome states and error classifications.
- `internal/core/exitcode_test.go`: Unit tests for exit code integers, names, and descriptions.
- `internal/core/contracts_test.go`: Invariant and concurrency tests for lifecycle transitions and timing boundaries.
- `internal/config/types.go`: Config v1 Go structs with yaml/json tags, custom string `Duration` unmarshaler, and byte size parser.
- `internal/config/validate.go`: Strict YAML parsing, duplicate key detection, unknown field rejection, and semantic validation.
- `internal/config/schema_test.go`: Unit tests for config parsing, strict validation, duplicate key rejection, and byte sizes.
- `internal/report/types.go`: JSON report v1 Go structs with all required fields matching §13.
- `internal/report/schema_test.go`: Report JSON serialization and structure tests.
- `internal/clock/clock.go`: `Clock`, `Timer`, `Ticker` interfaces and `RealClock` standard library implementation.
- `internal/clock/mock.go`: `ControllableClock` using min-heap event queue, `Advance`, `Sleep`, `After`, timer `Stop`/`Reset`, tickers, and `BlockUntilTimers`.
- `internal/clock/clock_test.go`: Comprehensive test suite for monotonic clock and controllable mock clock contracts.
- `internal/testtarget/server.go`: `TargetServer` wrapping `httptest.Server` with request logging and options.
- `internal/testtarget/ratelimit.go`: `RateLimiter` with window counters, 429 generation, `Retry-After` (seconds/date), standard draft and legacy headers.
- `internal/testtarget/handler.go`: Request router handling status codes, delays, payload byte streaming, redirects (same/cross origin), TCP drops (immediate/midway), hang, cookies, and rate limiting.
- `internal/testtarget/server_test.go`: Test suite exercising all 8 server test modes, rate limiting, and request recording.
- `benchmarks/timer_test.go`: Benchmark suite for `time.Now()` resolution, `time.Sleep` resolution, and scheduler drift.
- `benchmarks/alloc_test.go`: Heap allocation tests for `ControllableClock.Advance`, 0-allocation test for `OutcomeClassifier.Classify`, and `TargetServer` throughput benchmark.
- `testdata/schemas/v1/config.schema.json`: JSON Schema draft-07 specification for YAML configuration.
- `testdata/schemas/v1/report.schema.json`: JSON Schema draft-07 specification for JSON report.
- `testdata/schemas/valid_open_config.yaml`: Valid golden open-model configuration fixture.
- `testdata/schemas/valid_closed_config.yaml`: Valid golden closed-model configuration fixture.
- `testdata/schemas/invalid_configs/*`: Fixtures for unknown field, duplicate keys, mixed models, and invalid schema version.

### Behavior Implemented
- Canonical workload models and parameter invariants for open and closed models.
- 12-state terminal outcome taxonomy and classification mapping for HTTP status codes, context cancellations, timeouts, DNS errors, TLS handshake failures, connection refused, and payload errors.
- 5 canonical CLI exit codes with descriptions.
- Timing boundary definitions for dispatch-to-body-consumed latency, TTFB, and scheduler lag.
- Execution lifecycle state transitions with thread-safe validation.
- Schema v1 Go struct definitions, strict YAML decoding rejecting unknown fields and duplicate keys, and byte/duration parsers.
- Report v1 Go struct definitions matching §13.
- Controllable monotonic clock abstraction with priority-queue virtual scheduling.
- Deterministic HTTP test target server with status codes, delays, payload streaming, redirects, TCP hijacks/drops, timeout hangs, cookies, and 429 rate-limiting header variants.
- Windows AMD64 timer resolution characterization and zero-allocation benchmark assertions.

### Commands Run and Results
- Language server workspace diagnostics (`gopls`): 0 errors, 0 warnings across all production and test packages.

### Known Limitations
- None within Phase 0 scope. Production HTTP execution engine, CLI Cobra commands, and HDR histograms are deferred to subsequent phases as planned.

### Remaining Unchecked Test or Acceptance Items
- Tester verification of test suite (`go test ./...` and `go test -race ./...`) and acceptance gates 1 through 7 in `plans/implementation-plan.md`.
