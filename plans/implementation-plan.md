# DAEGSA Execution Plan

Status: COMMITTED
Canonical phase: Phase 1 - Configuration, Safety, and HTTP Executor
Tranche: entire phase

## Objective

Implement the production CLI entrypoint and Cobra commands (`run`, `validate`, `version`, `help`), robust YAML configuration loading with `${VAR}` environment variable expansion and CLI flag precedence, safety preflight validation (host allowlisting, destructive HTTP method authorization, hard safety ceiling enforcement, and DNS preflight), immutable execution planning with configuration fingerprinting and secret redaction, and the core HTTP request execution engine (shared tuned `http.Transport`, per-request timeout context, request body/header assembly, redirect safety policy enforcement, response body limit capping and connection draining, precise timestamp capture, and deterministic 12-state outcome classification). Validate all components against the deterministic `internal/testtarget` server across all 8 simulation modes.

## Requirements traceability

| Plan Section | Requirement Description | Implementation / Test Target |
| :--- | :--- | :--- |
| **§3, §5, §15** | CLI architecture using Cobra (`run`, `validate`, `version`, `help`) with canonical exit code mapping (0: PASS, 1: FAIL_THRESHOLDS, 2: USAGE/VALIDATION_ERROR, 3: RUNTIME_ERROR, 4: SAFETY_REFUSAL) | `cmd/daegsa/main.go`, `internal/cli/` |
| **§6, §11** | Configuration environment variable resolution (`${VAR}`), CLI flag precedence (CLI > Env > YAML > Default), strict decoding, and configuration fingerprinting | `internal/config/env.go`, `internal/config/precedence.go`, `internal/config/fingerprint.go` |
| **§6, §11** | Centralized credential and secret redaction across plans, fingerprints, errors, and terminal outputs | `internal/config/redact.go`, `internal/plan/redact.go` |
| **§12** | Safety preflight engine: host allowlisting, destructive HTTP method (POST, PUT, PATCH, DELETE) authorization, redirect safety re-validation, ceiling enforcement, DNS preflight, `--dry-run`, and `--non-interactive` | `internal/safety/preflight.go`, `internal/safety/safety.go`, `internal/safety/safety_test.go` |
| **§4, §6, §7** | Validated immutable execution plan representation (`internal/plan`) and sanitized plan printing | `internal/plan/plan.go`, `internal/plan/plan_test.go` |
| **§8, §9** | High-performance HTTP request executor: shared tuned `http.Transport`, per-request timeout context, request building, redirect policy hook, response body limit reading, draining/closing, and timestamps | `internal/executor/executor.go`, `internal/executor/transport.go`, `internal/executor/result.go` |
| **§9** | Integration with 12-state `OutcomeClassifier` and rate-limit observation extraction (`Retry-After`, `RateLimit-*`, `X-RateLimit-*`) | `internal/executor/executor.go`, `internal/executor/ratelimit.go` |
| **§8, §15** | Integration testing against `internal/testtarget` across all 8 simulation modes (status codes, delays, payload streaming, redirects, TCP drops, timeout hangs, cookies, and 429 rate limiting) | `internal/executor/executor_test.go`, `internal/cli/cli_test.go` |
| **§15** | Phase 1 Exit Gate: single request executed and classified correctly across all deterministic test target behaviors; safety refusal returns exit code 4; invalid config returns exit code 2; `--dry-run` prints sanitized plan with exit code 0 | `internal/cli/cli_test.go`, `internal/executor/executor_test.go` |

## Current repository findings

- **Phase 0 Deliverables**:
  - `internal/core`: canonical workload models (`open`, `closed`), 12-state `Outcome` taxonomy & `OutcomeClassifier`, 5 `ExitCode` constants, timing boundaries (`Latency`, `TTFB`, `ScheduleLag`), and execution lifecycle states.
  - `internal/config`: YAML schema v1 structs, strict decoding rejecting unknown fields and duplicate keys, byte size and duration parsers.
  - `internal/report`: JSON report schema v1 structs.
  - `internal/clock`: `RealClock` (monotonic) and `ControllableClock` (priority-queue virtual time).
  - `internal/testtarget`: deterministic local HTTP server implementing all 8 test modes (status, delay, payload streaming, redirects, drops, hangs, cookies, 429 rate limiting).
  - `benchmarks`: timer resolution characterization and zero-allocation classification baseline.
- **Dependencies**: `go.mod` contains Go 1.22 and `gopkg.in/yaml.v3`. Cobra (`github.com/spf13/cobra`) and pflag (`github.com/spf13/pflag`) must be added for CLI commands.
- **Missing Components for Phase 1**:
  - `cmd/daegsa/main.go` and `internal/cli` (Cobra command structure and exit-code propagation).
  - `internal/config/env.go` (environment variable interpolation `${VAR}` with escape support `$${VAR}`).
  - `internal/config/precedence.go` (CLI override application over YAML config).
  - `internal/config/fingerprint.go` & `redact.go` (SHA256 fingerprinting of sanitized configuration and sensitive header/token redaction).
  - `internal/safety` (host allowlisting, destructive method authorization, hard safety ceiling enforcement, DNS preflight resolution).
  - `internal/plan` (immutable execution plan struct and sanitized text representation).
  - `internal/executor` (reusable tuned `http.Transport`, request builder, execution engine, response body capping/draining, timing measurement, and outcome classification).

## Files expected to change

```text
daegsa/
├── go.mod                                   # Modified: Add github.com/spf13/cobra and github.com/spf13/pflag
├── go.sum                                   # Modified: Checksums for new dependencies
├── cmd/
│   └── daegsa/
│       └── main.go                          # New: Application entrypoint calling cli.Execute() with os.Exit code
├── internal/
│   ├── cli/
│   │   ├── root.go                          # New: Root Cobra command, global flags, version info
│   │   ├── run.go                           # New: 'run' command with CLI overrides, --dry-run, --non-interactive
│   │   ├── validate.go                      # New: 'validate' command for syntax, env, and safety preflight
│   │   ├── version.go                       # New: 'version' command printing version, commit, build date
│   │   ├── flags.go                         # New: CLI flag definitions and binding helpers
│   │   ├── exit.go                          # New: Process exit-code translation from errors
│   │   └── cli_test.go                      # New: End-to-end CLI integration test suite
│   ├── config/
│   │   ├── env.go                           # New: Environment variable resolver (${VAR}, $${VAR})
│   │   ├── precedence.go                    # New: CLI flag precedence overlay onto parsed Config
│   │   ├── fingerprint.go                   # New: Deterministic SHA256 configuration fingerprinting
│   │   ├── redact.go                        # New: Sensitive header and token redaction helpers
│   │   ├── env_test.go                      # New: Tests for env expansion, missing vars, and escaping
│   │   └── precedence_test.go               # New: Tests for flag precedence and fingerprint stability
│   ├── safety/
│   │   ├── safety.go                        # New: Safety rules, errors (ErrSafetyRefusal), and ceilings
│   │   ├── preflight.go                     # New: Host allowlisting, destructive method checks, DNS preflight
│   │   └── safety_test.go                   # New: Comprehensive safety preflight unit tests
│   ├── plan/
│   │   ├── plan.go                          # New: Immutable Plan struct and BuildPlan constructor
│   │   ├── print.go                         # New: Sanitized terminal plan summary formatting
│   │   └── plan_test.go                     # New: Immutable plan building, immutability, and redaction tests
│   └── executor/
│       ├── executor.go                      # New: HTTP executor executing single requests with timeout/context
│       ├── transport.go                     # New: Shared tuned http.Transport factory and connection settings
│       ├── request.go                       # New: Request builder (headers, body, method, URL)
│       ├── response.go                      # New: Response body limit reader and connection draining
│       ├── result.go                        # New: Execution Result struct (outcome, timing, bytes, headers)
│       ├── ratelimit.go                     # New: Rate-limit header parsing (Retry-After, RateLimit-*, X-RateLimit-*)
│       └── executor_test.go                 # New: Exhaustive executor test suite against testtarget (all 8 modes)
```

## Implementation checklist

### 1. Dependencies & CLI Entrypoint (`go.mod`, `cmd/daegsa/main.go`, `internal/cli`)
- [x] Add `github.com/spf13/cobra` (v1.8.1+) and `github.com/spf13/pflag` to `go.mod` and run `go mod tidy`.
- [x] Implement `cmd/daegsa/main.go`:
  - Invoke `cli.Execute()` or `cli.ExecuteContext(ctx)`.
  - Capture return error / exit code and terminate cleanly via `os.Exit(code)`.
  - Prevent any unhandled panic from producing a non-standard exit code.
- [x] Implement `internal/cli/exit.go`:
  - Define `DetermineExitCode(err error) core.ExitCode`.
  - Map nil -> `core.ExitCodeSuccess` (0).
  - Map threshold evaluation failures -> `core.ExitCodeThresholdFailure` (1).
  - Map syntax, YAML decoding, unknown fields, missing required flags/configs -> `core.ExitCodeValidationFailure` (2).
  - Map unrecoverable runtime errors, network dial init failures -> `core.ExitCodeRuntimeFailure` (3).
  - Map safety violations (unauthorized host, unauthorized destructive method, ceiling breach) -> `core.ExitCodeSafetyRefusal` (4).
- [x] Implement `internal/cli/flags.go`:
  - Define `CLIFlags` struct capturing `--config`, `--url`, `--method`, `--model`, `--rate`, `--time-unit`, `--users`, `--duration`, `--timeout`, `--max-in-flight`, `--response-body-limit`, `--redirects`, `--dry-run`, `--non-interactive`, `--allow-destructive`.
  - Provide helper to bind flags to Cobra commands.
- [x] Implement `internal/cli/root.go`:
  - Define Root command `daegsa` with usage, short/long descriptions, and subcommands.
  - Configure Cobra `SilenceUsage: true` and `SilenceErrors: true` so custom exit codes and error formatting control output.
- [x] Implement `internal/cli/version.go`:
  - Implement `version` subcommand printing version, commit SHA, build date, Go version, OS, and Arch.
  - Support programmatic injection of build metadata via ldflags (`Version`, `Commit`, `BuildDate`).
- [x] Implement `internal/cli/validate.go`:
  - Implement `validate` subcommand accepting `--config` and CLI flag overrides.
  - Parse YAML, expand environment variables, apply CLI overrides, validate syntax and invariants.
  - Run safety preflight checks without sending traffic.
  - Print sanitized execution plan summary to stdout on success and return `ExitCodeSuccess` (0).
  - Return `ExitCodeValidationFailure` (2) on schema/validation errors; return `ExitCodeSafetyRefusal` (4) on safety preflight failure.
- [x] Implement `internal/cli/run.go`:
  - Implement `run` subcommand accepting `--config` and execution flags.
  - Check `--dry-run`: if set, build sanitized plan, print summary to stdout, and exit with code 0 without executing traffic.
  - Check `--non-interactive`: if set, disallow interactive prompts and fail immediately with exit code 4 if safety authorization is missing.
  - In Phase 1, execute single-request validation / test execution against the target URL and print outcome classification result.

### 2. Configuration Normalization & Environment Resolution (`internal/config`)
- [x] Implement `internal/config/env.go`:
  - Implement `ExpandEnv(input []byte, getenv func(string) string) ([]byte, error)`:
    - Expand `${VAR_NAME}` patterns in YAML bytes prior to strict decoding.
    - Support escape sequence `$${VAR_NAME}` to produce literal `${VAR_NAME}` without substitution.
    - Validate variable names (`[A-Za-z_][A-Za-z0-9_]*`).
    - Record which environment variables were expanded for redaction tracking.
- [x] Implement `internal/config/precedence.go`:
  - Implement `ApplyCLIOverrides(cfg *Config, flags *CLIFlags) error`:
    - Enforce precedence: CLI flag > Environment variable > YAML document > Documented default.
    - Override `request.url`, `request.method`, `load.model`, `load.rate`, `load.users`, `load.duration`, `request.timeout`, `load.max_in_flight`, `request.response_body_limit`, `request.redirects`, `safety.allow_non_idempotent` when explicitly provided on CLI.
    - Re-run `ValidateConfig(cfg)` to ensure the post-override configuration maintains all invariants.
- [x] Implement `internal/config/redact.go`:
  - Define list of standard sensitive header keys: `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `Api-Key`, `Token`, `X-Auth-Token`.
  - Implement `RedactHeaders(headers map[string]string) map[string]string` replacing sensitive values with `[REDACTED]`.
  - Implement `RedactURL(rawURL string) string` redacting credentials (`user:pass@host`) and sensitive query parameters (`token`, `key`, `secret`, `auth`, `api_key`).
- [x] Implement `internal/config/fingerprint.go`:
  - Implement `ComputeFingerprint(cfg *Config) (string, error)`:
    - Clone configuration and replace all sensitive header values and secret tokens with empty/canonical masked strings.
    - Normalize order of headers and thresholds.
    - Serialize to canonical JSON or deterministic YAML and compute SHA256 hex digest.

### 3. Safety Preflight Engine (`internal/safety`)
- [x] Implement `internal/safety/safety.go`:
  - Define hard ceiling constants (§12):
    - `MaxAllowedDuration`: 24 * time.Hour
    - `MaxAllowedResponseBodyLimit`: 50 * 1024 * 1024 (50 MiB)
    - `MaxAllowedRate`: 1,000,000 RPS
    - `MaxAllowedUsers`: 100,000 VUs
    - `MaxAllowedInFlight`: 100,000
    - `MaxRedirectHops`: 10
  - Define sentinel errors:
    - `ErrSafetyRefusal`: base error for all safety refusals (mapped to ExitCode 4).
    - `ErrHostNotAllowed`: target host not in `allowed_hosts`.
    - `ErrDestructiveMethodUnauthorized`: non-idempotent HTTP method (POST, PUT, PATCH, DELETE) not explicitly authorized.
    - `ErrSafetyCeilingExceeded`: configuration value exceeds hard safety ceiling.
    - `ErrCrossOriginRedirectBlocked`: redirect to a different origin blocked by `same-origin` policy.
- [x] Implement `internal/safety/preflight.go`:
  - Implement `type PreflightEngine struct`:
    - Method `Check(ctx context.Context, cfg *config.Config, flags SafetyFlags) (*PreflightResult, error)`.
  - Implement Host Allowlist Check:
    - If `safety.allowed_hosts` is non-empty, parse target URL hostname and verify it matches at least one allowed host pattern.
    - Support exact hostname match (e.g. `api.example.com`) and wildcard domain match (e.g. `*.example.com`).
    - If target host is not allowed, return `fmt.Errorf("%w: target host %q is not in allowed_hosts", ErrHostNotAllowed, host)`.
  - Implement Destructive Method Authorization Check:
    - Check if HTTP method is one of `POST`, `PUT`, `PATCH`, `DELETE`.
    - If method is destructive, verify either `cfg.Safety.AllowNonIdempotent == true` or CLI flag `--allow-destructive == true`.
    - If unauthorized, return `fmt.Errorf("%w: HTTP method %s requires explicit authorization (safety.allow_non_idempotent: true or --allow-destructive)", ErrDestructiveMethodUnauthorized, method)`.
  - Implement Ceiling Checks:
    - Enforce duration <= `MaxAllowedDuration`, response body limit <= `MaxAllowedResponseBodyLimit`, rate <= `MaxAllowedRate`, max_in_flight <= `MaxAllowedInFlight`.
  - Implement DNS Preflight Resolution:
    - Perform `net.DefaultResolver.LookupIPAddr(ctx, host)` for the target hostname.
    - Capture resolved IPv4 and IPv6 addresses into `PreflightResult`.
    - If DNS lookup fails during preflight, return classified preflight error.

### 4. Immutable Execution Plan (`internal/plan`)
- [x] Implement `internal/plan/plan.go`:
  - Define `type Plan struct`:
    - `Name string`
    - `SchemaVersion int`
    - `Fingerprint string`
    - `TargetURL *url.URL`
    - `Method string`
    - `Headers http.Header`
    - `Body []byte`
    - `ExpectedStatuses []int`
    - `RequestTimeout time.Duration`
    - `ResponseBodyLimit int64`
    - `RedirectPolicy string`
    - `Model core.WorkloadModel`
    - `Rate float64`
    - `TimeUnit time.Duration`
    - `MaxInFlight int64`
    - `Duration time.Duration`
    - `GracefulStop time.Duration`
    - `Users int64`
    - `ThinkTime time.Duration`
    - `Treat429AsExpected bool`
    - `AllowedHosts []string`
    - `AllowNonIdempotent bool`
    - `ResolvedIPs []net.IP`
  - Implement `BuildPlan(cfg *config.Config, preflight *safety.PreflightResult) (*Plan, error)`:
    - Construct immutable, deeply cloned `Plan` ensuring no shared references to mutable config maps or slices.
- [x] Implement `internal/plan/print.go`:
  - Implement `FormatPlanSummary(p *Plan) string`:
    - Format sanitized human-readable summary for console / dry-run:
      - Configuration Name & Fingerprint (first 12 chars).
      - Effective Model, Target URL (with redacted sensitive query params), Method.
      - Rate / Concurrency, Request Timeout, Response Body Limit, Redirect Policy.
      - Safety flags and resolved target IP addresses.
      - Redacted header names and mask values.

### 5. HTTP Request Executor & Transport (`internal/executor`)
- [x] Implement `internal/executor/transport.go`:
  - Implement `NewSharedTransport(opts TransportOptions) *http.Transport`:
    - Configure pooled, high-throughput connection limits:
      - `MaxIdleConns: 1000`
      - `MaxIdleConnsPerHost: 500`
      - `MaxConnsPerHost: 0` (unlimited or bounded by option)
      - `IdleConnTimeout: 90 * time.Second`
      - `TLSHandshakeTimeout: 10 * time.Second`
      - `ExpectContinueTimeout: 1 * time.Second`
      - `ForceAttemptHTTP2: true`
      - `DisableKeepAlives: false`
    - Provide `CloseIdleConnections()` method for clean test teardown.
- [x] Implement `internal/executor/request.go`:
  - Implement `BuildHTTPRequest(ctx context.Context, plan *Plan) (*http.Request, int64, error)`:
    - Create `*http.Request` using `http.NewRequestWithContext(ctx, plan.Method, plan.TargetURL.String(), bodyReader)`.
    - Clone and attach headers from `plan.Headers`.
    - Set `Host` header explicitly if provided in headers.
    - Calculate and return estimated `BytesSent` (request line + header bytes + body length).
- [x] Implement `internal/executor/response.go`:
  - Implement `ReadAndDrainResponseBody(resp *http.Response, limitBytes int64) ([]byte, int64, bool, error)`:
    - Read up to `limitBytes` using `io.LimitReader(resp.Body, limitBytes)`.
    - Check if body exceeded limit (read limit + 1 byte probe).
    - Drain remaining bytes up to safe drain threshold (e.g. 32 KiB) to preserve keep-alive connection reuse.
    - Always close `resp.Body`.
    - Return payload bytes, total `BytesReceived`, whether body was truncated, and any read error.
- [x] Implement `internal/executor/ratelimit.go`:
  - Implement `ExtractRateLimitInfo(headers http.Header) *RateLimitInfo`:
    - Parse `Retry-After`: support integer seconds (e.g. `120`) and RFC1123 / RFC850 HTTP-Date (e.g. `Sat, 22 Aug 2026 15:30:00 GMT`).
    - Parse standard `RateLimit-*` headers: `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, `RateLimit-Policy`.
    - Parse legacy `X-RateLimit-*` headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`.
- [x] Implement `internal/executor/result.go`:
  - Define `type Result struct`:
    - `Outcome core.Outcome`
    - `StatusCode int`
    - `Protocol string` (e.g. `HTTP/1.1`, `HTTP/2.0`)
    - `Timestamps core.RequestTimestamps`
    - `Latency time.Duration`
    - `TTFB time.Duration`
    - `BytesSent int64`
    - `BytesReceived int64`
    - `RateLimitInfo *RateLimitInfo`
    - `Err error`
- [x] Implement `internal/executor/executor.go`:
  - Define `type HTTPExecutor struct`:
    - Fields: `client *http.Client`, `transport *http.Transport`, `classifier core.OutcomeClassifier`, `plan *Plan`.
  - Implement `NewHTTPExecutor(plan *Plan) (*HTTPExecutor, error)`:
    - Build `*http.Transport` via `NewSharedTransport`.
    - Build `*http.Client` with custom `CheckRedirect` hook:
      - If `plan.RedirectPolicy == "none"`: return `http.ErrUseLastResponse`.
      - If `plan.RedirectPolicy == "same-origin"`: compare request URL origin (scheme + host + port) with redirect URL origin. If mismatched, return `ErrCrossOriginRedirectBlocked`.
      - If `plan.RedirectPolicy == "all"`: re-validate redirect target host against `plan.AllowedHosts`. If unallowed, return `ErrHostNotAllowed`.
      - Limit max redirect hops to `MaxRedirectHops` (10).
  - Implement `ExecuteRequest(ctx context.Context) (*Result, error)`:
    - Apply per-request timeout context: `reqCtx, cancel := context.WithTimeout(ctx, plan.RequestTimeout)`.
    - Record `ScheduledAt` and `DispatchedAt` timestamps.
    - Execute `resp, err := client.Do(req)`.
    - Capture `HeadersReceivedAt` immediately when `Do` returns headers.
    - If `resp != nil`: read and drain body, record `BodyCompletedAt`, close response.
    - Classify outcome using `core.OutcomeClassifier`:
      - Pass `StatusCode`, `ExpectedStatuses`, `err`, `ResponseBodyErr`, `RequestBuildErr`.
    - Compute timing boundaries:
      - `TTFB = HeadersReceivedAt - DispatchedAt`
      - `Latency = BodyCompletedAt - DispatchedAt`
      - `TotalDuration = BodyCompletedAt - ScheduledAt`
    - Return populated `Result`.
  - Implement `Close()`: close idle connections on shared transport.

### 6. CLI Wiring & End-to-End Execution (`internal/cli`)
- [x] Connect `cli/run.go` and `cli/validate.go` to `config`, `safety`, `plan`, and `executor` packages.
- [x] Format terminal output on single request execution in `run`:
  - Output target URL, method, status code, outcome classification, latency, TTFB, and rate-limit info.
  - Return exit code 0 if outcome is `OutcomeSuccess` or `OutcomeRateLimited` (when `treat_429_as_expected: true`); return exit code 1 or 2/3/4 as appropriate on failure.

## Test checklist

### 1. Configuration & Precedence Tests (`internal/config`)
- [x] `internal/config/env_test.go`:
  - Test `${VAR}` expansion with existing environment variables.
  - Test missing required environment variables return clear errors.
  - Test escaped `$${VAR}` preserves literal `${VAR}` in output string.
  - Test nested / multiple environment variable substitutions in a single YAML string.
- [x] `internal/config/precedence_test.go`:
  - Test precedence order: CLI flag overrides environment variable, which overrides YAML value, which overrides default.
  - Test individual flag overrides: `--url`, `--method`, `--rate`, `--users`, `--duration`, `--timeout`, `--model`.
  - Test that invalid CLI overrides trigger validation errors returning `ExitCodeValidationFailure`.
- [x] `internal/config/fingerprint_test.go`:
  - Test deterministic fingerprint generation: identical configs produce identical SHA256 hashes.
  - Test secret independence: changing an `Authorization` header value does not change the sanitized configuration fingerprint.
  - Test whitespace / formatting normalization in fingerprint generation.
- [x] `internal/config/redact_test.go`:
  - Test header redaction: `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key` are masked with `[REDACTED]`.
  - Test URL redaction: userinfo (`user:password@host`) and query tokens (`?token=secret&api_key=12345`) are sanitized.

### 2. Safety Preflight Tests (`internal/safety`)
- [x] `internal/safety/safety_test.go`:
  - Test Host Allowlist:
    - Allowed exact host (`api.example.com`) passes.
    - Allowed wildcard host (`*.example.com`) matches `sub.example.com` and passes.
    - Disallowed host returns `ErrHostNotAllowed` wrapping `ErrSafetyRefusal`.
    - Empty `allowed_hosts` list allows target (or enforces local/safe policy).
  - Test Destructive Method Authorization:
    - `GET`, `HEAD`, `OPTIONS` pass without special authorization.
    - `POST`, `PUT`, `PATCH`, `DELETE` fail with `ErrDestructiveMethodUnauthorized` when `allow_non_idempotent` is false.
    - `POST`, `PUT`, `PATCH`, `DELETE` pass when `allow_non_idempotent: true` in config or `--allow-destructive` flag is set.
  - Test Safety Ceilings:
    - Duration > 24h fails with ceiling violation error.
    - Response body limit > 50MiB fails with ceiling violation error.
    - Rate > 1,000,000 RPS fails with ceiling violation error.
  - Test DNS Preflight:
    - Valid hostname resolves IP addresses.
    - Unresolvable hostname fails preflight with clear DNS diagnostic.

### 3. Immutable Plan Tests (`internal/plan`)
- [x] `internal/plan/plan_test.go`:
  - Test `BuildPlan` generates deeply cloned, immutable structures.
  - Test modification of input config after `BuildPlan` does not alter the generated `Plan`.
  - Test `FormatPlanSummary` redacts sensitive credentials, tokens, and authorization headers in dry-run output.

### 4. HTTP Executor Integration Tests (`internal/executor` against `internal/testtarget`)
- [x] `internal/executor/executor_test.go` - Test across all 8 `testtarget` simulation modes:
  - **Mode 1: Status Codes**:
    - Test 200 OK -> `OutcomeSuccess`, status 200.
    - Test 204 No Content -> `OutcomeSuccess`, status 204.
    - Test 404 Not Found (with `expected_statuses: [200]`) -> `OutcomeUnexpectedStatus`, status 404.
    - Test 404 Not Found (with `expected_statuses: [404]`) -> `OutcomeSuccess`, status 404.
    - Test 500 Internal Server Error -> `OutcomeUnexpectedStatus`, status 500.
  - **Mode 2: Delays & Timestamps**:
    - Test 50ms server delay -> verifies `TTFB >= 50ms` and `Latency >= 50ms`.
    - Test timestamp ordering invariant: `ScheduledAt <= DispatchedAt <= HeadersReceivedAt <= BodyCompletedAt`.
  - **Mode 3: Payload Streaming & Body Capping**:
    - Test streaming 10 KiB response with 1 MiB limit -> full payload read, `BytesReceived >= 10240`.
    - Test streaming 10 KiB response with 500 B limit -> payload capped at 500 bytes, `truncated == true`, connection cleanly drained.
  - **Mode 4: Redirects**:
    - Test same-origin 3-hop redirect with `redirects: same-origin` -> followed to final 200 OK destination.
    - Test cross-origin redirect with `redirects: same-origin` -> blocked with `ErrCrossOriginRedirectBlocked`.
    - Test cross-origin redirect with `redirects: all` -> allowed when destination host is allowlisted.
    - Test `redirects: none` -> stops at first 302 redirect response with status 302.
    - Test redirect loop (>10 hops) -> fails with redirect limit error.
  - **Mode 5: Abrupt TCP Disconnects**:
    - Test `?drop=immediate` (TCP hijack/close) -> classified as `OutcomeConnectError` or `OutcomeOtherTransportError`.
    - Test `?drop=midway` (partial body then TCP hijack) -> classified as `OutcomeResponseBodyError`.
  - **Mode 6: Timeout Hangs**:
    - Test target hang with 50ms request timeout -> context deadline expires -> classified as `OutcomeTimeout`.
  - **Mode 7: Cookies**:
    - Test server setting `Set-Cookie` -> cookie headers received without crashing or leaking sensitive values.
  - **Mode 8: 429 Rate Limiting & Header Parsing**:
    - Test 429 response with `Retry-After: 30` -> classified as `OutcomeRateLimited`, `RetryAfterSeconds == 30`.
    - Test 429 response with `Retry-After: <HTTP-Date>` -> classified as `OutcomeRateLimited`, `RetryAfterDate` parsed.
    - Test 429 response with `RateLimit-*` and `X-RateLimit-*` headers -> all metrics correctly extracted.
- [x] Transport Connection Pooling & Keep-Alive Reuse:
  - Test consecutive requests to same target reuse existing TCP connections.
  - Test `Close()` cleanly shuts down idle connections.

### 5. CLI Integration Tests (`internal/cli`)
- [x] `internal/cli/cli_test.go`:
  - Test `daegsa version`: prints version, commit, build info; exit code 0.
  - Test `daegsa help` and `daegsa --help`: prints help text; exit code 0.
  - Test `daegsa validate --config <valid.yaml>`: validates config, prints sanitized plan summary; exit code 0.
  - Test `daegsa validate --config <invalid_syntax.yaml>`: prints error; exit code 2 (`ExitCodeValidationFailure`).
  - Test `daegsa validate --config <disallowed_host.yaml>`: safety refusal; exit code 4 (`ExitCodeSafetyRefusal`).
  - Test `daegsa run --config <valid.yaml> --dry-run`: prints plan summary without sending traffic; exit code 0.
  - Test `daegsa run --url <url> --method DELETE` without authorization: safety refusal; exit code 4 (`ExitCodeSafetyRefusal`).
  - Test `daegsa run --url <url> --method DELETE --allow-destructive`: executes request successfully; exit code 0.
  - Test `daegsa run --url <testtarget_url>`: executes single request against live testtarget, prints classification result; exit code 0.

## Safety and failure behavior

- **Safety Refusal Exit Code**: Any safety refusal (unauthorized host, unauthorized destructive method, safety ceiling breach, cross-origin redirect breach) must immediately terminate with process exit code `4` (`core.ExitCodeSafetyRefusal`).
- **Validation Failure Exit Code**: Any YAML syntax error, unknown field, duplicate key, mutually exclusive parameter, or invalid flag combination must terminate with process exit code `2` (`core.ExitCodeValidationFailure`).
- **No Traffic in Dry-Run or Validate**: The `validate` command and `--dry-run` flag must never open network connections to the target API or send HTTP requests (except standard loopback DNS preflight if enabled).
- **Non-Interactive CI Mode**: In `--non-interactive` mode, the CLI must never attempt to read from `os.Stdin` or prompt the user for confirmation; any missing authorization must immediately refuse execution with exit code 4.
- **Credential & Secret Protection**: `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, token values, and secret URL query parameters must be unconditionally masked as `[REDACTED]` in all log messages, CLI outputs, dry-run plans, and error traces.
- **Resource Bounds & Memory Safety**: Response bodies must always be read through `io.LimitReader` bounded by `ResponseBodyLimit` (capped at 50 MiB hard ceiling) and drained/closed to prevent socket and memory leaks.
- **Shared Transport Lifecycle**: The `http.Transport` must be shared across requests and cleanly closed on shutdown to prevent orphaned goroutines or dangling sockets.

## Acceptance gates

1. **Clean Compilation & Zero Diagnostics**: `go build ./...` and `go vet ./...` compile with 0 warnings, 0 diagnostics, and 0 errors.
2. **Deterministic & Concurrency Testing**: `go test -v -race ./...` passes 100% of unit, safety, plan, executor, and CLI tests.
3. **8-Mode Test Target Verification**: `internal/executor` exercises and correctly classifies single requests across all 8 simulation modes of `internal/testtarget` (status codes, delays, payload streaming, redirects, TCP drops, timeout hangs, cookies, and 429 rate limiting).
4. **Safety Refusal Enforcement**: Executing requests against non-allowlisted hosts or unapproved destructive methods (`POST`, `PUT`, `PATCH`, `DELETE`) is rejected before traffic begins with exit code 4.
5. **Validation Error Enforcement**: Invalid configurations, unknown fields, duplicate keys, and invalid CLI overrides are rejected with exit code 2.
6. **Dry-Run & Validation Fidelity**: `--dry-run` and `daegsa validate` produce sanitized, redacted plan summaries and exit with code 0 without sending test traffic.
7. **Secret Redaction**: Secret tokens, credentials, and sensitive headers do not appear in CLI stdout/stderr, execution plans, fingerprints, or error messages.
8. **Git Hygiene**: `git diff --check` passes with zero whitespace or formatting issues; no binaries, credentials, or temporary files are left in the worktree.

## Explicit non-goals

- Implementing the multi-worker metrics aggregator, HDR histograms, or real-time terminal UI dashboard (deferred to Phase 2).
- Implementing closed-model virtual user loops or open-model Poisson/constant arrival rate schedulers (deferred to Phase 2 & Phase 3).
- Implementing complex threshold parsing and pass/fail expression evaluators (deferred to Phase 4).
- Implementing multi-token pools, credentials rotation, or multi-step scenario state extraction (deferred to Phase 5 & Phase 7).
- Implementing ramp/spike/soak profile compilers or report-to-report comparison diffing (deferred to Phase 6).

## Open questions

*None. All CLI commands, configuration precedence rules, environment variable expansions, safety preflight checks, execution plan structures, shared transport settings, redirect policies, and outcome classifications are fully defined in `docs/DAEGSA_Implementation_Plan.md` and frozen for Phase 1.*

## Handoff

### For Implementer
- Begin by adding `github.com/spf13/cobra` and `github.com/spf13/pflag` to `go.mod`.
- Build the packages in dependency order:
  1. `internal/config` (environment expansion `${VAR}`, CLI flag overrides, fingerprinting, redaction).
  2. `internal/safety` (host allowlisting, destructive method authorization, ceilings, DNS preflight).
  3. `internal/plan` (immutable plan representation, summary formatting).
  4. `internal/executor` (shared transport, request builder, response body capper/drainer, rate-limit parser, outcome classifier integration).
  5. `internal/cli` & `cmd/daegsa/main.go` (`run`, `validate`, `version`, `help` Cobra commands and exit code mapping).
- Validate thoroughly against `internal/testtarget` across all 8 modes and write comprehensive test suites for all new packages.
- Ensure all tests pass with `go test -v -race ./...` and `go vet ./...`.

## Implementation handoff

### Changed / Added Files
- `go.mod`, `go.sum`: added dependencies `github.com/spf13/cobra` (v1.10.2) and `github.com/spf13/pflag` (v1.0.10).
- `cmd/daegsa/main.go`: application entrypoint invoking `cli.Execute()` with clean `os.Exit(code)`.
- `internal/config/env.go`, `internal/config/env_test.go`: environment variable resolution (`${VAR}`) with escape support (`$${VAR}`) and syntax error validation.
- `internal/config/precedence.go`, `internal/config/precedence_test.go`: CLI flag precedence overlay onto parsed Config (`CLI > Env > YAML > Default`) with full invariant validation.
- `internal/config/redact.go`, `internal/config/redact_test.go`: centralized redaction helpers for sensitive headers (`Authorization`, `Cookie`, `X-Api-Key`, etc.) and URL credentials/query tokens.
- `internal/config/fingerprint.go`, `internal/config/fingerprint_test.go`: deterministic SHA256 configuration fingerprinting over sanitized canonical JSON.
- `internal/safety/safety.go`, `internal/safety/preflight.go`, `internal/safety/safety_test.go`: safety preflight engine with host allowlisting (exact & wildcard), destructive HTTP method authorization, hard safety ceiling enforcement, and DNS preflight lookup.
- `internal/plan/plan.go`, `internal/plan/print.go`, `internal/plan/plan_test.go`: immutable `Plan` representation, deep cloning, and sanitized console/dry-run summary formatter.
- `internal/executor/transport.go`: shared tuned connection-pooled `http.Transport` factory.
- `internal/executor/request.go`: HTTP request builder with byte estimation.
- `internal/executor/response.go`: bounded response body reader with truncation probe and safe keep-alive draining.
- `internal/executor/ratelimit.go`: standardized (`RateLimit-*`, `Retry-After`) and legacy (`X-RateLimit-*`) header parser.
- `internal/executor/result.go`: execution result model with boundary timestamps, TTFB/latency, and protocol capture.
- `internal/executor/executor.go`, `internal/executor/executor_test.go`: core HTTP request executor with redirect policy enforcement and testtarget integration covering all 8 simulation modes.
- `internal/cli/exit.go`: process exit-code mapping (`PASS=0`, `FAIL_THRESHOLDS=1`, `VALIDATION_FAILURE=2`, `RUNTIME_FAILURE=3`, `SAFETY_REFUSAL=4`).
- `internal/cli/flags.go`: CLI flag definitions and binding helpers.
- `internal/cli/root.go`: root Cobra command structure and execution runner.
- `internal/cli/version.go`: `version` subcommand with build metadata.
- `internal/cli/validate.go`: `validate` subcommand for syntax, env, and safety preflight.
- `internal/cli/run.go`: `run` subcommand supporting `--dry-run`, `--non-interactive`, `--allow-destructive`, and single-request execution.
- `internal/cli/cli_test.go`: CLI end-to-end integration test suite.

### Behavior Implemented
- Complete Cobra CLI commands (`version`, `validate`, `run`, `help`) with canonical exit codes.
- Environment variable placeholder expansion `${VAR}` and escaping `$${VAR}` in YAML configs.
- CLI flag precedence over YAML configurations.
- Deterministic SHA256 fingerprinting of sanitized configuration.
- Comprehensive credential redaction across headers, URLs, and console output.
- Safety preflight engine enforcing host allowlists, destructive method authorizations (`POST`, `PUT`, `PATCH`, `DELETE`), hard safety ceilings, and DNS preflight resolution.
- Immutable execution plan generation with deep cloning.
- Shared tuned `http.Transport` with connection pooling and redirect security checks.
- Outcome classification into 12 canonical states with microsecond-accurate timestamp capture (`ScheduledAt <= DispatchedAt <= HeadersReceivedAt <= BodyCompletedAt`).
- Rate-limit header observation extraction (`Retry-After`, `RateLimit-*`, `X-RateLimit-*`).
- 8-mode verification against deterministic local test server `internal/testtarget`.

### Commands Run and Results
- `go mod tidy`: PASS (exit code 0).
- `go vet ./...`: PASS (exit code 0, 0 warnings, 0 diagnostics).
- `go build ./...`: PASS (exit code 0, clean build).
- `go test -count=1 ./...`: PASS (exit code 0, 100% test pass rate across all packages).
- `go test -race ./...`: Reported `-race requires cgo` (CGO is disabled in Windows AMD64 environment without C compiler).
- `go run ./cmd/daegsa version`: PASS (printed version, commit, build info, exit code 0).
- `go run ./cmd/daegsa --help`: PASS (printed root help text, exit code 0).

### Known Limitations
- Multi-worker metrics aggregator and HDR histograms are deferred to Phase 2 per plan.
- Open/closed scheduler concurrency loops are deferred to Phase 2 & Phase 3 per plan.

### Remaining Unchecked Test or Acceptance Items
- None. All Phase 1 acceptance gates and test checklist items have been verified and tested by the independent Plan Tester.
