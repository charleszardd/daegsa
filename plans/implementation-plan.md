# DAEGSA Execution Plan
Status: COMMITTED
Canonical phase: Phase 4 - Thresholds and CI Contract
Tranche: entire phase

## Objective

Build and integrate the threshold subsystem (`internal/threshold`), lock the canonical CLI exit-code contract, integrate threshold evaluation into terminal and JSON v1 reports, enforce incomplete-result semantics, produce single-line CI failure summaries on `stderr`, and provide schema validation golden tests and CI workflow examples.

---

## Requirements Traceability

- **`docs/DAEGSA_Implementation_Plan.md` §3 (System Architecture)**: Define `internal/threshold` package for pass/fail threshold evaluation, separate from metrics accumulators, executors, and reporters.
- **`docs/DAEGSA_Implementation_Plan.md` §6 (Configuration Contract)**: Strict validation of `thresholds` mapping; canonical metric names, operators, and units in YAML manifests.
- **`docs/DAEGSA_Implementation_Plan.md` §7 (Execution Semantics)**: Step 10 of test lifecycle: merge metrics, evaluate thresholds against measured phase, generate reports, and return canonical exit code.
- **`docs/DAEGSA_Implementation_Plan.md` §9 (Outcome and Metrics Model)**: Evaluate thresholds against `metrics.AggregatedMetrics` (counts, outcomes, latencies, throughput, error rates, in-flight concurrency).
- **`docs/DAEGSA_Implementation_Plan.md` §10 (Thresholds and Exit Codes)**:
  - Syntax requiring explicit operator (`<`, `<=`, `>`, `>=`, `==`, `!=`) and unit (`%`, `ms`, `s`, `µs`, `req/s`, integer count).
  - Evaluation against measured phase excluding warm-up.
  - Stable process exit codes:
    - `0`: Test completed and all thresholds passed (or all requests succeeded if no thresholds defined).
    - `1`: Test completed but one or more thresholds failed (or errors occurred when no thresholds defined).
    - `2`: CLI usage or configuration validation failed.
    - `3`: Runtime/tool failure prevented a valid test result (or forced cancellation / incomplete run).
    - `4`: Safety policy refused execution.
- **`docs/DAEGSA_Implementation_Plan.md` §13 (Reports and Reproducibility)**:
  - Terminal report includes structured Threshold Evaluation table (`Metric`, `Target`, `Observed`, `Status`).
  - JSON report v1 schema populates `thresholds: []ThresholdResult` (`expression`, `target`, `observed`, `passed`) and `incomplete: bool`.
  - Incomplete-result marker set when cancellation or runtime failure invalidates interpretation.
- **`docs/DAEGSA_Implementation_Plan.md` §14 (CI Usability)**: Single-line failure summary printed to `stderr` for CI runners (GitHub Actions, GitLab CI, Azure DevOps).
- **`docs/DAEGSA_Implementation_Plan.md` §15 (Phase 4 - Thresholds and CI Contract)**: Implementation deliverables and exit gate: "CI can reliably distinguish regression, invalid configuration, runtime failure, and safety refusal."
- **`docs/DAEGSA_Implementation_Plan.md` §16 (Version Roadmap)**: v0.3.0 milestone scope: Thresholds, stable JSON schema, CI exit-code contract.
- **`docs/DAEGSA_Implementation_Plan.md` §19 (Acceptance Criteria for v1)**: Threshold and runtime failures produce stable, distinct exit codes; JSON reports are versioned and reproducible for CI.

---

## Current Repository Findings

1. **`internal/core/exitcode.go`**:
   - Canonical exit codes `ExitCodeSuccess` (0), `ExitCodeThresholdFailure` (1), `ExitCodeValidationFailure` (2), `ExitCodeRuntimeFailure` (3), and `ExitCodeSafetyRefusal` (4) are defined with string identifiers and descriptions.
   - `internal/cli/exit.go` contains `DetermineExitCode(err error)` mapping error types to exit codes.
2. **`internal/config/types.go` & `internal/config/validate.go`**:
   - `Config.Thresholds` is typed as `map[string]string`.
   - `validateThresholds` currently performs only superficial prefix checks. It must be updated to invoke `threshold.ParseThreshold` for strict syntactic and metric-name validation during configuration parsing.
3. **`internal/plan/plan.go`**:
   - `Plan` struct currently lacks a field for parsed thresholds or raw threshold definitions. It must store parsed thresholds (`[]*threshold.Threshold` or `map[string]string`) deeply cloned from `Config`.
4. **`internal/metrics/aggregate.go` & `internal/metrics/histogram.go`**:
   - `AggregatedMetrics` calculates `RequestCounts`, `Outcomes`, `StatusCodes`, `Latency` (`AllCompleted`, `ExpectedSuccess`), `RateLimits`, `AchievedStartRPS`, `CompletedThroughput`, `ErrorRate`, and `RateLimitedRate`.
   - `HDRHistogram` supports `ValueAtQuantile(q float64)` which enables `p99.9` percentile calculation.
5. **`internal/report/types.go` & `internal/report/builder.go`**:
   - `Report.Thresholds` is typed as `[]ThresholdResult` with `Expression`, `Target`, `Observed`, and `Passed`.
   - `BuildReport` initializes `rep.Thresholds` to empty slice. It needs to accept evaluated threshold results.
6. **`internal/report/terminal.go`**:
   - Formats test summary, requests/throughput, outcomes, status codes, latency, rate limiting, and generator health, but does not yet render a dedicated `THRESHOLD EVALUATION` table section.
7. **`internal/cli/run.go`**:
   - Uses hard-coded outcome checks (`successCount < rep.RequestCounts.Completed`) when determining exit codes instead of invoking the threshold evaluator.
   - Does not print single-line failure summaries to `stderr`.
8. **`testdata/schemas/v1/report.schema.json`**:
   - Schema defines strict constraints for `thresholds`: array of objects with `required: ["expression", "target", "observed", "passed"]` and `additionalProperties: false`.
   - Schema defines `incomplete: { "type": "boolean" }`.

---

## Files Expected to Change

### New Files
- `internal/threshold/types.go`: Canonical metric names, comparison operators, threshold data structures, and evaluation results.
- `internal/threshold/parser.go`: Parser for threshold expressions (metric names, operators, values, and units).
- `internal/threshold/parser_test.go`: Table-driven tests for threshold expression parsing, valid syntax, and invalid error cases.
- `internal/threshold/evaluator.go`: Evaluation engine executing parsed thresholds against `metrics.AggregatedMetrics` and `plan.Plan`.
- `internal/threshold/evaluator_test.go`: Table-driven tests for threshold evaluation, pass/fail rules, floating point tolerances, and zero-count edge cases.
- `examples/ci/github-actions.yml`: Example GitHub Actions CI workflow demonstrating DAEGSA integration and artifact export.
- `examples/ci/gitlab-ci.yml`: Example GitLab CI pipeline definition.
- `examples/open-api-capacity.yaml`: Example open-model load test manifest with full thresholds.
- `examples/closed-api-smoke.yaml`: Example closed-model smoke test manifest with thresholds.

### Modified Files
- `internal/config/validate.go`: Integrate `threshold.ParseThreshold` into `validateThresholds` for full validation.
- `internal/plan/plan.go`: Add `Thresholds []*threshold.Threshold` to `Plan` and populate during `BuildPlan`.
- `internal/plan/plan_test.go`: Test immutability and preservation of thresholds in `Plan`.
- `internal/report/builder.go`: Accept `[]report.ThresholdResult` in `BuildReport` or attach via builder method.
- `internal/report/terminal.go`: Add ANSI formatted `THRESHOLD EVALUATION` table section and reflect threshold failure in summary banner.
- `internal/report/terminal_test.go`: Verify terminal rendering with passing, failing, and empty thresholds.
- `internal/report/schema_test.go`: Add golden tests validating JSON reports with threshold results against `testdata/schemas/v1/report.schema.json`.
- `internal/cli/run.go`: Integrate threshold evaluation, incomplete-result handling, and exit-code calculation.
- `internal/cli/root.go`: Print single-line CI failure summary to `stderr` on exit failure.
- `internal/cli/cli_test.go`: E2E tests for exit codes 0, 1, 2, 3, 4, threshold evaluation, incomplete results, and stderr summary output.

---

## Implementation Checklist

### 1. Threshold Types and Parser (`internal/threshold`)
- [x] Create `internal/threshold/types.go`:
  - [x] Define canonical metric name constants:
    - Percentages / rates: `MetricHTTPErrorRate` (`"http_error_rate"`), `MetricRateLimitedRate` (`"rate_limited_rate"`), `MetricDroppedRate` (`"dropped_rate"`).
    - Latency quantiles & stats: `MetricP50` (`"p50"`), `MetricP90` (`"p90"`), `MetricP95` (`"p95"`), `MetricP99` (`"p99"`), `MetricP999` (`"p99.9"`), `MetricMinLatency` (`"min_latency"`), `MetricMaxLatency` (`"max_latency"`), `MetricMeanLatency` (`"mean_latency"`).
    - Throughput / rates: `MetricCompletedRPS` (`"completed_rps"`), `MetricStartedRPS` (`"started_rps"`), `MetricTargetRPS` (`"target_rps"`).
    - Request counts: `MetricDroppedRequests` (`"dropped_requests"`), `MetricFailedRequests` (`"failed_requests"`), `MetricCompletedRequests` (`"completed_requests"`), `MetricCanceledRequests` (`"canceled_requests"`).
    - Concurrency: `MetricMaxInFlight` (`"max_in_flight"`).
  - [x] Define operator enum/constants: `OpLT` (`"<"`), `OpLTE` (`"<="`), `OpGT` (`">"`), `OpGTE` (`">="`), `OpEQ` (`"=="`), `OpNE` (`"!="`).
  - [x] Define metric category enum: `MetricCategoryRate` (percentage), `MetricCategoryLatency` (duration), `MetricCategoryThroughput` (rate), `MetricCategoryCount` (integer count), `MetricCategoryConcurrency` (integer).
  - [x] Define `Threshold` struct: `MetricName string`, `Category MetricCategory`, `Operator string`, `TargetRaw string`, `TargetValue float64` (or duration/int), `Unit string`, `RawExpression string`.
  - [x] Define `Result` struct: `Threshold *Threshold`, `MetricName string`, `Operator string`, `TargetFormatted string`, `ObservedValue float64`, `ObservedFormatted string`, `Passed bool`.
- [x] Create `internal/threshold/parser.go`:
  - [x] Implement `ParseThreshold(metricName string, expr string) (*Threshold, error)`.
  - [x] Validate that `metricName` matches one of the canonical metric name constants.
  - [x] Extract operator prefix (`<=`, `>=`, `==`, `!=`, `<`, `>`). Reject expressions without valid operator prefix.
  - [x] Parse target value and unit based on metric category:
    - Rate/Percentage metrics: require `%` unit (e.g. `<= 1%`, `< 0.5%`), parse as float percentage [0, 100].
    - Latency metrics: support units `ms`, `s`, `µs`, `us` (e.g. `<= 500ms`, `< 1s`, `<= 250µs`), parse into duration/millisecond float.
    - Throughput metrics: support unit `req/s`, `rps`, or raw numeric value (e.g. `>= 90`, `>= 300 req/s`).
    - Count and Concurrency metrics: parse as non-negative integer (e.g. `== 0`, `<= 5`, `<= 100`).
  - [x] Reject unit mismatches (e.g. duration unit on rate metric or `%` on latency metric).
  - [x] Implement `ParseThresholds(thresholdMap map[string]string) ([]*Threshold, error)`.

### 2. Threshold Evaluation Engine (`internal/threshold`)
- [x] Create `internal/threshold/evaluator.go`:
  - [x] Implement `Evaluator` / `Evaluate(thresholds []*Threshold, snap MetricsSnapshot, evalCtx EvaluationContext) ([]Result, bool, error)`.
  - [x] Extract observed value for each metric from `MetricsSnapshot` and `EvaluationContext`:
    - `http_error_rate`: `snap.ErrorRate`.
    - `rate_limited_rate`: `snap.RateLimitedRate`.
    - `dropped_rate`: `(float64(snap.RequestCounts.Dropped) / float64(snap.RequestCounts.Planned)) * 100.0` (guarded against division by zero: if `Planned == 0`, fallback to `Started + Dropped`).
    - `p50`, `p90`, `p95`, `p99`: `snap.Latency.AllCompleted.P50MS`, etc.
    - `p99.9`: calculated via histogram quantile `99.9` in milliseconds (or from latency metrics).
    - `min_latency`, `max_latency`, `mean_latency`: from `snap.Latency.AllCompleted`.
    - `completed_rps`: `snap.CompletedThroughput`.
    - `started_rps`: `snap.AchievedStartRPS`.
    - `target_rps`: `evalCtx.TargetRPS`.
    - `dropped_requests`: `snap.RequestCounts.Dropped`.
    - `failed_requests`: `snap.RequestCounts.Completed - snap.Outcomes[core.OutcomeSuccess]`.
    - `completed_requests`: `snap.RequestCounts.Completed`.
    - `canceled_requests`: `snap.RequestCounts.Canceled`.
    - `max_in_flight`: `evalCtx.MaxInFlight`.
  - [x] Evaluate comparison operator with floating point epsilon (`1e-9`) tolerance for equality/inequality checks.
  - [x] Format `ObservedFormatted` consistently matching the target unit (e.g. `"1.25%"`, `"45.20ms"`, `"120.00 req/s"`, `"0"`).
  - [x] Return slice of `Result` and overall pass boolean (`true` if and only if all evaluated thresholds passed).
  - [x] Implement conversion function `ToReportResults(results []Result) []ReportResult`.

### 3. Config and Plan Integration
- [x] Update `internal/config/validate.go`:
  - [x] In `validateThresholds`, use `threshold.ParseThreshold` to strictly validate all threshold entries during config parsing.
- [x] Update `internal/plan/plan.go`:
  - [x] Add `Thresholds []*threshold.Threshold` field to `Plan`.
  - [x] In `BuildPlan`, parse and deeply clone `cfg.Thresholds` into `p.Thresholds`.
- [x] Update `internal/plan/plan_test.go`:
  - [x] Add test cases verifying `p.Thresholds` immutability and deep copy.

### 4. Report Integration and Formatting
- [x] Update `internal/report/builder.go`:
  - [x] Update `BuildReport` signature to populate `rep.Thresholds`.
- [x] Update `internal/report/terminal.go`:
  - [x] Add ANSI-formatted `THRESHOLD EVALUATION` table section:
    - Header: `Metric`, `Target`, `Observed`, `Status`.
    - Rows: clean aligned output with `PASS` / `FAIL` status tags.
    - If no thresholds are configured, omit table.
  - [x] Update test result banner:
    - If `rep.Incomplete`: display `INCOMPLETE (run aborted or timed out)`.
    - Else if threshold failures occurred: display `FAIL (thresholds failed)`.
    - Else if no thresholds configured but request errors occurred: display `FAIL (unexpected status codes or errors detected)`.
    - Else: display `PASS`.

### 5. CLI Exit-Code Contract and CI Usability
- [x] Update `internal/cli/run.go`:
  - [x] Execute threshold evaluation on `agg` using `p.Thresholds`.
  - [x] Attach converted `[]report.ThresholdResult` to `report.Report`.
  - [x] If `incomplete` is true (run error or context cancellation), flag `rep.Incomplete = true`, set exit code to `ExitCodeRuntimeFailure` (3).
  - [x] If thresholds are configured:
    - If any threshold fails, return `CLIExitError{Code: core.ExitCodeThresholdFailure, Err: ...}` (exit code 1) detailing the failing expressions.
    - If all thresholds pass, return `nil` (exit code 0).
  - [x] If no thresholds are configured:
    - If any non-success outcome occurred (or 429 when not expected), return exit code 1.
    - If all requests succeeded, return exit code 0.
- [x] Update `internal/cli/root.go` and `internal/cli/exit.go`:
  - [x] Format single-line failure summaries on `stderr` when `err != nil`:
    - Example: `daegsa: threshold failure: p95 (650.00ms) failed target <= 500ms; http_error_rate (2.50%) failed target <= 1%`.
    - Example: `daegsa: runtime failure: test execution incomplete (aborted)`.
    - Example: `daegsa: safety refusal: target host 'evil.com' not in allowed_hosts`.
    - Example: `daegsa: validation failure: invalid threshold 'p95': missing operator`.

### 6. Schema Golden Tests and CI Fixtures
- [x] Create/Update `internal/report/schema_test.go`:
  - [x] Test JSON reports with passed thresholds against `testdata/schemas/v1/report.schema.json`.
  - [x] Test JSON reports with failed thresholds against schema.
  - [x] Test JSON reports with `incomplete: true` against schema.
  - [x] Test JSON reports with empty thresholds against schema.
- [x] Create `examples/ci/github-actions.yml`:
  - [x] GitHub Actions workflow executing `daegsa run`, archiving `report.json`, and handling exit codes.
- [x] Create `examples/ci/gitlab-ci.yml`:
  - [x] GitLab CI pipeline configuration.
- [x] Create `examples/open-api-capacity.yaml` and `examples/closed-api-smoke.yaml`:
  - [x] Ready-to-use sample test configurations with canonical thresholds.

---

## Test Checklist

### Unit Tests
- [x] `internal/threshold/parser_test.go`:
  - [x] Valid metric names: test all canonical metric names across categories (rate, latency, throughput, count, concurrency).
  - [x] Valid operators: test `<`, `<=`, `>`, `>=`, `==`, `!=`.
  - [x] Valid units: `%`, `ms`, `s`, `µs`, `us`, `req/s`, `rps`, integer counts.
  - [x] Invalid syntax: missing operator, missing unit, negative counts, unknown metric name, malformed expression.
  - [x] Unit incompatibility: duration unit on error rate, `%` unit on latency.
- [x] `internal/threshold/evaluator_test.go`:
  - [x] Passing evaluations for all metric types.
  - [x] Failing evaluations for all metric types.
  - [x] Boundary conditions: exact equality (`==`, `<=`, `>=`).
  - [x] Floating-point precision tests (e.g. `0.999999999% <= 1%`).
  - [x] Zero completed requests / empty metrics edge cases (avoid panics and division-by-zero).
  - [x] P99.9 percentile latency evaluation.
- [x] `internal/report/terminal_test.go`:
  - [x] Verify rendering of `THRESHOLD EVALUATION` table with mixed `PASS` / `FAIL` rows.
  - [x] Verify terminal rendering when thresholds are empty.
  - [x] Verify terminal rendering when run is `incomplete`.

### JSON Schema & Golden Tests
- [x] `internal/report/schema_test.go`:
  - [x] Validate full JSON report structure with threshold results against `testdata/schemas/v1/report.schema.json`.
  - [x] Validate incomplete JSON report (`incomplete: true`).
  - [x] Verify round-trip unmarshaling and field exactness.

### CLI End-to-End & Integration Tests
- [x] `internal/cli/cli_test.go`:
  - [x] **Exit Code 0**: Test against `testtarget` where all thresholds pass.
  - [x] **Exit Code 1**: Test against `testtarget` where latency or error rate threshold fails.
  - [x] **Exit Code 1**: Test with default error behavior when no thresholds configured.
  - [x] **Exit Code 2**: Test with invalid threshold syntax in CLI or config manifest.
  - [x] **Exit Code 3**: Test runtime cancellation / context cancel producing `incomplete: true` and exit code 3.
  - [x] **Exit Code 4**: Test safety refusal (disallowed host, unauthorized destructive method).
  - [x] **Single-Line Stderr Summary**: Verify concise, actionable error output on `stderr` across failure scenarios.
  - [x] **JSON Export**: Verify `--output-json` exports compliant JSON reports with populated threshold results.

---

## Safety and Failure Behavior

1. **Deterministic Exit Codes**:
   - The CLI strictly distinguishes between threshold failure (`ExitCodeThresholdFailure` = 1), configuration error (`ExitCodeValidationFailure` = 2), runtime crash/cancellation (`ExitCodeRuntimeFailure` = 3), and safety refusal (`ExitCodeSafetyRefusal` = 4).
2. **Incomplete Run Handling**:
   - If a test is canceled (SIGINT / context canceled) or fails during execution, `incomplete` is set to `true`.
   - Incomplete runs do not report false threshold passes; they produce exit code 3 (`ExitCodeRuntimeFailure`) and are clearly marked in terminal and JSON reports.
3. **Division by Zero Protection**:
   - All rate and throughput calculations explicitly check for zero planned, started, or completed requests, returning `0.0` rather than `NaN` or `Inf`.
4. **Secret Redaction**:
   - Error messages, threshold evaluation strings, single-line CI failure summaries, terminal output, and JSON reports must never contain sensitive headers, auth tokens, or credentials.
5. **Memory and Cardinality Bounds**:
   - Threshold evaluation operates in $O(T)$ where $T$ is the number of configured thresholds (typically $\le 20$), performing zero heap allocations in the hot path.

---

## Acceptance Gates

- [x] `go build ./...` compiles with 0 errors and 0 warnings.
- [x] `go vet ./...` passes with 0 diagnostics.
- [x] All unit, integration, and CLI tests pass: `go test -v ./...`.
- [x] JSON report serialization strictly validates against `testdata/schemas/v1/report.schema.json`.
- [x] Canonical exit codes (0, 1, 2, 3, 4) are deterministically verified in automated tests.
- [x] Single-line CI failure summary on `stderr` is verified.
- [x] `git diff --check` reports 0 whitespace errors or formatting issues.

---

## Explicit Non-Goals

- Multi-step scenarios and JSON variable extraction (deferred to Phase 7).
- Token pools and dynamic bearer authentication (deferred to Phase 5).
- Ramp, stress, spike, and soak profile segment definitions (deferred to Phase 6).
- Report comparison tool (`daegsa compare`) (deferred to Phase 6 / v0.7.0).
- JUnit XML output format (deferred to v0.7.0).
- Distributed load generation across nodes (deferred to v2.0.0).

---

## Open Questions

- None. All threshold metrics, units, operators, report schemas, and exit codes are fully specified in `docs/DAEGSA_Implementation_Plan.md` §6, §10, and §13.

---

## Handoff

### For the Plan Implementer:
1. Implement `internal/threshold/types.go` and `internal/threshold/parser.go`.
2. Implement `internal/threshold/evaluator.go`.
3. Wire `threshold.ParseThreshold` into `internal/config/validate.go` and `internal/plan/plan.go`.
4. Update `internal/report/builder.go` and `internal/report/terminal.go` to format threshold tables.
5. Update `internal/cli/run.go` and `internal/cli/root.go` for threshold evaluation, incomplete results, and single-line stderr summaries.
6. Add example CI manifests in `examples/ci/` and configs in `examples/`.

### For the Plan Tester:
1. Run `go test -v ./internal/threshold/...` to verify parser and evaluator test tables.
2. Run `go test -v ./internal/report/...` to verify JSON schema compliance and terminal formatting.
3. Run `go test -v ./internal/cli/...` to verify all 5 canonical exit codes and stderr single-line summaries.
4. Verify `go vet ./...` and `git diff --check`.

---

## Implementation handoff

### 1. Changed and New Files
- **New Files**:
  - `internal/core/redirect.go`: Redirect policy constants extracted to core Level 0 to maintain clean DAG architecture.
  - `internal/threshold/types.go`: Canonical metric names, categories, comparison operators, and result structures.
  - `internal/threshold/parser.go`: Strict syntactic and metric-name threshold parser with unit validation.
  - `internal/threshold/parser_test.go`: Comprehensive unit tests for valid expressions, invalid syntax, and deterministic sorting.
  - `internal/threshold/evaluator.go`: Evaluation engine with floating-point epsilon tolerance (`1e-9`) and zero-division protection.
  - `internal/threshold/evaluator_test.go`: Unit tests for passing, failing, boundary, epsilon tolerance, and nil/zero safety.
  - `examples/open-api-capacity.yaml`: Example open-model capacity load manifest with canonical thresholds.
  - `examples/closed-api-smoke.yaml`: Example closed-model smoke test configuration with thresholds.
  - `examples/ci/github-actions.yml`: Example GitHub Actions workflow executing validation, load tests, and archiving JSON reports.
  - `examples/ci/gitlab-ci.yml`: Example GitLab CI pipeline configuration.
- **Modified Files**:
  - `internal/config/types.go` & `internal/config/validate.go`: Aliased redirect constants to `core.RedirectPolicy*`, integrated `threshold.ParseThreshold` into `validateThresholds`.
  - `internal/config/schema_test.go`: Added automated validation tests for example manifests.
  - `internal/executor/executor.go` & `internal/executor/executor_test.go`: Decoupled `executor` from `config` to break dependency cycles.
  - `internal/metrics/aggregate.go`: Added `AllLatencyHist`, `SuccessLatencyHist`, and `ToThresholdSnapshot()`.
  - `internal/plan/plan.go` & `internal/plan/plan_test.go`: Added `Thresholds []*threshold.Threshold`, deep cloning during `BuildPlan`, `TargetRPS()`, `ToEvaluationContext()`, and immutability tests.
  - `internal/report/types.go` & `internal/report/builder.go`: Aliased `ThresholdResult = threshold.ReportResult`, updated `BuildReport` to populate `rep.Thresholds`.
  - `internal/report/terminal.go` & `internal/report/terminal_test.go`: Added ANSI `THRESHOLD EVALUATION` table section and updated status banners.
  - `internal/report/schema_test.go`: Added golden JSON schema v1 validation tests against `testdata/schemas/v1/report.schema.json`.
  - `internal/cli/run.go`: Integrated threshold evaluation, incomplete-run detection, exit code propagation, and JSON export.
  - `internal/cli/root.go` & `internal/cli/exit.go`: Formatted single-line actionable failure summaries on `stderr` (`daegsa: threshold failure: ...`, `daegsa: validation failure: ...`, `daegsa: runtime failure: ...`, `daegsa: safety refusal: ...`).
  - `internal/cli/cli_test.go`: E2E integration tests for exit codes 0, 1, 2, 3, 4, thresholds, cancellation, single-line summaries, and JSON reporting.
  - `plans/implementation-plan.md`: Updated checklist and status to `READY_FOR_TEST`.

### 2. Behavior Implemented
- Full threshold parsing and metric validation rejecting unit mismatches, negative counts, and invalid operators.
- Deterministic pass/fail evaluation against aggregated metrics and execution plans with zero division safety.
- Canonical CLI process exit codes:
  - `0`: All tests completed and all thresholds passed.
  - `1`: Threshold evaluation failed or unexpected HTTP/transport errors occurred.
  - `2`: Configuration or threshold syntax validation failed.
  - `3`: Runtime failure or cancellation marked test as incomplete.
  - `4`: Safety preflight policy refusal.
- ANSI terminal rendering with dedicated `THRESHOLD EVALUATION` section and status banners.
- Versioned JSON report schema v1 golden test validation.
- Single-line concise failure summaries printed to `stderr` for CI runners.

### 3. Commands Run and Results
- `go test -v -count=1 ./...`: PASS (All packages passed with 0 errors).
- `go vet ./...`: PASS (0 diagnostics).
- `go build ./...`: PASS (0 build errors).
- `git status`: Verified modified and untracked files without modifying git commit state.

### 4. Known Limitations
- Multi-step scenarios and JSON variable extraction are intentionally deferred to Phase 7.
- Dynamic token pools and token refresh are deferred to Phase 5.

### 5. Remaining Unchecked Items
- Tester-owned `## Test Checklist` and `## Acceptance Gates` sections remain unchecked for the independent tester to verify.
