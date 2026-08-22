# DAEGSA Execution Plan
Status: COMMITTED
Canonical phase: Phase 7 - Multi-Step Scenarios
Tranche: entire phase

## Objective

Implement closed-model multi-step scenario workflows where virtual users (VUs) execute sequential HTTP request steps with strictly isolated session state and private cookie jars. Support deterministic variable extraction (JSONPath, HTTP headers, cookies, regular expressions), dynamic variable substitution across URLs, headers, and request bodies, configurable step failure policies (`stop`, `abort_vu`, `continue`), per-step think times, step-level metrics collection and histogram aggregation, step-specific threshold evaluation (`step.<step_name>.<metric>`), enhanced terminal and JSON reporting, and deterministic end-to-end integration tests using `internal/testtarget`.

## Requirements traceability

- **§2 Workload Model and Terminology:** Closed model for session and multi-step workflow testing; fixed number of virtual users executing sequential step loops with isolated session state and think times.
- **§4 High-Level Architecture:** Workload Controller -> Closed VU Scheduler -> Scenario Executor -> Per-VU State & Request Builder -> Shared HTTP Transport.
- **§6 Configuration Contract:** Strict YAML schema validation; scenario definition with named steps; unknown field and duplicate key rejection; secret reference handling.
- **§7 Execution Semantics:** Closed scheduler VU loops; per-step and per-iteration think time; step failure handling (`stop`, `abort_vu`, `continue`); graceful drain and cancellation.
- **§8 HTTP Correctness:** Shared tuned `http.Transport`; per-VU isolated `http.CookieJar`; safe response body reading and keep-alive draining; connection reuse across steps.
- **§9 Outcome and Metrics Model:** Step-level and scenario-level metrics; per-step latency percentiles, outcome taxonomy, and HTTP status distributions; memory-bounded histograms.
- **§10 Thresholds and Exit Codes:** Step-specific threshold syntax (`step.<step_name>.<metric>`); evaluation against step metrics snapshots; stable exit codes (0 for pass, 1 for threshold failure, 2 for invalid config, 3 for runtime failure, 4 for safety refusal).
- **§11 Authentication, Secrets, and State:** Multi-step per-VU state (`VUState`); isolated cookie jars; deterministic token pools; redaction of extracted credentials and sensitive headers.
- **§12 Safety Controls:** Preflight safety verification applied to all scenario step URLs and HTTP methods; host allowlist enforcement; destructive method safeguards (`allow_non_idempotent`).
- **§13 Reports and Reproducibility:** Terminal and versioned JSON reports with scenario summary, iteration counts, and step-level latency/outcome breakdowns.
- **§15 Implementation Phases (Phase 7 - Multi-Step Scenarios):** Per-VU state and cookies; JSON-path extraction and variable substitution; think time and deterministic data selection; login and refresh flows; step-level metrics and thresholds. Exit gate: users remain isolated and scenario failures have explicit stop/continue behavior.

## Current repository findings

- **`internal/config/types.go` & `validate.go`:** Currently models single-request configurations (`RequestConfig`) alongside `LoadConfig` (open/closed/profile). Must be extended to support `ScenarioConfig`, `StepConfig`, `ExtractRuleConfig`, and `OnFailurePolicy` with mutual exclusivity validation (either single `request` or multi-step `scenario` defined).
- **`internal/plan/plan.go`:** `BuildPlan` validates and deep-clones a single target request. Must be extended to compile multi-step scenarios into immutable `CompiledScenario` and `CompiledStep` structures with safety preflight executed across all step URLs and HTTP methods.
- **`internal/auth/jar.go`:** `VUJarManager` already provides bounded, pre-allocated `http.CookieJar` instances per VU index, providing the foundation for `VUState` cookie isolation.
- **`internal/executor/executor.go` & `request.go`:** Currently builds requests from static `*plan.Plan` properties. Must be refactored or complemented with parameterized request building supporting dynamic URL, header, and body variable substitution.
- **`internal/scheduler/closed.go`:** `runVU` currently issues single HTTP requests in a loop. Must support running a sequence of scenario steps per iteration via a scenario executor, handling step failure policies, step think times, and step metrics collection.
- **`internal/metrics/aggregate.go` & `worker.go`:** `WorkerMetrics` and `AggregatedMetrics` track root-level and segment-level request metrics. Must be extended to track and merge step-level metrics (`StepMetrics` / `Steps map[string]*WorkerMetrics`) and scenario iteration counters.
- **`internal/threshold/parser.go` & `evaluator.go`:** Parses canonical root metrics. Must support step-qualified metrics (e.g. `step.login.p95`, `step.checkout.http_error_rate`) and evaluate them against corresponding step metrics snapshots.
- **`internal/report/types.go`, `builder.go`, & `terminal.go`:** Currently generates single-request and profile-segment reports. Must incorporate scenario execution metadata, iteration counts, and a step breakdown table.
- **`internal/testtarget/handler.go`:** Implements mock auth and cookie endpoints. Needs multi-step workflow endpoints (`/auth/login`, `/api/items`, `/api/logout`, `/scenario/fail-step`, `/scenario/dynamic`) for realistic workflow contract tests.

## Files expected to change

- **Create:**
  - `internal/scenario/types.go` — Domain types: `Step`, `Scenario`, `VUState`, `ExtractionRule`, `ExtractionSource`, `OnFailurePolicy`.
  - `internal/scenario/extract.go` — Extraction engine: JSONPath, HTTP header, cookie, and regex extractors with boundary error handling.
  - `internal/scenario/substitute.go` — Variable substitution engine: template parsing, `${var}` replacement in URLs, headers, and bodies.
  - `internal/scenario/executor.go` — `ScenarioExecutor`: step execution loop, variable extraction, think time enforcement, and failure policy handling.
  - `internal/scenario/extract_test.go` — Unit tests for extraction rules, missing keys, invalid expressions, and edge cases.
  - `internal/scenario/substitute_test.go` — Unit tests for variable substitution in URLs, headers, JSON bodies, and escaping.
  - `internal/scenario/executor_test.go` — Unit tests for step iteration, state propagation, and failure policies.
  - `internal/scenario/isolation_test.go` — Concurrency tests verifying strict per-VU variable and cookie isolation.
  - `examples/multi-step-scenario.yaml` — Complete example demonstrating login -> extract token/cookie -> query items -> logout workflow.
- **Modify:**
  - `internal/config/types.go` — Add `ScenarioConfig`, `StepConfig`, `ExtractRuleConfig`, and failure policy constants.
  - `internal/config/validate.go` — Add scenario validation: step name uniqueness, valid extraction sources, failure policy values, and model validation.
  - `internal/config/validate_test.go` — Add validation test cases for scenario syntax, duplicate step names, invalid extraction rules, and negative think times.
  - `internal/plan/plan.go` — Support compiling scenarios in `BuildPlan`; preflight all step URLs and methods against safety allowlists.
  - `internal/plan/plan_test.go` — Tests for plan compilation with multi-step scenarios.
  - `internal/metrics/worker.go` & `aggregate.go` — Add step-level metric accumulators, merge logic, and scenario iteration counters.
  - `internal/metrics/metrics_test.go` — Unit tests verifying step metric aggregation and reconciliation with root metrics.
  - `internal/threshold/types.go`, `parser.go`, & `evaluator.go` — Support `step.<step_name>.<metric>` threshold parsing and step snapshot evaluation.
  - `internal/threshold/threshold_test.go` — Tests for step threshold parsing, validation, and evaluation.
  - `internal/scheduler/closed.go` — Integrate scenario step execution in VU worker loop with think times and failure policies.
  - `internal/scheduler/closed_test.go` — Tests for closed-model scenario execution, duration expiration, graceful stop, and cancellation.
  - `internal/report/types.go`, `builder.go`, & `terminal.go` — Add scenario step summary to JSON schema and terminal output.
  - `internal/report/report_test.go` — Tests verifying scenario and step metrics rendering in terminal and JSON formats.
  - `internal/testtarget/handler.go` & `server.go` — Add multi-step API endpoints (`/auth/login`, `/api/items`, `/api/logout`, `/scenario/fail-step`).
  - `internal/testtarget/server_test.go` — Tests for testtarget scenario endpoints.
  - `README.md` — Document multi-step scenario configuration syntax, extraction rules, and step-level thresholds.

## Implementation checklist

### 1. Scenario Schema, Models, and Validation (`internal/config`, `internal/scenario`)
- [x] In `internal/config/types.go`, define `ScenarioConfig`, `StepConfig`, `ExtractRuleConfig`, and `OnFailurePolicy` (`stop`, `abort_vu`, `continue`).
- [x] In `internal/config/types.go`, add `Scenario *ScenarioConfig` to `Config` struct.
- [x] In `internal/config/validate.go`, enforce mutual exclusivity: exactly one of `Request` (single request) or `Scenario` (multi-step workflow) must be specified.
- [x] In `internal/config/validate.go`, validate `ScenarioConfig`: scenario name non-empty, steps count bounded (1 to 50 steps), step names non-empty and unique within scenario.
- [x] In `internal/config/validate.go`, validate each `StepConfig`: valid HTTP method, valid URL/template, valid `expected_statuses`, timeout >= 0, response body limit, redirects, and non-negative `think_time`.
- [x] In `internal/config/validate.go`, validate `ExtractRuleConfig`: extraction source must be `json` (or `jsonpath`), `header`, `cookie`, or `regex`; expression must be non-empty and syntactically valid (e.g. valid regex compilation).
- [x] In `internal/config/validate.go`, validate `OnFailure`: must be one of `stop` (default), `abort_vu`, or `continue`.
- [x] In `internal/config/validate.go`, enforce that scenarios require `load.model: closed`.
- [x] Add comprehensive test cases in `internal/config/validate_test.go` for all scenario validation paths.

### 2. Per-VU State Management & Variable Substitution (`internal/scenario`)
- [x] In `internal/scenario/types.go`, define `VUState` holding `VUID int`, `Iteration int64`, `Variables map[string]string`, `CookieJar http.CookieJar`, and `DeterministicTokens []string`.
- [x] Implement `NewVUState(vuID int, jar http.CookieJar, initialVars map[string]string) *VUState`.
- [x] Implement `ResetIteration()` on `VUState` to clean or reset per-iteration variables while preserving session variables/cookies as configured.
- [x] In `internal/scenario/substitute.go`, implement `SubstituteVariables(template string, vars map[string]string) (string, error)` that resolves `${var_name}` placeholders.
- [x] Support placeholder substitution in URL paths/queries, request header values, and request bodies.
- [x] Handle escaping (e.g. `$${LITERAL}` -> `${LITERAL}`) and return descriptive errors for unresolvable variables.
- [x] Write unit tests in `internal/scenario/substitute_test.go` for URL, header, JSON body, escaping, and missing variable scenarios.

### 3. Response Extraction Engine (`internal/scenario`)
- [x] In `internal/scenario/extract.go`, implement JSON/JSONPath extraction supporting dot notation and array indexing (`token`, `$.token`, `data.user.id`, `$.items[0].id`) without external heavy dependencies, using standard JSON AST parsing.
- [x] In `internal/scenario/extract.go`, implement HTTP header extraction (`from: header`, `expression: "Header-Name"`).
- [x] In `internal/scenario/extract.go`, implement cookie extraction (`from: cookie`, `expression: "cookie_name"`) from response `Set-Cookie` and VU cookie jar.
- [x] In `internal/scenario/extract.go`, implement regular expression extraction (`from: regex`, `expression: "pattern_with_group"`), extracting the first capture group.
- [x] Implement `ExtractAll(resp *http.Response, body []byte, rules map[string]ExtractRuleConfig, state *VUState) error` which applies all rules and stores results into `state.Variables`.
- [x] Ensure extraction failures (missing JSON key, header absent, regex no match) return distinct, descriptive errors without crashing or leaking sensitive payload data.
- [x] Write unit tests in `internal/scenario/extract_test.go` covering all extraction sources, type conversions (number/boolean to string), malformed JSON/regex, and missing keys.

### 4. Scenario Execution Engine (`internal/scenario`, `internal/executor`)
- [x] In `internal/scenario/types.go`, define `StepResult` capturing step name, step index, HTTP `executor.Result`, extraction errors, and whether the step succeeded.
- [x] In `internal/scenario/executor.go`, implement `ScenarioExecutor` holding shared `http.Transport`, compiled scenario definition, safety policy, clock, and outcome classifier.
- [x] Implement `ExecuteStep(ctx context.Context, state *VUState, step *CompiledStep) (*StepResult, error)`:
  - Perform variable substitution on step URL, headers, and body using `state.Variables`.
  - Build HTTP request with request context and timeout.
  - Execute request using shared `http.Transport` and VU's private `http.CookieJar`.
  - Read and safely drain response body up to `ResponseBodyLimit`.
  - Classify outcome and check against `expected_statuses`.
  - If request succeeded and extraction rules exist, run `ExtractAll` and store extracted variables in `state.Variables`.
  - If extraction fails, classify step outcome as extraction error (`OutcomeUnexpectedStatus` or custom classification) and record failure.
- [x] In `ScenarioExecutor.ExecuteIteration(ctx context.Context, state *VUState, onStepDone func(stepResult *StepResult)) (bool, error)`:
  - Iterate through compiled scenario steps sequentially.
  - Execute each step and invoke `onStepDone` callback for step metrics recording.
  - If a step fails, evaluate `step.OnFailure`:
    - `OnFailureStop`: stop current iteration, return `iterationFailed=true`, proceed to next iteration.
    - `OnFailureAbortVU`: terminate VU execution entirely (return abort signal).
    - `OnFailureContinue`: continue executing subsequent steps in the iteration.
  - Apply step `think_time` between steps when configured.
- [x] Write unit tests in `internal/scenario/executor_test.go` covering step sequences, extraction chaining across steps, and all failure policies.

### 5. Closed-Model Scenario Scheduler Integration (`internal/scheduler`)
- [x] In `internal/plan/plan.go`, extend `BuildPlan` to compile `ScenarioConfig` into `CompiledScenario` with preflight validation of all step URLs and methods against `AllowedHosts` and `AllowNonIdempotent`.
- [x] In `internal/scheduler/closed.go`, update `ClosedScheduler` to support scenario execution when `plan.Scenario != nil`.
- [x] In `runVU`, initialize `VUState` with VU ID and private `http.CookieJar` from `plan.JarManager.GetJar(workerID)`.
- [x] Execute iterations via `ScenarioExecutor.ExecuteIteration`, recording iteration counts (planned, started, completed, failed) and routing step results to worker step metric accumulators.
- [x] Respect iteration-level `load.think_time` between scenario iterations and per-step `think_time` between steps.
- [x] Handle graceful stop and context cancellation cleanly across active scenario iterations.
- [x] Write tests in `internal/scheduler/closed_test.go` validating closed-model scenario execution, VU concurrency, and clean shutdown.

### 6. Step-Level & Scenario-Level Metrics Aggregation (`internal/metrics`)
- [x] In `internal/metrics/worker.go`, add `StepWorkers map[string]*WorkerMetrics` to `WorkerMetrics` to accumulate per-step request counts, outcomes, status codes, and latency histograms.
- [x] In `internal/metrics/aggregate.go`, add `Steps map[string]*AggregatedMetrics` and `ScenarioIterations ScenarioIterationCounts` to `AggregatedMetrics`.
- [x] Implement `MergeStepWorkers(workers []*WorkerMetrics, duration time.Duration) (map[string]*AggregatedMetrics, error)` to produce reconciled step-level aggregate summaries.
- [x] Ensure root metrics accurately reconcile with the sum of all step metrics across all VUs.
- [x] Write tests in `internal/metrics/metrics_test.go` verifying step metrics accumulation, histogram accuracy, and root-to-step reconciliation.

### 7. Step-Level Threshold Evaluation (`internal/threshold`)
- [x] In `internal/threshold/parser.go`, update `ParseThreshold` to support step metric keys with the syntax `step.<step_name>.<metric_name>` (e.g. `step.login.p95`, `step.checkout.http_error_rate`, `step.get_items.completed_rps`).
- [x] Parse and record `StepName string` on `Threshold` struct when prefix `step.` is present.
- [x] In `internal/threshold/evaluator.go`, update `Evaluate` to evaluate root thresholds against root metrics snapshot and step-specific thresholds against the corresponding step's `MetricsSnapshot`.
- [x] If a threshold targets a non-existent step name, return a clear configuration validation error.
- [x] Write unit tests in `internal/threshold/threshold_test.go` for step threshold parsing, validation, and evaluation.

### 8. Terminal & JSON Reporting for Scenarios (`internal/report`)
- [x] In `internal/report/types.go`, add `Scenario *ScenarioReport` to `Report` struct containing scenario name, iteration summary, and `Steps []StepReport`.
- [x] Define `StepReport` containing step name, request counts, outcomes, status codes, latency summary, and throughput.
- [x] In `internal/report/builder.go`, populate `rep.Scenario` when scenario metrics are present.
- [x] In `internal/report/terminal.go`, add a formatted **Scenario Steps** section displaying step name, request count, success rate, p50, p95, p99, and error count.
- [x] Ensure all extracted variables, tokens, and cookies are excluded from logs, terminal output, and JSON reports.
- [x] Write tests in `internal/report/report_test.go` verifying scenario JSON schema compliance and terminal report formatting.

### 9. Deterministic Test Target Extensions (`internal/testtarget`)
- [x] In `internal/testtarget/handler.go`, add `/auth/login` endpoint that accepts POST JSON credentials (`{"username":"...","password":"..."}`), sets a session cookie, and returns a JSON payload with `{"token":"...","session_id":"...","user_id":"..."}`.
- [x] Add `/api/items` endpoint that validates `Authorization: Bearer <token>` or session cookie and returns a JSON array of items.
- [x] Add `/api/logout` endpoint that invalidates session tokens and cookies.
- [x] Add `/scenario/fail-step` endpoint supporting configurable failure modes (`?status=500`, `?status=401`) for testing `on_failure` policies.
- [x] Add `/scenario/dynamic` endpoint returning JSON payloads with variable keys and regex patterns for extraction verification.
- [x] Write tests in `internal/testtarget/server_test.go` verifying scenario endpoints.

### 10. CLI, Configuration Examples, and Documentation
- [x] Update `internal/cli/run.go` and `validate.go` to support scenario execution and validation seamlessly.
- [x] Create `examples/multi-step-scenario.yaml` demonstrating a 3-step scenario: Login -> Fetch Items -> Logout with JSONPath extraction and variable substitution.
- [x] Update `README.md` with scenario configuration syntax, extraction rules, variable substitution syntax, step failure policies, and step-level thresholds.

## Test checklist

### Unit Tests
- [x] `internal/config`: Test parsing and validation of valid scenario YAML, duplicate step names, invalid HTTP methods, missing URLs, invalid extraction sources, invalid regex syntax, invalid `on_failure` values, and mutual exclusivity between `request` and `scenario`.
- [x] `internal/scenario/extract_test.go`: Test JSONPath extraction (flat, nested, array indexing, missing keys, type conversions), header extraction (case-insensitive, missing header), cookie extraction (existing/missing), regex extraction (matching group, no match, malformed regex).
- [x] `internal/scenario/substitute_test.go`: Test substitution in URL path, query params, headers, JSON body, escaping `$${VAR}`, missing variables error handling.
- [x] `internal/scenario/executor_test.go`: Test sequential step execution, state variable chaining across steps, think time enforcement, and `on_failure` policies (`stop`, `abort_vu`, `continue`).
- [x] `internal/scenario/isolation_test.go`: Concurrency test with multiple VUs verifying that VU A's variables and cookies never leak to VU B.
- [x] `internal/metrics`: Test worker-local step metric accumulation, aggregation, histogram merging, and exact reconciliation with root metrics.
- [x] `internal/threshold`: Test parsing and evaluation of step thresholds (`step.login.p95: "<= 100ms"`, `step.items.http_error_rate: "<= 1%"`), unknown step rejection, and pass/fail reporting.
- [x] `internal/report`: Test scenario report serialization, JSON schema validation, and terminal formatting.

### Integration Tests with `internal/testtarget`
- [x] **Multi-step workflow test:** 3-step workflow (`POST /auth/login` -> extract token/cookie -> `GET /api/items` with Bearer token & cookie -> `POST /api/logout`). Verify all steps succeed and tokens/cookies propagate.
- [x] **Cross-VU isolation test:** 10 concurrent VUs executing multi-step login flows simultaneously, verifying each VU receives and maintains unique session tokens and cookies without cross-talk.
- [x] **Step failure policy test (`on_failure: stop`):** Step 1 succeeds, Step 2 fails with 500, Step 3 is skipped; VU completes iteration and begins next iteration.
- [x] **Step failure policy test (`on_failure: continue`):** Step 1 succeeds, Step 2 fails, Step 3 executes.
- [x] **Step failure policy test (`on_failure: abort_vu`):** Step 1 fails, VU terminates immediately and executes no further iterations.
- [x] **Extraction error handling test:** Step returns JSON missing the expected key; step fails cleanly with extraction error and obeys `on_failure` policy.
- [x] **Step threshold pass/fail test:** Configure `step.login.p95: "<= 50ms"` and `step.items.p95: "<= 1ms"` (failing), verify CLI exits with exit code 1 and identifies the failing step threshold.

### Repository Verification
- [x] `gofmt -l .` reports zero files.
- [x] `go vet ./...` reports zero issues.
- [x] `go test -count=1 ./...` passes cleanly across all packages.
- [x] `go build ./...` builds cleanly without warnings.
- [x] `git diff --check` passes.

## Safety and failure behavior

- **Safety Preflight for All Steps (§12):** All step URLs and HTTP methods are resolved and validated during preflight against `safety.allowed_hosts` and `safety.allow_non_idempotent`. If any step targets a disallowed host or uses a non-idempotent method without permission, execution is refused before traffic starts (exit code 4).
- **Redaction of Extracted Variables and Cookies (§11):** All extracted variables, session tokens, authorization headers, and cookie values are treated as sensitive credentials and scrubbed from errors, terminal logs, fingerprints, and JSON reports.
- **Extraction Error Safety:** Missing JSONPath keys, absent headers, or regex mismatches do not panic or corrupt VU state; they record a classified error and trigger the configured `on_failure` policy.
- **Resource Bounds (§9):** Step-level histograms use bounded memory per step per worker ($O(\text{workers} \times \text{steps})$). Scenario steps are bounded at config validation time (max 50 steps).
- **Graceful Shutdown (§7):** When test duration expires or cancellation is signaled, active scenario steps finish within `graceful_stop` before connections are aborted.

## Acceptance gates

1. **Deterministic Multi-Step Execution:** A closed-model test with $\ge 2$ steps executes steps in strict sequence for each VU, passing extracted tokens and cookies to subsequent steps.
2. **Strict VU Isolation:** In a concurrent test with multiple VUs, extracted variables and session cookies never leak across VU boundaries.
3. **Explicit Failure Policy Enforcement:** Tests verify that `on_failure: stop` terminates only the current iteration, `on_failure: abort_vu` terminates the VU, and `on_failure: continue` proceeds to the next step.
4. **Step-Level Metrics and Thresholds:** Step-specific latency percentiles, request counts, and outcomes are reported in terminal and JSON outputs; `step.<name>.<metric>` thresholds evaluate accurately and return exit code 1 on violation.
5. **Full Repository Verification:** All unit, integration, and contract tests pass with `go test -count=1 ./...`, `go vet ./...`, `gofmt -l .`, and `go build ./...`.

## Explicit non-goals

- JavaScript or dynamic script execution runtimes for scenario logic.
- Open-model arrival-rate multi-step pipelines (scenarios are strictly closed-model per canonical plan §2, §11).
- Unbounded response payload retention or recording full request/response bodies.
- Distributed scenario execution across multiple machines.
- Non-HTTP protocols (WebSocket, gRPC).

## Open questions

- *None.* All scenario semantics, extraction sources, failure policies, isolation contracts, and threshold formats are aligned with `docs/DAEGSA_Implementation_Plan.md` and repository standards.

## Handoff

- **For Plan Implementer:** Follow the checklist items in order. Implement domain models in `internal/scenario`, extend `internal/config`, `internal/plan`, `internal/metrics`, `internal/threshold`, `internal/scheduler`, and `internal/report`, add test endpoints to `internal/testtarget`, and ensure all tests pass.
- **For Plan Tester:** Independently verify per-VU state isolation across concurrent workers, extraction error handling, all three `on_failure` policies, step-level threshold failures, and JSON report schema compatibility.

## Implementation handoff

### Changed Files
- `internal/config/types.go` — Added `ScenarioConfig`, `StepConfig`, `ExtractRuleConfig`, failure policy constants, and `Scenario` field on `Config`.
- `internal/config/validate.go` — Added mutual exclusivity check (`request` vs `scenario`), model validation (`load.model: closed`), and step validation (names, URLs, methods, timeouts, status codes, think times, failure policies, extraction rules).
- `internal/config/fingerprint.go` — Added scenario sanitization and header/credential redaction in config fingerprint calculation.
- `internal/config/validate_test.go` — Added comprehensive validation test cases in `TestValidateConfig_ScenarioValidation`.
- `internal/scenario/types.go` — Defined `VUState`, `StepResult`, `ErrAbortVU`, and scenario type aliases.
- `internal/scenario/extract.go` — Implemented response extraction engine for JSON/JSONPath, headers, cookies, and regex capture groups.
- `internal/scenario/substitute.go` — Implemented dynamic variable substitution (`${var}`) with `$${LITERAL}` escaping.
- `internal/scenario/executor.go` — Implemented `ScenarioExecutor`, `ExecuteStep`, and `ExecuteIteration` with isolated cookie jars, body draining, and failure policies.
- `internal/scenario/extract_test.go` — Added unit tests for all extraction sources, error modes, and edge cases.
- `internal/scenario/substitute_test.go` — Added unit tests for variable substitution in URLs, headers, bodies, and escaping.
- `internal/scenario/executor_test.go` — Added tests for step sequences, extraction chaining, and `stop`/`abort_vu`/`continue` failure policies.
- `internal/scenario/isolation_test.go` — Added concurrency test with 10 VUs verifying strict state and cookie isolation.
- `internal/plan/plan.go` — Added compiled scenario models (`CompiledScenario`, `CompiledStep`, `ExtractionRule`), scenario compilation in `BuildPlan`, and secret collection.
- `internal/plan/plan_test.go` — Added tests for scenario plan compilation.
- `internal/safety/preflight.go` — Added safety allowlist, destructive method, response body limit, and DNS preflight checks for all scenario steps.
- `internal/executor/executor.go` — Exposed `Transport() *http.Transport` method on `HTTPExecutor`.
- `internal/metrics/worker.go` — Added `StepWorkers map[string]*WorkerMetrics`, `ScenarioIterations`, and `GetOrCreateStepWorker`.
- `internal/metrics/aggregate.go` — Added `ScenarioIterations`, step merging in `MergeWorkers`, `MergeStepWorkers`, and `ToStepThresholdSnapshots`.
- `internal/metrics/aggregate_test.go` — Added tests for step metrics reconciliation and snapshot extraction.
- `internal/threshold/types.go` — Added `StepName` field to `Threshold`.
- `internal/threshold/parser.go` — Added support for `step.<step_name>.<metric>` threshold syntax and step name parsing.
- `internal/threshold/evaluator.go` — Added `EvaluateWithSteps` supporting step snapshot evaluation.
- `internal/threshold/parser_test.go` — Added tests for valid and invalid step threshold expressions.
- `internal/threshold/evaluator_test.go` — Added tests for multi-step threshold evaluation pass/fail behavior.
- `internal/scheduler/closed.go` — Integrated `ScenarioExecutor` in `ClosedScheduler.runVU` loop with think time, failure policy handling, and step metrics routing.
- `internal/scheduler/closed_test.go` — Added integration test for multi-step scenario closed execution and metric verification.
- `internal/report/types.go` — Added `ScenarioReport`, `StepReport`, and `Scenario` field on `Report`.
- `internal/report/builder.go` — Added scenario report construction in `BuildReport`.
- `internal/report/terminal.go` — Added `SCENARIO STEPS` table formatting in `FormatTerminalReport`.
- `internal/report/terminal_test.go` — Added terminal report formatting test for scenario steps.
- `internal/report/schema_test.go` — Added JSON report schema validation test for scenario structures.
- `internal/testtarget/handler.go` — Added `/auth/login`, `/api/items`, `/api/logout`, `/scenario/fail-step`, `/scenario/dynamic` endpoints.
- `internal/testtarget/server_test.go` — Added tests for testtarget scenario endpoints.
- `internal/cli/run.go` — Integrated `EvaluateWithSteps` for step threshold evaluation in CLI run command.
- `internal/cli/cli_test.go` — Added end-to-end CLI integration test `TestCLI_Run_MultiStepScenario`.
- `examples/multi-step-scenario.yaml` — Created complete example configuration.
- `README.md` — Documented multi-step scenario features, configuration syntax, and extraction rules.

### Behavior Implemented
- **Closed Workload Scenario Execution:** Sequential multi-step request pipelines within each virtual user loop.
- **Dynamic Response Extraction & Variable Substitution:** JSONPath dot/bracket notation, HTTP headers, cookies, and regex capture groups extracted into `VUState.Variables` and substituted across step URLs, headers, and request bodies.
- **Strict Per-VU Isolation:** Dedicated per-worker `VUState` memory and isolated `http.CookieJar` instances with zero cross-worker leakage.
- **Configurable Failure Policies:** `on_failure: stop` (skips remainder of current iteration), `on_failure: abort_vu` (terminates VU entirely), `on_failure: continue` (continues subsequent steps in iteration).
- **Step-Level Metrics and Thresholds:** Full latency histograms, request counts, and outcome tracking per step, reconciling with root totals and evaluated via `step.<step_name>.<metric>`.
- **Preflight Safety Enforcement:** Host allowlists and destructive method permissions validated across all scenario step URLs.
- **Terminal & JSON Reporting:** Formatted `SCENARIO STEPS` table in terminal output and structured `scenario` object in schema-compliant JSON reports.

### Commands Run and Results
- `gofmt -l .` — 0 unformatted files (clean).
- `go vet ./...` — 0 issues (passed).
- `go build ./...` — built successfully with 0 warnings.
- `go test -v -count=1 ./...` — all tests passed across all packages (`internal/config`, `internal/scenario`, `internal/plan`, `internal/safety`, `internal/executor`, `internal/metrics`, `internal/threshold`, `internal/scheduler`, `internal/report`, `internal/testtarget`, `internal/cli`).

### Known Limitations
- Scenarios are exclusively closed-model workloads (`load.model: closed`).
- Extraction rules support JSON/JSONPath, HTTP headers, cookies, and regex capture groups; arbitrary scripting is intentionally not supported.

### Remaining Unchecked Items
- Test checklist items under `## Test checklist` and `## Acceptance gates` remain unchecked for independent verification by the tester.