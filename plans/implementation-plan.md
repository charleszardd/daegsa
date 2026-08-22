# DAEGSA Execution Plan
Status: COMMITTED
Canonical phase: Phase 6 - Profiles and Rate-Limit Analysis
Tranche: entire phase

## Objective

Add deterministic open-model load profiles, segment-aware bounded metrics and rate-limit analysis, conservative generator calibration, schema-v2 reports, and size-bounded v1/v2 report comparison while preserving v1 configuration behavior and Phase 0-5 contracts.

## Required work

- Introduce schema v2 profile segments for open workloads; preserve schema v1 unchanged.
- Validate unique ordered warmup/measured/cooldown segments, constant or ramp rates, exact duration partition, peak-rate and total-duration safety bounds, and CLI override conflicts.
- Compile immutable deterministic segments and execute them through one stable worker pool/shared transport with segment-local pacing and one final graceful drain.
- Attribute scheduled, dropped, completed, canceled, outcome, status, latency, and rate-limit observations to exactly one bounded segment; produce all-traffic and measured-only reconciled summaries.
- Exclude warm-up/cool-down from thresholds and default success evaluation while retaining their report data.
- Extend bounded rate-limit analysis with 429 counts, first-throttle context, Retry-After parsing, standard-over-legacy precedence, parse validity, consistency, sanitized samples, and the no-throttling caveat.
- Add calibration fields and warnings for achieved start rate below 95%, scheduler lag, and max-in-flight drops without unsupported CPU-saturation claims.
- Add strict report schema v2 while retaining v1 readers; implement size-limited `daegsa compare baseline.json candidate.json` with factual deltas and comparability warnings.
- Add deterministic compiler/scheduler/metrics/report/compare/CLI tests, examples, and documentation.

## Resource and compatibility invariants

- Memory remains O(workers + compiled segments), never O(workers * segments).
- All queues, samples, strings, and segment counts are bounded.
- No catch-up bursts and stable lane IDs across segment boundaries.
- Root metrics exactly equal segment sums; measured metrics exactly equal measured-segment sums.
- Existing schema-v1 files and legacy constant-rate behavior remain compatible.
- Existing safety, redaction, exit-code, and outcome contracts remain unchanged.
- Preserve unrelated dirty formatting changes; if a Phase 6 file overlaps, retain and disclose them.

## Exit gate

Profile compilation is deterministic; segment results reconcile with overall and measured reports; rate-limit/calibration output is bounded and factual; v1/v2 comparison is safe; all supported repository gates and independent tester review pass.

## Proposed commit

`feat(profile): add segmented load analysis and report comparison`
## Implementation handoff

Status: READY_FOR_TEST

### Implemented behavior

- Added schema-v2 open workload segments with strict constant/ramp validation, ordered warmup/measured/cooldown stages, deterministic inclusive ramp interpolation, exact nanosecond duration partitioning, bounded source/compiled counts, fingerprint inclusion, override conflict rejection, and peak target-RPS/total-duration safety enforcement.
- Added immutable compiled profiles to execution plans and sanitized dry-run summaries.
- Refactored the open scheduler to retain one stable worker pool and shared transport across segment boundaries, reset pacing at each boundary, avoid catch-up bursts, preserve stable lane IDs, and drain only after the final segment.
- Added O(workers + segments) rotating worker-local segment metrics, exact all-traffic and measured-only reconciliation, per-segment 429 attribution, first-throttle planned context, and measured-only threshold/default result evaluation.
- Added bounded rate-limit parse-validity and consistency evidence with standard-over-legacy precedence, delta/date Retry-After parsing, control-character sanitization, known-secret redaction, and the no-throttling caveat.
- Added conservative per-segment and measured calibration for low achieved start rate, scheduler lag, and max-in-flight drops; CPU saturation is not claimed when CPU sampling is unavailable.
- Added strict report schema v2 while retaining unchanged v1 schemas/output for legacy configurations.
- Added size-limited strict v1/v2 report loading and `daegsa compare baseline.json candidate.json` with factual latency/throughput/error/drop/429/segment deltas, threshold transitions, zero-baseline handling, and comparability warnings.
- Added focused compiler, validation, scheduler reconciliation, rate-limit, metrics, report, schema, comparison tests; a profile example; and user documentation.

### Principal changed files

- `internal/profile/*`
- `internal/config/{types.go,validate.go,precedence.go,fingerprint.go}`
- `internal/plan/{plan.go,print.go}` and `internal/safety/preflight.go`
- `internal/scheduler/open.go`
- `internal/metrics/{worker.go,aggregate.go,segment.go,ratelimit.go,health.go}`
- `internal/executor/ratelimit.go`
- `internal/report/{types.go,builder.go,phase6.go,terminal.go}`
- `internal/compare/*` and `internal/cli/compare.go`
- `testdata/schemas/v2/*`, `examples/profile-rate-limit.yaml`, and `README.md`

### Validation run

- `gofmt -l .`: PASS (no files reported)
- `go test -count=1 ./...`: PASS
- `go vet ./...`: PASS
- `go build ./...`: PASS
- `git diff --check`: PASS (line-ending notices only)
- Real CLI schema-v2 profile validation: PASS
- `go test -race ./...`: attempted; unavailable because this Windows Go environment has CGO disabled and no C compiler

### Preserved unrelated edits

Seven pre-existing formatting-only edits remain preserved. Phase 6 necessarily overlaps `internal/metrics/aggregate.go`; its prior formatting changes were retained.

### Tester focus

Independently verify exact segment/root/measured reconciliation, deterministic-clock boundary behavior, v1/v2 strict schema compatibility, report redaction, compare size/structure rejection, and resource complexity. No commit or push was performed.