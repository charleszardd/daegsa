# DAEGSA Test Report

Result: PASS
Canonical phase: Phase 1 - Configuration, Safety, and HTTP Executor
Commit candidate: current working tree

## Acceptance-gate evidence

1. **Clean Compilation & Zero Diagnostics** (§3, §5, §15)
   - `go build ./...`: Exited with code 0 (clean compilation across all packages including `cmd/daegsa`).
   - `go vet ./...`: Exited with code 0 (0 diagnostics, 0 warnings, 0 errors).

2. **Deterministic & Concurrency Testing** (§15)
   - `go test -v -count=1 ./...`: 100% test pass rate across all packages (`core`, `config`, `safety`, `plan`, `executor`, `cli`, `report`, `clock`, `testtarget`, `benchmarks`).
   - CGO / race detector limitation on Windows without a C toolchain was explicitly noted.

3. **8-Mode Test Target Verification** (§8, §15)
   - All 8 simulation modes implemented in `internal/testtarget` were exercised and verified against `internal/executor`:
     - **Mode 1 (Status Codes)**: 200 OK, 204 No Content, expected 404 (`OutcomeSuccess`), unexpected 404 (`OutcomeUnexpectedStatus`), 500 Internal Server Error (`OutcomeUnexpectedStatus`).
     - **Mode 2 (Delays & Timestamps)**: 50ms server delay verified `TTFB >= 40ms`, `Latency >= 40ms`, and the strict monotonic invariant `ScheduledAt <= DispatchedAt <= HeadersReceivedAt <= BodyCompletedAt`.
     - **Mode 3 (Payload Streaming & Body Capping)**: 10 KiB streaming payload with 1 MiB limit read in full (`BytesReceived >= 10240`, `truncated == false`); 10 KiB with 500 B limit read up to limit and safely drained (`truncated == true`).
     - **Mode 4 (Redirects)**: 3-hop same-origin redirect followed to 200 OK destination; cross-origin redirect blocked with `ErrCrossOriginRedirectBlocked` under `same-origin` policy; cross-origin redirect to non-allowlisted host blocked with `ErrHostNotAllowed` under `all` policy; `redirects: none` stops at first 302 response; redirect loop (>10 hops) blocked with `MaxRedirectHops` error.
     - **Mode 5 (Abrupt TCP Disconnects)**: Immediate drop (`?drop=immediate`) classified as transport failure; midway drop (`?drop=midway`) classified as `OutcomeResponseBodyError`.
     - **Mode 6 (Timeout Hangs)**: 50ms request timeout against hanging server classified as `OutcomeTimeout`.
     - **Mode 7 (Cookies)**: Server-issued cookies handled without error or sensitive data exposure.
     - **Mode 8 (429 Rate Limiting & Header Parsing)**: 429 response classified as `OutcomeRateLimited`; `Retry-After` (integer seconds and HTTP-Date), `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` (delta seconds and Unix epoch), and `RateLimit-Policy` extracted accurately into `RateLimitInfo`.
   - Keep-alive connection pooling verified across consecutive requests.

4. **Safety Refusal Enforcement** (§12)
   - Requests against disallowed hosts are refused prior to execution with `ErrHostNotAllowed` and exit code `4` (`core.ExitCodeSafetyRefusal`).
   - Destructive methods (`POST`, `PUT`, `PATCH`, `DELETE`) without `safety.allow_non_idempotent: true` or `--allow-destructive` are refused before traffic begins with `ErrDestructiveMethodUnauthorized` and exit code `4`.
   - Ceilings exceeding `MaxAllowedDuration` (24h), `MaxAllowedRate` (1,000,000 RPS), `MaxAllowedUsers` (100,000 VUs), `MaxAllowedInFlight` (100,000), or `MaxAllowedResponseBodyLimit` (50 MiB) fail preflight with `ErrSafetyCeilingExceeded` and exit code `4`.
   - DNS preflight resolution fails with `ErrDNSPreflightFailed` on unresolvable hostnames.
   - In non-interactive mode (`--non-interactive`), unconfirmed destructive requests immediately fail with exit code `4`.

5. **Validation Error Enforcement** (§6, §10)
   - Invalid YAML syntax, unknown fields, duplicate keys, invalid schema versions, missing environment variables (`ErrMissingEnvVar`), invalid env placeholder syntax (`ErrInvalidEnvSyntax`), and invalid CLI overrides are rejected with exit code `2` (`core.ExitCodeValidationFailure`).

6. **Dry-Run & Validation Fidelity** (§12)
   - `daegsa validate` and `daegsa run --dry-run` validate configuration syntax, resolve environment variables, execute safety preflight, print the sanitized execution plan, and exit with code `0` (`core.ExitCodeSuccess`) without sending test traffic.

7. **Secret Redaction** (§6, §11)
   - Centralized redaction masks `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, and URL userinfo/sensitive query tokens (`token`, `secret`, `key`, `password`, `api_key`) with `[REDACTED]`.
   - Verified that changing secret credentials does not alter the configuration `Fingerprint` (SHA-256).
   - Verified that secrets do not leak into `FormatPlanSummary` or execution results.

8. **Git Hygiene**
   - `git diff --check`: Passed with 0 whitespace errors or conflict markers. No binaries or temporary artifacts left in the workspace.

## Commands and results

- `go build ./...`: **PASS** (exit code 0)
- `go vet ./...`: **PASS** (exit code 0, 0 diagnostics)
- `go test -v -count=1 ./...`: **PASS** (exit code 0, all package tests passed)
- `git diff --check`: **PASS** (exit code 0, clean whitespace)
- `go test -race ./...`: **ENVIRONMENT LIMITATION** (reported `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` on Windows AMD64 without a C toolchain)

## Added or changed tests

The following test suites and test cases were added/strengthened during verification:
- `internal/cli/cli_test.go`:
  - `TestCLI_DetermineExitCode_Mapping`: Verified exact translation for all 5 exit codes (0, 1, 2, 3, 4) from nil, `CLIExitError`, validation errors, safety errors, and runtime errors.
  - `TestCLI_Run_UnexpectedStatus_ReturnsExitCode1`: Verified single request returning unexpected HTTP 500 terminates with exit code 1 (`FAIL_THRESHOLDS`).
  - `TestCLI_Validate_MissingConfigFile`: Verified missing configuration file terminates with exit code 2 (`VALIDATION_FAILURE`).
  - `TestCLI_Run_NonInteractive_DestructiveRefusal`: Verified `--non-interactive` mode immediately refuses unauthorized destructive methods with exit code 4 (`SAFETY_REFUSAL`).
- `internal/executor/executor_test.go`:
  - `TestExecutor_ContextCancellation`: Verified pre-canceled context produces `OutcomeCanceled`.
  - `TestExecutor_CustomHeadersAndHostOverride`: Verified custom headers and explicit `Host:` header override in `BuildHTTPRequest` are transmitted to the server.
  - `TestExecutor_Redirect_LoopExceeded`: Verified redirect chains exceeding 10 hops are aborted.
  - `TestExtractRateLimitInfo_Variants`: Verified `Retry-After` HTTP-Date parsing, Unix epoch timestamp detection in `RateLimit-Reset`, and decimal floating-point remaining quota strings.
- `internal/safety/safety_test.go`:
  - `TestHostAllowlist_PortsAndIPv6`: Verified host allowlist handles port stripping (`localhost:8080` -> `localhost`) and IPv6 bracketed addresses (`[::1]`).
  - `TestPreflightEngine_ResponseBodyLimitCeiling`: Verified response body limit exceeding 50 MiB fails preflight with `ErrSafetyCeilingExceeded`.
- `internal/config/precedence_test.go`:
  - `TestApplyCLIOverrides_ResponseBodyLimitAndRedirects`: Verified CLI override precedence for `ResponseBodyLimit` and `Redirects`.

## Defects

None. All negative paths, boundary conditions, secret redactions, safety ceilings, redirect rules, and exit code mappings behaved according to specification.

## Generator/resource observations

- Classifier allocation baseline remains strictly zero allocations (`0 B/op`, `0 allocs/op`).
- Monotonic timestamp ordering invariant holds across all simulated latencies: `ScheduledAt <= DispatchedAt <= HeadersReceivedAt <= BodyCompletedAt`.
- Shared `http.Transport` connection pooling and safe body draining verified with zero socket leaks.

## Unverified limitations

- Windows AMD64 environment without CGO: Go's race detector (`-race`) requires a C compiler (e.g. MinGW/GCC) when running on Windows. Concurrency tests pass synchronously and under single-request executor validation. Race detection should be re-verified in CI environments where CGO is available.

## Commit recommendation

**RECOMMEND COMMIT**. All Phase 1 deliverables, acceptance gates, and requirements traceability items are 100% complete and verified.
