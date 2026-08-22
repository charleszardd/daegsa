# DAEGSA

REST API Load, Capacity, and Rate-Limit Testing CLI

## Name meaning

- Daega / 대가 (代價/대가) can mean price, cost, or compensation depending on context.
- Dagsa in Cebuano/Bisaya means an influx or a large number arriving.
- SA is inspired by the ending of *dagsa* and its crowd/influx meaning.

Concept: measure the performance cost of a large influx of API traffic.

# 1. Product Direction

DAEGSA is a portable Go CLI for repeatable REST API load, capacity, stress, spike, soak, rate-limit, and authenticated endpoint testing. The first target is a standalone Windows x64 executable that teammates can run without Go, Node.js, Java, Python, or Docker.

```text
daegsa.exe run --config examples/donations.yaml

daegsa.exe run --url "https://api.example.com/items" --method GET \
  --model open --rps 100 --duration 30s
```

DAEGSA should not try to reproduce every feature of k6, Vegeta, JMeter, or other mature tools. Its value is an opinionated workflow for backend teams that need:

- A portable, script-free Windows executable.
- Strict, reviewable YAML rather than a general-purpose scripting runtime.
- Explicit open- and closed-workload semantics.
- Strong production and destructive-request safeguards.
- First-class HTTP 429 and rate-limit analysis.
- Environment-based secrets and deterministic token pools.
- Stable JSON reports, thresholds, and exit codes for CI.
- Generator-health reporting so client saturation is not mistaken for server saturation.
- Reproducible test manifests and simple before/after result comparison.

## Non-goals for v1

- Browser rendering or frontend performance testing.
- WebSocket, gRPC, GraphQL-specific, or non-HTTP protocols.
- A JavaScript/plugin execution runtime.
- Distributed load generation.
- A hosted dashboard or cloud control plane.
- Automatic claims about the target's safe production capacity.
- Unbounded response capture or full payload logging.

# 2. Workload Model and Terminology

## Requests per second (RPS)

RPS means **requests per second**. A target of `100 RPS` asks DAEGSA to start approximately 100 HTTP requests every second.

RPS is not the same as concurrent users:

- RPS controls how often requests start.
- Concurrency is the number of requests currently in flight.
- Virtual users represent independent user loops or sessions.
- Completed throughput is how many requests finish per second.

If the target becomes slow, 100 RPS may require hundreds of concurrent in-flight requests. DAEGSA must therefore report target rate, achieved start rate, completed throughput, concurrency, and dropped requests separately.

## Open model

In an open model, requests are scheduled according to an arrival rate independent of response time.

Example: with `rate: 100` and `time_unit: 1s`, DAEGSA attempts to start requests at evenly spaced intervals throughout each second. A slow response does not intentionally reduce the next second's target rate.

The open model is the recommended flagship model for:

- API capacity measurement.
- Rate-limit discovery and verification.
- Spike and ramp tests.
- Testing behavior during degradation.
- Reproducing externally imposed traffic rates.

The open model can overload both the target and the generator. It requires a hard `max_in_flight` limit. When that limit is reached, DAEGSA records a dropped request instead of building an unbounded queue or issuing a later catch-up burst.

## Closed model

In a closed model, a fixed number of virtual users repeatedly send a request and wait for its completion before sending the next one.

The closed model is recommended for:

- Simulating a fixed number of interactive users.
- Session and multi-step workflow testing.
- Tests where users naturally wait for a response.
- Establishing a simple baseline before arrival-rate testing.

A closed model can hide degradation because slower responses automatically reduce the request rate. DAEGSA must show this reduced achieved RPS clearly.

## Product decision

Neither model is universally best. DAEGSA will support both through an explicit `load.model` field:

```yaml
load:
  model: open
  rate: 100
  time_unit: 1s
  max_in_flight: 500
  duration: 30s
```

```yaml
load:
  model: closed
  users: 50
  duration: 30s
  think_time: 250ms
```

Open model is the primary recommendation for single-endpoint capacity and rate-limit testing. Closed model remains a first-class option rather than being emulated through an RPS limiter.

# 3. Recommended Technology

| Area | Choice |
| --- | --- |
| Language | Go |
| Configuration | Strict, versioned YAML |
| CLI | Cobra |
| HTTP | Go `net/http` |
| Metrics | Bounded per-worker histograms and counters |
| Reports | Terminal + versioned JSON; JUnit later |
| Distribution | Standalone executable + checksums through GitHub Releases |

Do not lock the implementation to a histogram library until accuracy, allocation rate, merge behavior, maintenance, and Windows compatibility are benchmarked. Hide the implementation behind an internal histogram interface.

# 4. High-Level Architecture

```text
CLI Parser
    |
Config Loader -> Strict Validation -> Safety Preflight
    |                                   |
    +---------------- Test Planner -----+
                         |
               Workload Controller
                 /               \
        Open Arrival Scheduler   Closed VU Scheduler
                 \               /
                  Request Executor
                         |
          Auth + Per-VU State + Request Builder
                         |
              Shared Tuned HTTP Transport
                         |
                     Target API
                         |
                 Outcome Observation
                         |
          Per-Worker Bounded Metrics / Histograms
                         |
                 Merge and Evaluation
                    /             \
             Console Report    JSON Report
```

The request hot path must not acquire one global metrics mutex for every request. Prefer worker-local aggregation with periodic snapshots and a deterministic final merge.

# 5. Repository Structure

```text
daegsa/
├── cmd/daegsa/main.go
├── internal/
│   ├── cli/           # Cobra commands and exit-code mapping
│   ├── config/        # YAML schema, strict decoding, env expansion
│   ├── plan/          # Validated immutable execution plan
│   ├── safety/        # Target and destructive-request preflight
│   ├── scheduler/     # Open and closed workload controllers
│   ├── executor/      # HTTP request execution and classification
│   ├── auth/          # Static auth, token pools, per-VU sessions
│   ├── metrics/       # Counters, histograms, snapshots, merge
│   ├── threshold/     # Pass/fail evaluation
│   ├── report/        # Terminal, JSON, and later JUnit
│   ├── compare/       # Report-to-report regression comparison
│   └── testtarget/    # Deterministic local integration server
├── examples/
├── testdata/
├── benchmarks/
├── dist/
├── docs/
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

# 6. Configuration Contract

Every YAML document must have a schema version. Unknown fields and duplicate keys are errors.

```yaml
schema_version: 1
name: donation-api-capacity

request:
  url: ${BASE_URL}/api/donations
  method: GET
  headers:
    Accept: application/json
    Authorization: Bearer ${API_TOKEN}
  expected_statuses: [200]
  timeout: 5s
  response_body_limit: 1MiB
  redirects: same-origin

load:
  model: open
  rate: 100
  time_unit: 1s
  max_in_flight: 500
  duration: 30s
  graceful_stop: 10s

rate_limit:
  treat_429_as_expected: false

thresholds:
  http_error_rate: "<= 1%"
  p95: "<= 500ms"
  p99: "<= 1s"
  completed_rps: ">= 90"
  dropped_requests: "== 0"

safety:
  allowed_hosts:
    - api.example.com
  allow_non_idempotent: false
```

## Validation rules

- Reject unknown and duplicate YAML fields.
- Reject incompatible model fields, such as `users` on an open model or `rate` on a closed model.
- Require positive duration, timeout, rate/users, and safety ceilings.
- Define CLI precedence as: explicit CLI flag, then environment value, then YAML, then documented default.
- Expand only explicit `${VARIABLE}` placeholders and define how literal placeholders are escaped.
- Never include resolved secrets in errors, logs, fingerprints, or reports.
- Record a sanitized configuration fingerprint in the JSON report.
- The `validate` command performs parsing, normalization, safety checks, and secret-presence checks without sending traffic or printing secret values.

# 7. Execution Semantics

## Test lifecycle

1. Parse and strictly validate configuration.
2. Resolve environment placeholders without mutating the source file.
3. Perform safety and target preflight.
4. Build an immutable execution plan.
5. Optionally perform a short warm-up excluded from threshold evaluation.
6. Start the monotonic test clock and scheduler.
7. Stop scheduling new requests when duration expires.
8. Allow in-flight requests to finish during `graceful_stop`.
9. Cancel remaining requests after the grace period.
10. Merge metrics, evaluate thresholds, write reports, and return the documented exit code.

## Open scheduler requirements

- Use a monotonic clock for scheduling and duration measurement.
- Space arrivals across the configured time unit rather than sending one batch each second.
- Do not maintain an unbounded request queue.
- Do not issue catch-up bursts after scheduler delays.
- Count work that could not start because of `max_in_flight` as dropped.
- Track scheduler lag and the achieved request-start rate.
- Make ramp and spike profiles compile into explicit time segments before execution.

## Closed scheduler requirements

- Run exactly the configured number of virtual-user loops.
- Give each virtual user independent state and, when enabled, an independent cookie jar.
- Wait for the current iteration to finish before the same user starts another.
- Support fixed or deterministic seeded think time later; do not introduce uncontrolled randomness.

# 8. HTTP Correctness

- Reuse one tuned `http.Transport`; never create a transport per request.
- Use per-VU clients or cookie jars when session isolation is enabled while sharing the transport.
- Expose safe connection controls such as maximum connections and idle connections per host.
- Report negotiated HTTP protocol because HTTP/1.1 and HTTP/2 have different connection/concurrency behavior.
- Use same-origin redirects by default and re-run safety validation for every redirect destination.
- Do not retry requests automatically in v1. A retry changes generated load and can make non-idempotent operations unsafe.
- Measure latency from immediately before transport execution until the configured response body has been consumed or intentionally capped.
- Drain and close response bodies when safe so keep-alive reuse behaves predictably.
- Cap response bytes retained and total bytes read according to configuration.
- Separate request timeout, test cancellation, DNS, connection, TLS, write, header, and body-read failures where Go exposes them reliably.
- Track bytes sent and received so local bandwidth saturation can be detected.

# 9. Outcome and Metrics Model

## Outcome taxonomy

Every scheduled request has one terminal classification:

- `success`
- `unexpected_status`
- `rate_limited`
- `timeout`
- `dns_error`
- `connect_error`
- `tls_error`
- `request_build_error`
- `response_body_error`
- `canceled`
- `other_transport_error`
- `dropped` for work that was scheduled but never started

HTTP success is determined by `request.expected_statuses`, not by a hard-coded `2xx` rule. A 429 is always counted separately and may additionally pass or fail thresholds according to explicit rate-limit-test configuration.

## Required measurements

- Planned, scheduled, started, completed, canceled, and dropped requests.
- Successful and failed outcomes by category.
- HTTP status-code distribution.
- Target arrival rate, achieved start rate, and completed throughput.
- Current and maximum in-flight requests.
- Minimum, maximum, mean, p50, p90, p95, and p99 latency.
- Latency distribution for all completed HTTP responses and, separately, expected-success responses.
- Bytes sent and received.
- `Retry-After`, standardized `RateLimit-*`, and common `X-RateLimit-*` observations.
- Generator CPU, memory, goroutine count, scheduler lag, and socket/connection warnings where reliably measurable.

## Bounded-memory requirements

- Never retain every response or latency sample during long tests.
- Use bounded histograms with documented range and precision.
- Keep only bounded samples of distinct error messages, with counts for every normalized error class.
- Bound status/header cardinality and never use secrets or arbitrary response values as metric labels.

# 10. Thresholds and Exit Codes

Threshold syntax must include an operator and an unambiguous unit:

```yaml
thresholds:
  http_error_rate: "<= 1%"
  rate_limited_rate: "<= 5%"
  p95: "<= 500ms"
  p99: "<= 1s"
  completed_rps: ">= 300"
  dropped_requests: "== 0"
```

Thresholds are evaluated only against the measured phase, excluding configured warm-up. The report records each expression, observed value, and result.

Stable process exit codes:

| Code | Meaning |
| --- | --- |
| 0 | Test completed and all thresholds passed |
| 1 | Test completed but one or more thresholds failed |
| 2 | CLI usage or configuration validation failed |
| 3 | Runtime/tool failure prevented a valid test result |
| 4 | Safety policy refused execution |

# 11. Authentication, Secrets, and State

- Resolve secrets from environment variables; committed YAML should contain references only.
- Redact Authorization, proxy authorization, API-key headers, cookies, set-cookie values, and configured sensitive headers.
- Static Bearer and custom-header authentication are sufficient for v0.1.
- Token pools use deterministic assignment. Closed tests assign tokens to virtual users; open tests assign them to stable worker lanes rather than relying on completion order.
- Multi-step scenarios keep extracted variables and cookies per virtual user.
- Login/session refresh is deferred until scenario execution is stable.
- Reports record the authentication mode and token count, never token values.

# 12. Safety Controls

- Enforce hard maximum users, RPS, in-flight requests, duration, request bytes, and response bytes.
- Treat CLI safety ceilings as upper bounds that YAML cannot override.
- Require explicit host allowlisting for protected or production targets.
- Require a separate explicit authorization for POST, PUT, PATCH, and DELETE against protected targets.
- Resolve and display the final scheme, hostname, port, and IP addresses during preflight.
- Revalidate redirect destinations and block cross-origin redirects by default.
- Print the effective model, target, RPS/users, maximum concurrency, duration, method, and safety flags before execution.
- Provide `--dry-run` to print the sanitized execution plan without sending traffic.
- Provide `--non-interactive` for CI; it must fail rather than wait for confirmation.
- Recommend staging/sandbox systems for payment, email, SMS, webhook, or other externally integrated endpoints.
- Do not add an insecure TLS flag to the initial MVP. If added later, make it noisy, explicit, and visible in reports.

# 13. Reports and Reproducibility

## Terminal report

Lead with:

- Test result and threshold summary.
- Model and effective load.
- Target versus achieved start RPS and completed throughput.
- Success, error, 429, canceled, and dropped rates.
- p50, p90, p95, p99, and maximum latency.
- Generator saturation warnings.

Average latency may be shown for completeness but must not be the headline measurement.

## JSON report

The JSON report requires its own `report_schema_version` and includes:

- DAEGSA version, commit, build date, OS, and architecture.
- Sanitized configuration and configuration fingerprint.
- UTC start/end times and monotonic elapsed duration.
- Workload model and compiled profile segments.
- Request/outcome counters, status distribution, latency summary, and rate-limit observations.
- Generator-health measurements and warnings.
- Threshold expressions, observed values, and pass/fail results.
- Incomplete-result marker when forced cancellation or runtime failure invalidates interpretation.

Report schema changes must follow documented compatibility rules. Add `daegsa compare baseline.json candidate.json` after the base report format is stable.

# 14. Features That Sharpen DAEGSA

These features differentiate DAEGSA without turning it into a general scripting platform.

## Rate-limit intelligence

- Report the first observed throttling segment and rate.
- Parse `Retry-After` as both delta seconds and HTTP date.
- Record consistency of limit, remaining, and reset headers.
- Show throttled percentage by profile segment.
- Never present an observed no-throttle rate as a guaranteed safe production limit.

## Generator self-diagnostics

- Add `daegsa doctor` for DNS, TLS, clock, socket, CPU, and local resource checks.
- Warn when CPU, scheduler lag, sockets, bandwidth, or `max_in_flight` prevent the requested load.
- Include a deterministic local `daegsa self-test` target or integration harness.
- Benchmark scheduler drift and metrics overhead in CI.

## Reproducibility and comparison

- Store a sanitized execution-plan fingerprint and optional user-supplied build label.
- Support deterministic seeded payload/token selection.
- Compare two JSON reports and highlight threshold and percentile regressions.
- Keep comparison statistical claims conservative until repeated-run variance is modeled.

## CI usability

- Stable exit codes and report schema.
- Optional JUnit output after JSON stabilizes.
- Concise one-line failure summaries suitable for GitHub Actions, GitLab CI, Jenkins, and Azure DevOps.
- Never prompt in non-interactive mode.

# 15. Implementation Phases

## Phase 0 - Freeze Semantics and Build the Test Harness

- Approve open/closed model semantics, outcome taxonomy, timing boundary, shutdown behavior, and exit codes.
- Define YAML schema v1 and JSON report schema v1.
- Build a deterministic local HTTP server supporting delay, status, body size, redirect, disconnect, timeout, cookies, and 429 responses.
- Add scheduler-clock abstractions where needed for deterministic tests without replacing real integration tests.
- Capture baseline CPU, allocation, and timer behavior on Windows AMD64.

**Exit gate:** ambiguous workload and outcome behavior is resolved in executable contract tests.

## Phase 1 - Configuration, Safety, and HTTP Executor

- Implement `run`, `validate`, `version`, and `help`.
- Strict YAML parsing, environment resolution, sanitized plans, and precedence rules.
- Safety ceilings, host policy, redirect restrictions, dry-run, and CI mode.
- Correct request building, shared transport, timeout, body cap, drain/close, and error classification.

**Exit gate:** one request can be executed and classified correctly across all deterministic test-target behaviors.

## Phase 2 - Metrics and Closed-Model Baseline

- Implement worker-local counters and bounded histograms.
- Implement fixed virtual-user loops, think time, duration, graceful stop, and cancellation.
- Produce terminal and JSON reports.
- Add race, leak, allocation, and long-duration bounded-memory tests.

**Exit gate:** closed tests are repeatable, memory-bounded, and do not leak goroutines or connections.

## Phase 3 - Open Arrival-Rate Engine

- Implement constant arrival rate with fractional spacing.
- Enforce `max_in_flight`, dropped-request accounting, and no catch-up bursts.
- Measure scheduling lag, achieved start rate, and completed throughput.
- Test slow-target and generator-saturation behavior.

**Exit gate:** scheduled-rate accuracy and dropped-work semantics pass deterministic and real-clock tolerances.

## Phase 4 - Thresholds and CI Contract

- Implement explicit threshold parsing and evaluation.
- Lock stable exit codes.
- Add incomplete-result semantics.
- Add report-schema golden tests and CI examples.

**Exit gate:** CI can reliably distinguish regression, invalid configuration, runtime failure, and safety refusal.

## Phase 5 - Authentication and Secret Handling

- Static bearer and custom-header authentication.
- Deterministic token pools for both workload models.
- Per-VU cookies for closed tests.
- Central redaction tests covering errors, terminal output, and reports.

**Exit gate:** secrets cannot appear in standard outputs, fixtures, or reports.

## Phase 6 - Profiles and Rate-Limit Analysis

- Ramp, stress, spike, soak, warm-up, and cool-down segments.
- 429 and rate-limit header analysis by segment.
- Generator calibration warnings.
- Before/after report comparison.

**Exit gate:** profile compilation is deterministic and segment results reconcile with the overall report.

## Phase 7 - Multi-Step Scenarios

- Per-VU state and cookies.
- JSON-path extraction and variable substitution.
- Think time and deterministic data selection.
- Login and refresh flows.
- Step-level metrics and thresholds.

**Exit gate:** users remain isolated and scenario failures have explicit stop/continue behavior.

## Phase 8 - Distribution and Production Hardening

- Windows AMD64 release workflow, checksums, embedded version metadata, and SBOM.
- Reproducible `-trimpath` builds and release smoke tests.
- `doctor`, self-test, operational documentation, and safety runbook.
- Linux and additional architectures only after Windows behavior is stable.

**Exit gate:** a clean Windows machine can validate, execute, report, and diagnose a test without an installed runtime.

# 16. Version Roadmap

| Version | Scope |
| --- | --- |
| v0.1.0 | Config/safety contract, HTTP executor, closed model, bounded metrics |
| v0.2.0 | Open arrival-rate model, max-in-flight, dropped-work reporting |
| v0.3.0 | Thresholds, stable JSON schema, CI exit-code contract |
| v0.4.0 | Bearer/API-key auth, token pools, redaction hardening |
| v0.5.0 | Ramp, stress, spike, soak, warm-up/cool-down profiles |
| v0.6.0 | Rate-limit intelligence and generator diagnostics |
| v0.7.0 | Report comparison and optional JUnit output |
| v1.0.0 | Stable single-endpoint CLI and Windows distribution contract |
| v1.1.0 | Multi-step scenarios and extraction |
| v1.2.0 | Login/session automation |
| v2.0.0 | Distributed generation only after single-node calibration |

# 17. v0.1 Engineering MVP

The first milestone remains focused but must be a complete vertical slice:

- Portable Windows AMD64 executable.
- Strict versioned YAML and `validate` command.
- GET, POST, PUT, PATCH, and DELETE with explicit destructive-request safety.
- Fixed virtual-user closed model.
- Configurable duration, timeout, graceful stop, headers, and bounded request/response bodies.
- Static Bearer and custom-header values through environment references may be parsed, but token pools remain later.
- Shared reusable transport with safe redirect and connection defaults.
- Bounded metrics: counts, outcome classes, status distribution, achieved RPS, p50, p90, p95, and p99.
- Terminal and versioned JSON reports.
- Automated unit, integration, race, leak, and benchmark tests for the executor, runner, and metrics components.

The open arrival-rate engine is the immediate v0.2 priority and is required before DAEGSA is presented as a serious API capacity or rate-limit testing tool.

# 18. Engineering Principles

- Correct workload semantics before raw request volume.
- Correctness before convenience or feature count.
- The generator must measure and disclose its own limitations.
- Never mistake achieved start rate for completed throughput.
- Never hide scheduled work that was dropped or canceled.
- Separate planner, scheduler, executor, transport, metrics, authentication, safety, threshold, and reporting concerns.
- Use bounded memory and bounded cardinality everywhere in the hot path.
- Drain and close response bodies correctly to preserve intentional connection behavior.
- Never log credentials or full sensitive payloads by default.
- Prefer deterministic configuration and seeded behavior over uncontrolled randomness.
- Do not add distributed generation until single-node DAEGSA is stable, calibrated, and benchmarked.

# 19. Acceptance Criteria for v1

- Open and closed workload models behave according to their documented contracts.
- Target rate, achieved start rate, completed throughput, concurrency, and dropped work are reported separately.
- Slow targets cannot cause an unbounded local queue or silent catch-up burst.
- Metrics remain memory-bounded during soak tests.
- HTTP outcomes, 429 responses, timeouts, transport failures, cancellations, and dropped work remain distinguishable.
- Unknown configuration fields, unsafe targets, and destructive requests fail before traffic starts.
- Secrets do not appear in terminal output, errors, fingerprints, or JSON reports.
- Threshold and runtime failures produce stable, distinct exit codes.
- JSON reports are versioned and reproducible enough for CI comparison.
- Generator saturation produces visible warnings rather than misleading server conclusions.
- The Windows release runs on a clean machine without an installed language runtime.

# 20. References

- Grafana k6, open and closed workload models: <https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/open-vs-closed/>
- Grafana k6, arrival-rate executors: <https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/>
- Go `net/http.Transport`: <https://pkg.go.dev/net/http#Transport>
- Vegeta design and reporting terminology: <https://github.com/tsenart/vegeta>

DAEGSA - measure the cost of the influx.
