# DAEGSA Progress Record

Canonical phase: Phase 6 - Profiles and Rate-Limit Analysis
Tranche: entire phase
Status: COMMITTED

Intended commit subject: `feat(profile): add segmented load analysis and report comparison`

## Acceptance evidence summary

- **Deterministic profile compilation:** Source segments compile into immutable constant-rate intervals; ramp segments expand deterministically into discrete constant steps with exact nanosecond duration partitioning.
- **Ordered stage validation:** Strictly enforces `warmup` -> `measured` -> `cooldown` stage ordering with at least one `measured` segment required.
- **Safety bounds & CLI conflicts:** Rejects `--rate` and `--duration` flags when segmented profiles are configured; validates peak target RPS against `MaxAllowedRate` and total duration against `MaxAllowedDuration`.
- **Segment-aware open scheduler:** Single stable worker pool and shared transport across all segments; segment-local pacing reset without catch-up bursts; graceful drain after final segment.
- **Segment metrics reconciliation:** Memory complexity bounded at O(workers + segments); exact reconciliation between root metrics and segment aggregates (planned, scheduled, started, completed, canceled, dropped, status codes, outcomes, and 429s); threshold and default pass/fail evaluated strictly against measured segments.
- **Rate-limit intelligence:** Per-segment 429 tracking; `FirstThrottleOffsetNS` recorded; delta-seconds and RFC1123/RFC850 HTTP-Date `Retry-After` parsing; standard `RateLimit-*` precedence over legacy `X-RateLimit-*`; header consistency and parse validity tracking; control-character sanitization and secret redaction; explicit no-throttling warning banner.
- **Generator calibration:** Reliability indicators and warnings for low achieved start rate (<95%), scheduler lag (>50ms), and dropped requests due to `max_in_flight`; CPU saturation is marked unavailable when sampling is not present.
- **Report schemas v1 & v2:** Full backward compatibility for schema v1; strict schema v2 validation with `additionalProperties: false`.
- **Report comparison (`daegsa compare`):** Size-bounded 10MB report loading; absolute and percentage deltas across percentiles, throughput, error rates, 429s, and segments; safe zero-baseline handling; threshold transition tracking (`fail-to-pass`, `pass-to-fail`, `added`, `removed`); comparability warnings for configuration/model differences.
- **Validation suite:** All repository tests (`go test -count=1 ./...`), `go vet ./...`, `gofmt -l .`, and `git diff --check` passed cleanly.

## Remaining phase/tranche work

- Phase 6 has no remaining defects or unfulfilled requirements.
- Race detection (`go test -race ./...`) should be executed when running in an environment with a C compiler (CGO enabled).

## Next recommended reader scope

- **Phase 7 - Multi-Step Scenarios:** Per-VU state and cookies, JSON-path extraction and variable substitution, think time and deterministic data selection, login/refresh flows, step-level metrics and thresholds.
- **Phase 8 - Distribution and Production Hardening:** Windows AMD64 standalone release packaging, checksums, embedded metadata, SBOM, `-trimpath` builds, `doctor` command, self-test, and operational documentation.
