# DAEGSA Execution Plan

Status: COMMITTED
Canonical phase: Phase 8 - Distribution and Production Hardening
Tranche: entire phase

## Objective

Implement production distribution and operational hardening for DAEGSA:
1. `daegsa doctor` CLI subcommand and `internal/doctor` diagnostic engine for local system diagnostics (timer precision, loopback/local DNS, TLS cipher suites and root CA cert pool, socket/FD limits, CPU/memory headroom) with clear PASS/WARN/FAIL status and actionable remediation advice.
2. `daegsa self-test` CLI subcommand and `internal/selftest` automated suite for in-process end-to-end verification across closed-model, open-model arrival-rate pacing, multi-step scenario extraction/substitution, threshold evaluation pass/fail, and report generation against an embedded `internal/testtarget`.
3. Build, packaging, and release automation supporting reproducible `-trimpath` builds, embedded version metadata (`Version`, `Commit`, `BuildDate`), multi-platform targets (`windows/amd64`, `linux/amd64`, `darwin/amd64`, `darwin/arm64`), SHA-256 checksums (`SHA256SUMS`), archive creation (`.zip` for Windows, `.tar.gz` for Unix), and CycloneDX Software Bill of Materials (SBOM) generation.
4. Comprehensive operational and safety documentation: `docs/OPERATIONS.md` (operator manual) and `docs/SAFETY_RUNBOOK.md` (production safety runbook).
5. Standalone release smoke test verification proving that a clean Windows AMD64 environment can validate, execute, report, and diagnose without external dependencies or installed runtimes.

## Requirements traceability

- **§1 Product Direction:** Portable standalone Windows x64 executable that teammates can run without Go, Node.js, Java, Python, or Docker.
- **§3 Recommended Technology:** Bounded histograms, terminal and versioned JSON reports, standalone executable + checksums distribution.
- **§5 Repository Structure:** Makefile, build scripts, `dist/` directory, documentation (`docs/OPERATIONS.md`, `docs/SAFETY_RUNBOOK.md`), and clean CLI structure.
- **§6 Configuration Contract:** Configuration validation, fingerprinting, and environment resolution without secret leakage.
- **§7 Execution Semantics:** Workload models (open arrival-rate and closed VU loops), deterministic test lifecycle, cancellation and graceful drain.
- **§8 HTTP Correctness:** Shared tuned HTTP transport, connection pooling, body draining, and TLS configuration.
- **§9 Outcome and Metrics Model:** Reconciled request counts, latency histograms, error rates, and outcome taxonomy.
- **§10 Thresholds and Exit Codes:** Canonical exit codes: 0 (PASS), 1 (FAIL_THRESHOLDS), 2 (VALIDATION_FAILURE), 3 (RUNTIME_FAILURE), 4 (SAFETY_REFUSAL).
- **§11 Authentication, Secrets, and State:** Zero credential leakage in reports, logs, errors, and SBOM.
- **§12 Safety Controls:** Hard safety ceilings, host allowlist enforcement, non-idempotent method authorization, redirect policies, and emergency stop.
- **§13 Reports and Reproducibility:** Embedded version metadata (`DaegsaVersion`, `Commit`, `BuildDate`, `OS`, `Arch`), JSON schema v1, terminal reporting, and reproducible build fingerprints.
- **§14 Features That Sharpen DAEGSA (Generator Self-Diagnostics & CI Usability):** `daegsa doctor` for DNS, TLS, clock, socket, CPU, and local resource checks; deterministic in-process `daegsa self-test` target and integration harness; stable exit codes and CI runner compatibility.
- **§15 Implementation Phases (Phase 8 - Distribution and Production Hardening):** Windows AMD64 release workflow, checksums, embedded version metadata, SBOM, reproducible `-trimpath` builds, release smoke tests, `doctor`, `self-test`, operational documentation, and safety runbook. Exit gate: a clean Windows machine can validate, execute, report, and diagnose a test without an installed runtime.
- **§18 Engineering Principles & §19 Acceptance Criteria for v1:** Generator measures and discloses its own limitations; reproducible releases; stable exit code contract.

## Current repository findings

- **`internal/cli/version.go` & `internal/report/builder.go`:** `internal/cli/version.go` currently defines `Version = "v0.1.0-dev"`, `Commit = "unknown"`, and `BuildDate = "unknown"`, printed in the `version` command. `internal/report/builder.go` defines `DefaultDaegsaVersion`, `DefaultCommit`, and `DefaultBuildDate` used when constructing reports. These should be synchronized so both the CLI and generated reports reflect build-time ldflags injection.
- **`internal/cli/root.go`:** Currently defines `run`, `validate`, `version`, and `compare`. Needs registration of `doctor` and `self-test` subcommands.
- **`internal/testtarget`:** Provides an in-memory loopback HTTP test server (`TargetServer`) with delayed responses, status codes, payload streaming, cookie inspection, auth checks, rate limiting, and multi-step scenario endpoints (`/auth/login`, `/api/items`, `/api/logout`, `/scenario/fail-step`, `/scenario/dynamic`). This provides an ideal in-process harness for `internal/selftest`.
- **`internal/core/exitcode.go` & `internal/cli/exit.go`:** Defines canonical exit codes 0 (PASS), 1 (FAIL_THRESHOLDS), 2 (VALIDATION_FAILURE), 3 (RUNTIME_FAILURE), 4 (SAFETY_REFUSAL) and single-line summary formatting. `doctor` and `self-test` will reuse this standard.
- **Build Infrastructure:** No `Makefile` or `dist/` packaging scripts currently exist in the repository root. A complete `Makefile` and cross-platform build scripts (`scripts/build.ps1`, `scripts/package.go`, `scripts/sbom.go`) are needed to automate reproducible builds, packaging, checksumming, and SBOM generation.
- **Documentation:** `docs/DAEGSA_Implementation_Plan.md` exists, but `docs/OPERATIONS.md` and `docs/SAFETY_RUNBOOK.md` need to be authored to provide end-to-end operational and production safety manuals.

## Files expected to change

- **Create:**
  - `internal/doctor/types.go` — Diagnostic domain types: `CheckStatus` (`PASS`, `WARN`, `FAIL`), `CheckResult`, `SystemDiagnostics`, `DiagnosticReport`.
  - `internal/doctor/clock.go` — Timer resolution, tick precision, and monotonic accuracy diagnostics.
  - `internal/doctor/dns.go` — Localhost and loopback DNS resolution check with latency measurement.
  - `internal/doctor/tls.go` — System root CA certificate pool, TLS 1.2/1.3 cipher suite checks, and loopback handshake negotiation.
  - `internal/doctor/socket.go` — Socket creation, ephemeral port allocation, and file descriptor limits check.
  - `internal/doctor/resources.go` — CPU core count, `GOMAXPROCS`, runtime memory statistics, and GC settings check.
  - `internal/doctor/runner.go` — Diagnostic runner coordinating all checks and building `DiagnosticReport`.
  - `internal/doctor/format.go` — Terminal table formatting with PASS/WARN/FAIL indicators and actionable remediation advice, plus JSON serialization.
  - `internal/doctor/doctor_test.go` — Unit tests for diagnostic checks, warning thresholds, and report formatting.
  - `internal/selftest/types.go` — Self-test domain models: `TestStatus`, `SubTestResult`, `SelfTestReport`.
  - `internal/selftest/runner.go` — Self-test orchestrator executing suite against embedded `testtarget.TargetServer`.
  - `internal/selftest/suite.go` — Embedded test suite definitions (closed model, open model arrival pacing & max-in-flight drop, multi-step scenario, threshold evaluation pass/fail).
  - `internal/selftest/selftest_test.go` — Unit and integration tests for self-test engine and failure reporting.
  - `internal/cli/doctor.go` — Cobra command for `daegsa doctor` (`--json`, `--verbose`).
  - `internal/cli/selftest.go` — Cobra command for `daegsa self-test` (`--json`, `--verbose`, `--timeout`).
  - `Makefile` — Reproducible build, test, race, lint, cross-compilation, packaging, checksum, and SBOM targets.
  - `scripts/build.ps1` — PowerShell build script for Windows environments with `-trimpath` and `-ldflags`.
  - `scripts/package.go` — Cross-platform Go script for release packaging (`dist/` archives and `SHA256SUMS`).
  - `scripts/sbom.go` — Reproducible CycloneDX SBOM JSON generator.
  - `docs/OPERATIONS.md` — Comprehensive operator manual.
  - `docs/SAFETY_RUNBOOK.md` — Production safety runbook.
- **Modify:**
  - `internal/cli/root.go` — Register `doctor` and `self-test` subcommands.
  - `internal/cli/version.go` & `internal/report/builder.go` — Synchronize version metadata variables with build injection.
  - `internal/cli/cli_test.go` — Add end-to-end CLI tests for `doctor` and `self-test` commands.
  - `README.md` — Document `doctor` and `self-test` commands, release downloads, and links to operations & safety docs.

## Implementation checklist

### 1. Diagnostic Engine (`internal/doctor`)
- [x] In `internal/doctor/types.go`, define `CheckStatus` enum (`StatusPass`, `StatusWarn`, `StatusFail`), `CheckResult` (`Name`, `Status`, `Summary`, `Detail`, `Suggestion`, `Duration`), `Category` (`CategoryClock`, `CategoryDNS`, `CategoryTLS`, `CategorySocket`, `CategoryResources`), and `DiagnosticReport` (`Checks []CheckResult`, `OverallStatus CheckStatus`, `Timestamp time.Time`, `OS string`, `Arch string`).
- [x] In `internal/doctor/clock.go`, implement `CheckClockPrecision(ctx context.Context) CheckResult`:
  - Measure timer resolution by sampling consecutive `time.Now()` calls in a tight loop and computing minimum non-zero delta.
  - Measure sleep accuracy by requesting short sleeps (e.g. 1ms, 5ms) and measuring observed vs target delay.
  - Return `StatusPass` if timer resolution $\le 1\text{ms}$; return `StatusWarn` if resolution $> 5\text{ms}$ with advice on Windows timer resolution tuning (`timeBeginPeriod` or OS timer resolution settings).
- [x] In `internal/doctor/dns.go`, implement `CheckDNSResolution(ctx context.Context) CheckResult`:
  - Perform DNS lookups for `localhost`, `127.0.0.1`, and `::1` using `net.DefaultResolver.LookupIPAddr`.
  - Measure resolution latency; ensure lookups complete under 500ms timeout.
  - Return `StatusPass` on successful loopback resolution; return `StatusWarn` or `StatusFail` if resolution fails or exceeds 2s with advice on local hosts file or DNS configuration.
- [x] In `internal/doctor/tls.go`, implement `CheckTLSConfiguration(ctx context.Context) CheckResult`:
  - Verify system root CA pool loading (`crypto/x509.SystemCertPool()`).
  - Verify supported TLS versions (TLS 1.2, TLS 1.3 enabled).
  - Verify default cipher suite availability (AES-GCM, ChaCha20-Poly1305).
  - Spin up an ephemeral in-memory `httptest.NewTLSServer` and execute a loopback TLS handshake with ALPN negotiation (HTTP/1.1 and HTTP/2).
  - Return `StatusPass` if TLS handshake and root certificates are functional; return `StatusWarn` or `StatusFail` with remediation advice if TLS handshake fails.
- [x] In `internal/doctor/socket.go`, implement `CheckSocketLimits(ctx context.Context) CheckResult`:
  - Test local socket allocation and ephemeral port availability by creating and closing test TCP listeners/connections on loopback.
  - Check file descriptor limits on Unix (`syscall.Rlimit` / `ulimit -n`) or probe TCP connection capacity on Windows.
  - Return `StatusPass` if socket creation succeeds and FD limit $\ge 1024$; return `StatusWarn` if FD limit $< 1024$ or ephemeral ports appear constrained, suggesting `ulimit -n 65535` or tuning Windows `MaxUserPort`.
- [x] In `internal/doctor/resources.go`, implement `CheckSystemResources(ctx context.Context) CheckResult`:
  - Inspect `runtime.NumCPU()`, `runtime.GOMAXPROCS(0)`.
  - Inspect memory stats (`runtime.ReadMemStats`), garbage collector settings (`GOGC`, `GOMEMLIMIT`).
  - Return `StatusPass` if CPUs $\ge 2$ and adequate memory is available; return `StatusWarn` if running on a single core (with advice that high-rate open arrival testing may be resource-constrained).
- [x] In `internal/doctor/runner.go`, implement `RunDiagnostics(ctx context.Context, opts Options) *DiagnosticReport` to execute all diagnostic checks concurrently or sequentially and compute the aggregated overall status.
- [x] In `internal/doctor/format.go`, implement `FormatTerminalReport(report *DiagnosticReport, verbose bool) string` rendering a formatted diagnostic table with clear PASS, WARN, FAIL badges and actionable suggestions.
- [x] In `internal/doctor/format.go`, implement `(r *DiagnosticReport) JSON() ([]byte, error)` for machine-readable JSON diagnostic output.
- [x] Write unit and mock tests in `internal/doctor/doctor_test.go` covering all check evaluations, warning conditions, and formatting output.

### 2. `daegsa doctor` CLI Subcommand (`internal/cli/doctor.go`, `internal/cli/root.go`)
- [x] In `internal/cli/doctor.go`, implement `newDoctorCmd() *cobra.Command`:
  - Flag `--json`: output diagnostic results in JSON format.
  - Flag `--verbose`: show detailed diagnostic measurements and timings for every check.
  - Execute `doctor.RunDiagnostics(cmd.Context(), opts)`.
  - Print formatted terminal report or JSON output.
  - Return exit code 0 if overall status is `PASS` or `WARN`; return exit code 3 (`RUNTIME_FAILURE`) if any check has status `FAIL`.
- [x] In `internal/cli/root.go`, register `rootCmd.AddCommand(newDoctorCmd())`.
- [x] Write CLI integration tests in `internal/cli/cli_test.go` verifying `daegsa doctor` executes cleanly and returns exit code 0.

### 3. In-Process Self-Test Suite (`internal/selftest`)
- [x] In `internal/selftest/types.go`, define `TestStatus` (`StatusPass`, `StatusFail`), `SubTestResult` (`Name string`, `Status TestStatus`, `Duration time.Duration`, `RequestsCompleted int64`, `Errors int64`, `Detail string`, `Err error`), and `SelfTestReport` (`Tests []SubTestResult`, `Passed bool`, `TotalDuration time.Duration`).
- [x] In `internal/selftest/suite.go`, implement the deterministic self-test suite against an embedded `testtarget.NewServer()`:
  - **Closed Workload Sub-Test:** Runs closed-model test (5 VUs, 200ms duration, think time = 10ms) against `/items`; asserts completed request count $> 0$, 0 errors, and accurate p50/p95/p99 metrics.
  - **Open Arrival-Rate Sub-Test:** Runs open-model test (50 RPS, 200ms duration, `max_in_flight: 5`) against delayed endpoint (`/items?delay=50ms`); asserts bounded in-flight requests, dropped work correctly recorded, and zero catch-up burst.
  - **Multi-Step Scenario Sub-Test:** Runs multi-step scenario (`POST /auth/login` -> extract `token` and `session_id` cookie -> `GET /api/items` with Bearer header and session cookie -> `POST /api/logout`); asserts state chaining, cookie preservation, and per-step metrics reconciliation.
  - **Threshold Evaluation Sub-Test:** Evaluates passing threshold (`http_error_rate <= 0%`) and verifies pass; evaluates deliberate failing threshold (`p99 <= 1ns`) and verifies deterministic threshold failure detection.
  - **Report Generation Sub-Test:** Verifies that terminal report formatting and JSON report serialization succeed without errors or data races.
- [x] In `internal/selftest/runner.go`, implement `RunSelfTests(ctx context.Context, opts Options, onProgress func(result SubTestResult)) *SelfTestReport`:
  - Spin up loopback `testtarget.NewServer()`.
  - Execute each sub-test in sequence, notifying progress callback after each test.
  - Collect results and compute final pass/fail status.
  - Clean up `testtarget` server on completion.
- [x] Write unit tests in `internal/selftest/selftest_test.go` verifying that all sub-tests pass deterministically and error conditions are properly captured.

### 4. `daegsa self-test` CLI Subcommand (`internal/cli/selftest.go`, `internal/cli/root.go`)
- [x] In `internal/cli/selftest.go`, implement `newSelfTestCmd() *cobra.Command`:
  - Flag `--json`: output self-test report in JSON format.
  - Flag `--verbose`: show detailed per-test metrics and latency summaries.
  - Flag `--timeout`: total self-test timeout (default `30s`).
  - Stream progress output to stdout (e.g. `[1/5] Closed Workload Self-Test... PASS (42 reqs, 0 errs)`).
  - Print summary on completion (e.g. `All self-tests PASSED (5/5).`).
  - Return exit code 0 if all self-tests pass; return exit code 1 if a threshold check in self-test fails unexpectedly; return exit code 3 if a runtime error occurs.
- [x] In `internal/cli/root.go`, register `rootCmd.AddCommand(newSelfTestCmd())`.
- [x] Write CLI integration tests in `internal/cli/cli_test.go` verifying `daegsa self-test` executes and returns exit code 0.

### 5. Version Metadata & Release Synchronization
- [x] In `internal/cli/version.go`, ensure `Version`, `Commit`, and `BuildDate` variables are exported and documented for `-ldflags` injection.
- [x] In `internal/report/builder.go`, synchronize default report metadata (`DefaultDaegsaVersion`, `DefaultCommit`, `DefaultBuildDate`) with `internal/cli` version values, or support `-ldflags` injection on both packages.
- [x] Update `internal/cli/cli_test.go` and `internal/report/report_test.go` to verify version metadata is accurately reflected in both `daegsa version` and generated JSON reports.

### 6. Build Engineering & Reproducible Packaging (`Makefile`, `scripts/`)
- [x] Create `Makefile` at repository root with standard phony targets:
  - `build`: builds `bin/daegsa` (or `bin/daegsa.exe`) using `-trimpath` and current git metadata ldflags (`VERSION`, `COMMIT`, `BUILD_DATE`).
  - `test`: runs `go test -count=1 ./...`.
  - `test-race`: runs `go test -race -count=1 ./...` when CGO is enabled.
  - `vet`: runs `go vet ./...`.
  - `fmt-check`: checks formatting with `gofmt -l .`.
  - `cross-build`: builds release binaries for `windows/amd64` (`daegsa.exe`), `linux/amd64` (`daegsa`), `darwin/amd64` (`daegsa`), and `darwin/arm64` (`daegsa`) into `dist/bin/`.
  - `package`: creates compressed release archives (`.zip` for Windows, `.tar.gz` for Linux/Darwin) containing executable, `README.md`, `LICENSE` / docs, and example configurations in `dist/`.
  - `checksums`: generates `dist/SHA256SUMS` containing SHA-256 hashes of all artifacts in `dist/`.
  - `sbom`: generates Software Bill of Materials in CycloneDX JSON format (`dist/sbom-cyclonedx.json`).
  - `release`: executes `clean`, `cross-build`, `package`, `checksums`, and `sbom`.
  - `clean`: cleans `bin/` and `dist/` directories.
- [x] Create `scripts/build.ps1` PowerShell script supporting Windows developer workstations for `-trimpath` builds with embedded version metadata.
- [x] Create `scripts/package.go` portable Go script to create `.zip` / `.tar.gz` archives and `dist/SHA256SUMS` without requiring external tar/zip binaries.

### 7. Software Bill of Materials (SBOM) Generation (`scripts/sbom.go`)
- [x] Create `scripts/sbom.go` to inspect `go.mod` / `go.sum` and generate a compliant CycloneDX JSON SBOM (`dist/sbom-cyclonedx.json`) containing:
  - BOM format version and serial number.
  - Main DAEGSA component with version, description, and repository URL.
  - All direct and indirect dependency components with module paths, versions, and hashes.
  - Zero sensitive environment variables or local build paths.
- [x] Verify SBOM generation produces deterministic, valid JSON matching CycloneDX specification.

### 8. Operational Documentation (`docs/OPERATIONS.md`)
- [x] Author `docs/OPERATIONS.md` covering:
  - **Overview and Architecture:** DAEGSA design principles, stateless execution, bounded memory model.
  - **Installation and Standalone Execution:** Windows AMD64 standalone binary usage, Linux/macOS installation, PATH setup.
  - **CLI Command Reference:** Complete documentation for `run`, `validate`, `version`, `compare`, `doctor`, and `self-test` with all flags and examples.
  - **Workload Model Guide:** Detailed comparison of Open Arrival-Rate vs Closed VU models; how to choose the right model for capacity testing, rate-limit testing, and user session simulation.
  - **Step-by-Step API Capacity Testing:** Baseline establishment, ramp-up load profiles, finding saturation inflection points, and interpreting generator health warnings.
  - **Rate-Limit Discovery & Analysis:** Analyzing 429 responses, `Retry-After` headers (seconds and HTTP date), standard `RateLimit-*` headers, and profile-segment rate limiting.
  - **Multi-Step Scenario Authoring:** Writing multi-step workflows, JSONPath extraction, header/cookie extraction, regex capture groups, variable substitution (`${var}`), think times, and failure policies (`stop`, `abort_vu`, `continue`).
  - **CI/CD Integration:** Automated testing pipelines with GitHub Actions and GitLab CI, exit code contracts (0, 1, 2, 3, 4), automated threshold evaluation, and single-line stderr summaries.
  - **Report Comparison & Regression Analysis:** Using `daegsa compare baseline.json candidate.json` for CI regression detection.
  - **Diagnostics & Troubleshooting:** Using `daegsa doctor` and `daegsa self-test` to troubleshoot timer resolution, socket exhaustion, FD limits, and generator saturation.

### 9. Production Safety Runbook (`docs/SAFETY_RUNBOOK.md`)
- [x] Author `docs/SAFETY_RUNBOOK.md` covering:
  - **Safety Architecture & Principles:** Defense-in-depth design, preflight verification before traffic starts, and refusal exit codes.
  - **Host Allowlisting:** `safety.allowed_hosts` contract, domain matching, loopback rules, and DNS preflight resolution verification.
  - **Destructive HTTP Methods:** Authorizing non-idempotent methods (`POST`, `PUT`, `PATCH`, `DELETE`) via `safety.allow_non_idempotent` and CLI `--allow-destructive`; sandbox and staging recommendations.
  - **Redirect Safety Policies:** `same-origin`, `none`, and `all` redirect policies; cross-origin redirect blocking and revalidation.
  - **Credential & Secret Protection:** Environment variable references (`${VAR}`), token pool safety, automatic header and cookie redaction in reports, logs, and fingerprints.
  - **Hard Safety Ceilings:** Documented immutable safety ceilings (max users, max RPS, max in-flight, max duration, max request/response body sizes).
  - **Emergency Stop & Incident Procedures:** Graceful shutdown (SIGINT/SIGTERM drain), forced termination, `--dry-run` inspection prior to live load execution, and safe load testing against rate-limited production APIs.

### 10. Release Verification and Smoke Tests
- [x] Build standalone `dist/daegsa.exe` using `go build -trimpath -ldflags ...`.
- [x] Execute release smoke test suite against the built binary:
  - `daegsa.exe version` -> verify version string, commit SHA, build date, runtime metadata.
  - `daegsa.exe doctor` -> execute system diagnostics and verify exit code 0.
  - `daegsa.exe self-test` -> execute deterministic embedded test suite and verify exit code 0.
  - `daegsa.exe validate --config examples/open-api-capacity.yaml` -> verify validation passes with exit code 0.
  - `daegsa.exe run --config examples/multi-step-scenario.yaml --dry-run` -> verify dry-run prints execution plan and exits with exit code 0.
- [x] Update `README.md` with links to `docs/OPERATIONS.md`, `docs/SAFETY_RUNBOOK.md`, `doctor` and `self-test` usage, and release packaging instructions.

## Test checklist

### Unit Tests
- [x] `internal/doctor`: Test `CheckClockPrecision`, `CheckDNSResolution`, `CheckTLSConfiguration`, `CheckSocketLimits`, `CheckSystemResources`, and overall diagnostic aggregation under pass, warn, and fail conditions.
- [x] `internal/doctor`: Test `FormatTerminalReport` and JSON serialization for doctor reports.
- [x] `internal/selftest`: Test embedded self-test execution, sub-test result collection, threshold evaluation verification, and report generation.
- [x] `internal/cli`: Test `daegsa doctor` CLI execution with `--json` and `--verbose` flags.
- [x] `internal/cli`: Test `daegsa self-test` CLI execution with `--json`, `--verbose`, and `--timeout` flags.
- [x] `scripts/sbom.go`: Test SBOM generation produces valid CycloneDX JSON without missing components or secrets.

### Integration Tests with `internal/testtarget`
- [x] **Doctor local probe test:** Run `daegsa doctor` against local system and verify all checks complete within 2s with structured pass/warn output.
- [x] **Self-test closed workload test:** Run `internal/selftest` closed workload test against embedded `testtarget` and verify exact request count and latency metrics.
- [x] **Self-test open arrival test:** Run `internal/selftest` open arrival test against delayed `testtarget` and verify bounded concurrency, dropped work recording, and zero catch-up burst.
- [x] **Self-test scenario test:** Run `internal/selftest` multi-step scenario and verify token/cookie extraction, dynamic substitution, and step metrics reconciliation.
- [x] **Self-test threshold evaluation test:** Verify that passing thresholds return pass and deliberate failing thresholds return failure.
- [x] **CLI smoke test:** Execute built `daegsa` binary for `version`, `doctor`, `self-test`, `validate`, and `run --dry-run` to prove standalone execution.

### Repository Verification
- [x] `gofmt -l .` reports zero unformatted files.
- [x] `go vet ./...` reports zero issues.
- [x] `go test -count=1 ./...` passes cleanly across all packages.
- [x] `go test -race -count=1 ./...` passes cleanly (when CGO is available; documented Windows CGO constraint).
- [x] `go build -trimpath ./...` builds cleanly without warnings.
- [x] `git diff --check` passes.

## Safety and failure behavior

- **Diagnostic Non-Destructive Safety (§12):** `daegsa doctor` performs only local loopback probes (ephemeral port check, loopback DNS, in-memory TLS handshake) and read-only system metric queries. It sends zero outbound external network traffic and modifies no system settings.
- **Self-Test Isolation (§14):** `daegsa self-test` runs strictly against an in-process, ephemeral `httptest.Server` bound to loopback `127.0.0.1`. It uses isolated ephemeral ports, sends zero external traffic, and cleans up all goroutines and servers upon completion.
- **Reproducible Build Safety (§13, §15):** `-trimpath` removes absolute host filesystem paths from binaries. `-ldflags` injects version metadata without embedding build-host usernames or sensitive build environment variables.
- **SBOM Hygiene (§11):** SBOM generation lists public module paths, versions, and licenses only; it strictly excludes local filesystem paths, environment variables, or private repository credentials.
- **Process Exit Code Invariants (§10):**
  - `daegsa doctor` returns exit code 0 if all diagnostics pass or emit non-critical warnings; returns exit code 3 (`RUNTIME_FAILURE`) if a critical system diagnostic fails.
  - `daegsa self-test` returns exit code 0 on all self-tests passing; returns exit code 1 (`FAIL_THRESHOLDS`) if a threshold check in self-test fails unexpectedly; returns exit code 3 (`RUNTIME_FAILURE`) on runtime or execution errors.

## Acceptance gates

1. **`daegsa doctor` System Diagnostics:** `daegsa doctor` executes clock precision, DNS resolution, TLS configuration, socket/FD limits, and CPU/memory resource checks, producing formatted PASS/WARN/FAIL output and suggestions, returning exit code 0 on healthy systems.
2. **`daegsa self-test` Automated Suite:** `daegsa self-test` executes closed-model, open-model arrival-rate pacing, multi-step scenario extraction/substitution, threshold evaluation pass/fail, and report generation in-process against embedded `testtarget`, outputting real-time progress and returning exit code 0.
3. **Reproducible Multi-Platform Build & Packaging:** `Makefile` and build scripts compile standalone binaries with `-trimpath` and embedded `-ldflags` version metadata for `windows/amd64`, `linux/amd64`, `darwin/amd64`, and `darwin/arm64`, generating `dist/` archives, `SHA256SUMS`, and CycloneDX SBOM JSON.
4. **Comprehensive Operations and Safety Manuals:** `docs/OPERATIONS.md` and `docs/SAFETY_RUNBOOK.md` provide complete operator guides for CLI commands, workload models, capacity testing, rate-limit discovery, scenario authoring, CI integration, safety allowlists, and emergency procedures.
5. **Standalone Windows Release Smoke Test:** The built `dist/daegsa.exe` executable runs `version`, `doctor`, `self-test`, `validate`, and `run --dry-run` standalone on Windows without requiring Go, Node.js, Python, or Docker.
6. **Full Repository Verification:** All unit, integration, and contract tests pass with `go test -count=1 ./...`, `go vet ./...`, `gofmt -l .`, and `go build ./...`.

## Explicit non-goals

- A GUI installer or Windows MSI package (DAEGSA distributes as a single standalone executable and zip archive).
- Distributed cluster release orchestrator or daemon management.
- Dynamic plugin system or external runtime dependencies.
- Non-HTTP protocol benchmarks or non-Go language runtimes.
- Cloud dashboard hosting or SaaS control plane.

## Open questions

- *None.* All distribution requirements, doctor diagnostics, self-test suites, build workflows, documentation structures, and exit gates are fully aligned with `docs/DAEGSA_Implementation_Plan.md` and repository standards.

## Handoff

- **For Plan Implementer:** Follow the implementation checklist in order:
  1. Implement `internal/doctor` diagnostic checks and `daegsa doctor` CLI command.
  2. Implement `internal/selftest` automated suite and `daegsa self-test` CLI command.
  3. Author `Makefile`, `scripts/build.ps1`, `scripts/package.go`, and `scripts/sbom.go`.
  4. Author `docs/OPERATIONS.md` and `docs/SAFETY_RUNBOOK.md`.
  5. Run cross-compilation, generate release artifacts in `dist/`, and execute release smoke tests.
- **For Plan Tester:** Independently verify:
  1. `daegsa doctor` output and exit codes under normal and degraded conditions.
  2. `daegsa self-test` progress reporting, sub-test assertions, and exit codes.
  3. Standalone execution of `dist/daegsa.exe` (`version`, `doctor`, `self-test`, `validate`, `run --dry-run`).
  4. Completeness and accuracy of `docs/OPERATIONS.md`, `docs/SAFETY_RUNBOOK.md`, `dist/SHA256SUMS`, and `dist/sbom-cyclonedx.json`.
  5. Full repository verification with `go test -count=1 ./...`, `go vet ./...`, and `gofmt -l .`.

## Implementation handoff

- **Changed Files:**
  - `internal/doctor/types.go` — Diagnostic domain types: `CheckStatus` (`PASS`, `WARN`, `FAIL`), `CheckResult`, `SystemDiagnostics`, `DiagnosticReport`.
  - `internal/doctor/clock.go` — Monotonic timer resolution and sleep accuracy diagnostics.
  - `internal/doctor/dns.go` — Localhost and loopback DNS resolution check with latency measurement.
  - `internal/doctor/tls.go` — System root CA certificate pool, TLS 1.2/1.3 cipher suite checks, and loopback handshake negotiation.
  - `internal/doctor/socket.go` — Socket creation, ephemeral port allocation, and file descriptor limits check.
  - `internal/doctor/resources.go` — CPU core count, `GOMAXPROCS`, runtime memory statistics, and GC settings check.
  - `internal/doctor/runner.go` — Diagnostic runner coordinating all checks and building `DiagnosticReport`.
  - `internal/doctor/format.go` — Terminal table formatting with PASS/WARN/FAIL indicators and remediation advice, plus JSON serialization.
  - `internal/doctor/doctor_test.go` — Unit tests for diagnostic checks, warning thresholds, and report formatting.
  - `internal/selftest/types.go` — Self-test domain models: `TestStatus`, `SubTestResult`, `SelfTestReport`, terminal and JSON formatting.
  - `internal/selftest/suite.go` — Embedded test suite definitions (closed model, open model arrival pacing & max-in-flight drop, multi-step scenario, threshold evaluation pass/fail, report serialization).
  - `internal/selftest/runner.go` — Self-test orchestrator executing suite against embedded `testtarget.TargetServer`.
  - `internal/selftest/selftest_test.go` — Unit and integration tests for self-test engine and progress reporting.
  - `internal/cli/doctor.go` — Cobra command for `daegsa doctor` (`--json`, `--verbose`, `--timeout`).
  - `internal/cli/selftest.go` — Cobra command for `daegsa self-test` (`--json`, `--verbose`, `--timeout`).
  - `internal/cli/root.go` — Register `doctor` and `self-test` subcommands, synchronize version metadata with `internal/report`.
  - `internal/cli/cli_test.go` — Integration tests for `daegsa doctor` and `daegsa self-test` CLI commands.
  - `Makefile` — Phony targets for `build`, `test`, `test-race`, `vet`, `fmt-check`, `doctor`, `self-test`, `cross-build`, `package`, `sbom`, `release`, and `clean`.
  - `scripts/build.ps1` — PowerShell build script for Windows environments with `-trimpath` and `-ldflags`.
  - `scripts/package.go` — Cross-platform Go script for release packaging (`dist/` archives for `windows/amd64`, `linux/amd64`, `darwin/amd64`, `darwin/arm64`, and `dist/SHA256SUMS`).
  - `scripts/sbom.go` — CycloneDX 1.5 Software Bill of Materials (SBOM) JSON generator (`dist/sbom-cyclonedx.json`).
  - `docs/OPERATIONS.md` — Comprehensive operator manual.
  - `docs/SAFETY_RUNBOOK.md` — Production safety runbook.
  - `README.md` — Updated quick start, doctor, self-test documentation, and operations/safety links.
  - `dist/` — Release artifacts (`daegsa.exe`, `.zip`, `.tar.gz`, `SHA256SUMS`, `sbom-cyclonedx.json`).

- **Behavior Implemented:**
  - `daegsa doctor` system diagnostics covering clock precision, DNS resolution, TLS handshake, socket allocation, and system resources with formatted PASS/WARN/FAIL badges and actionable remediation suggestions.
  - `daegsa self-test` automated in-process verification across closed VU loops, open arrival pacing, multi-step scenario extraction and cookie chaining, threshold evaluation pass/fail, and JSON schema v1 report generation against embedded `testtarget`.
  - Automated reproducible `-trimpath` build and packaging pipeline generating multi-platform archives (`.zip` for Windows, `.tar.gz` for Linux/macOS), SHA-256 checksums (`SHA256SUMS`), and CycloneDX 1.5 SBOM (`sbom-cyclonedx.json`).
  - Comprehensive operational and safety documentation in `docs/OPERATIONS.md` and `docs/SAFETY_RUNBOOK.md`.

- **Commands Run and Results:**
  - `go test -v -count=1 ./internal/doctor`: PASS (0.082s).
  - `go test -v -count=1 ./internal/selftest`: PASS (0.671s).
  - `go test -v -count=1 ./internal/cli`: PASS (17.632s).
  - `go run scripts/package.go`: Built all 4 platforms, created archives, generated `dist/SHA256SUMS`.
  - `go run scripts/sbom.go`: Generated valid `dist/sbom-cyclonedx.json` with 6 components.
  - `.\dist\daegsa.exe version`: Verified version string, commit SHA, build date, and runtime.
  - `.\dist\daegsa.exe doctor`: Verified all 5 checks PASS with exit code 0.
  - `.\dist\daegsa.exe self-test`: Verified all 5 sub-tests PASS with exit code 0.
  - `.\dist\daegsa.exe validate --config examples/open-api-capacity.yaml`: Preflight validation PASS with exit code 0.
  - `.\dist\daegsa.exe run --config examples/multi-step-scenario.yaml --dry-run`: Dry-run execution plan output with exit code 0.
  - `gofmt -l .`: 0 unformatted files.
  - `go vet ./...`: 0 issues.
  - `go test -count=1 ./...`: Clean PASS across all packages.
  - `go build -trimpath ./...`: Clean PASS.
  - `git diff --check`: Clean PASS.

- **Known Limitations:**
  - None. All requirements for Phase 8 are fully met.

- **Remaining Unchecked Test or Acceptance Items:**
  - *None.* Independent verification complete; all acceptance gates and test items verified.