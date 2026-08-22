# DAEGSA Test Report
Result: PASS
Canonical phase: Phase 6 - Profiles and Rate-Limit Analysis
Commit candidate: current working tree

## Acceptance-gate evidence

### 1. Deterministic profile compilation
- **Constant and ramp segments:** Source segments compile into immutable constant-rate intervals (`internal/profile/compiler.go`). Linear ramp segments expand deterministically into `steps` constant slices with exact start/end rates (`internal/profile/compiler_test.go`).
- **Warmup, measured, cooldown segments:** Validation guarantees strict stage ordering (`warmup` -> `measured` -> `cooldown`) and requires at least one `measured` segment (`internal/config/validate.go:validateProfileSegments`).
- **Duration partitioning and inclusive interpolation:** Segment duration is partitioned down to integer nanoseconds with remainder distribution preserving total duration exactly (`internal/profile/compiler.go:32-38`).
- **Safety bounds and CLI override conflict handling:** Validated with ceiling checks for peak target RPS (`MaxAllowedRate`) and effective duration (`MaxAllowedDuration`) in `internal/safety/preflight.go`. Precedence checks reject `--rate` and `--duration` flags when segmented profiles are configured (`internal/config/precedence.go:62-79`).

### 2. Scheduler segment execution
- **Open scheduler segment transitions:** The open scheduler maintains a single stable worker pool (`make([]*metrics.WorkerMetrics, maxInFlight)`) and reusable HTTP executor/transport across all segment transitions without spawning ad-hoc worker goroutines (`internal/scheduler/open.go`).
- **Segment-local pacing reset without catch-up bursts:** Each segment computes target tick offsets locally from `startTime + StartOffset`. If execution lag exceeds an interval, target ticks advance to `actualTime + interval` rather than firing a catch-up burst (`internal/scheduler/open.go:241-245`).
- **Graceful drain after final segment:** Upon finishing the final segment, the dispatch channel is closed and graceful drain runs bounded by `plan.GracefulStop` before transitioning to `core.StateCompleted` (`internal/scheduler/open.go:135-165`).

### 3. Segment metrics reconciliation
- **Worker-local rotating segment metrics:** Workers record metrics into a single current-segment accumulator and flush into `SegmentCollector`, keeping memory complexity bounded at O(workers + segments) rather than O(workers * segments) (`internal/metrics/segment.go:35-79`).
- **Exact reconciliation:** Aggregated root metrics exactly equal segment sums for planned, scheduled, started, completed, canceled, dropped, status codes, outcomes, and 429 counts (`internal/scheduler/profile_test.go`). Measured summaries reconcile with measured-segment aggregates.
- **Threshold and default success evaluation:** Evaluated strictly against measured segments (`internal/cli/run.go:70-72`, `internal/report/terminal.go:243-255`), excluding warmup and cooldown traffic from pass/fail verdicts.

### 4. Rate-limit intelligence
- **429 counts and attribution per segment:** 429 HTTP responses are tracked per segment and aggregated into root metrics (`internal/scheduler/profile_test.go`).
- **First-throttle context:** `FirstThrottleOffsetNS` captures the planned nanosecond offset of the first 429 response relative to test start (`internal/scheduler/open.go:299-301`, `internal/report/phase6.go:71-76`).
- **Retry-After parsing:** Handles integer delta-seconds and RFC1123/RFC850 HTTP-Date timestamps into structured time fields (`internal/executor/ratelimit.go:43-56`).
- **Standard over legacy precedence:** `RateLimit-*` headers take strict precedence over `X-RateLimit-*` headers without falling back on invalid standard values (`internal/executor/ratelimit.go:58-67`, `internal/executor/ratelimit_phase6_test.go`).
- **Rate limit header consistency and parse validity:** Tracks observed count, parse error count, sample diversity, and agreement across workers with bounded memory (`internal/metrics/ratelimit.go`).
- **Control-character sanitization and secret redaction:** Control characters are stripped from headers/policy values (`internal/executor/ratelimit.go:120-132`); known secrets are sanitized across rate limit samples, headers, and consistency entries (`internal/report/phase6.go:87-107`, `internal/report/phase6_test.go`).
- **No-throttling caveat:** Terminal report outputs explicit warning banner when zero 429s are observed at tested rates (`internal/report/terminal.go:185-187`).

### 5. Generator calibration diagnostics
- **Calibration warnings:** Triggered when achieved start rate falls below 95% of target RPS, scheduler lag exceeds 50ms, or `max_in_flight` causes dropped requests (`internal/metrics/segment.go:165-193`, `internal/metrics/health_test.go`).
- **CPU saturation claims:** CPU saturation is never claimed when CPU metric sampling is unavailable; reports print `Max CPU: unavailable` (`internal/report/terminal.go:195-199`, `internal/metrics/health_test.go`).

### 6. Strict report schema v2
- **Schema v1 backward compatibility:** Schema v1 documents, parsing, and execution behaviors remain unchanged.
- **Schema v2 validation:** Both `testdata/schemas/v2/config.schema.json` and `testdata/schemas/v2/report.schema.json` validate JSON structures with strict rejection of unknown properties (`additionalProperties: false`) (`internal/config/schema_v2_test.go`).

### 7. Report comparison (`daegsa compare`)
- **Size-bounded loading:** Report loader enforces a 10MB maximum file size limit (`MaxReportFileBytes`), disallows unknown JSON fields, and rejects trailing data (`internal/compare/compare.go:37-70`, `internal/compare/load_test.go`).
- **Factual deltas:** Computes absolute and percentage changes for all latency percentiles (p50, p90, p95, p99, max), throughput, start rate, error rate, 429s, dropped counts, and matching segment percentiles (`internal/compare/compare.go:88-112`, `internal/compare/compare_test.go`).
- **Zero baseline handling:** Correctly marks percentages as unavailable when baseline is zero rather than generating NaNs or infinities.
- **Threshold transitions:** Distinguishes `fail-to-pass`, `pass-to-fail`, `added`, and `removed` threshold status changes (`internal/compare/compare.go:160-189`).
- **Comparability warnings:** Emits warnings when fingerprints, workload models, or profile segment definitions differ (`internal/compare/compare.go:76-85`).

## Commands and results

- `go build ./...`: **PASS** (exited code 0)
- `go vet ./...`: **PASS** (exited code 0)
- `gofmt -l .`: **PASS** (clean, no unformatted files)
- `git diff --check`: **PASS** (clean)
- `go test -v -count=1 ./...`: **PASS** (all packages passed)
- `daegsa compare baseline.json candidate.json`: **PASS** (verified via CLI test run)
- `daegsa validate --config examples/profile-rate-limit.yaml`: **PASS** (verified schema-v2 compilation)

## Added or changed tests

1. `internal/profile/compiler_test.go`:
   - `TestCompileBoundsAndInvalidInputs`: Verifies rejection of empty segments, invalid time units, exceeding `MaxSourceSegments` (64) / `MaxCompiledSegments` (256), non-positive rates, and zero durations.
   - `TestCompileMultiSegmentOrderingAndOffsets`: Verifies sequential offsets, duration sums, and `IncludedMeasured` flags.
2. `internal/config/profile_test.go`:
   - `TestProfileRejectsOrderingAndLegacyVersion`: Expanded table-driven negative tests covering duplicate names, missing measured stage, ramp steps < 2, conflicting rate/duration with segments, closed model with segments, and v2 without segments.
3. `internal/config/precedence_test.go`:
   - `TestApplyCLIOverrides_SegmentConflicts`: Verifies rejection of `--rate` and `--duration` flags on segmented profiles.
4. `internal/executor/ratelimit_phase6_test.go`:
   - `TestRetryAfter_DeltaSecondsAndHTTPDate`: Verifies parsing of integer delta seconds and RFC1123 HTTP dates.
   - `TestRateLimitReset_EpochAndDeltaAndDate`: Verifies reset header formats (delta seconds, epoch timestamp, HTTP date).
   - `TestRateLimitPrecedence_StandardWinsWhenValid`: Verifies `RateLimit-*` precedence over `X-RateLimit-*`.
   - `TestExtractRateLimitInfo_NilAndEmpty`: Verifies nil/empty header safety.
5. `internal/metrics/health_test.go`:
   - `TestBuildCalibration_Diagnostics`: Verifies calibration reliability and warning generation for low start rate (<95%), scheduler lag (>50ms), and dropped requests.
   - `TestGeneratorHealth_CPUAvailableFlag`: Verifies `CPUAvailable` state transitions.
6. `internal/compare/compare_test.go`:
   - `TestCompareDetailedDeltasAndThresholdTransitions`: Verifies delta calculation, percentage calculation, threshold status transitions (`fail-to-pass`, `removed`, `added`), and formatted string output.
7. `internal/compare/load_test.go`:
   - `TestLoadReportRejectsMalformedUnsupportedAndIncomplete`: Verifies rejection of malformed JSON, trailing data, unknown fields, and missing workload models.
   - `TestLoadReportRejectsExceededSize`: Verifies rejection of files exceeding 10MB.
   - `TestLoadReport_ValidReports`: Verifies loading of schema v1 and v2 reports.
8. `internal/report/phase6_test.go`:
   - `TestReportRateLimitRedaction`: Verifies redaction of known secrets in rate limit observations.
   - `TestTerminalReport_Phase6Sections`: Verifies terminal report sections for profile segments, no-throttling caveat, and unavailable CPU reporting.
9. `internal/scheduler/profile_test.go`:
   - `TestOpenScheduler_Segment429AttributionAndFirstThrottle`: Verifies per-segment 429 count attribution and `FirstThrottleOffsetNS` recording under live HTTP traffic.

## Defects

None.

## Generator/resource observations

- Memory complexity remains strictly bounded at O(workers + segments); worker metric accumulators rotate per segment rather than maintaining full NxM matrix.
- Rate limit header samples, retry-after samples, and error samples are strictly capped at 10 items.
- Report comparator enforces a 10MB ceiling with stream decoding to prevent memory exhaustion on corrupted files.

## Unverified limitations

- The race detector (`go test -race ./...`) could not be run in this Windows environment due to the absence of a C compiler (GCC) required for CGO (`cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in %PATH%`).

## Commit recommendation

Recommend commit for Phase 6: Profiles and Rate-Limit Analysis.