# DAEGSA Execution Plan
Status: COMMITTED
Canonical phase: Phase 5 - Authentication and Secret Handling
Tranche: entire phase

## Objective

Build and integrate the authentication and secret handling subsystem (`internal/auth`), provide deterministic token pool providers for both closed and open workload models, support per-VU cookie jar isolation for closed-model testing, harden centralized secret redaction across all operational outputs, logs, dry-run plans, ANSI terminal tables, and JSON v1 reports, and verify zero credential leakage via contract tests against `internal/testtarget`.

---

## Requirements Traceability

- **`docs/DAEGSA_Implementation_Plan.md` §3 (System Architecture)**:
  - Define `internal/auth` package boundary for static authentication, token pools, per-VU sessions, and credential handling.
- **`docs/DAEGSA_Implementation_Plan.md` §4 (High-Level Architecture)**:
  - Position `Auth + Per-VU State + Request Builder` directly in the request pipeline above the shared tuned HTTP transport.
- **`docs/DAEGSA_Implementation_Plan.md` §6 (Configuration Contract)**:
  - Add `auth` section to v1 YAML schema: `type`, `token`, `header_name`, `username`, `password`, `token_pool`, and `cookie_jar`.
  - Expand environment placeholders (`${API_TOKEN}`) without mutating source files.
  - Never include resolved secrets in errors, logs, configuration fingerprints, terminal reports, or JSON outputs.
- **`docs/DAEGSA_Implementation_Plan.md` §7 (Execution Semantics)**:
  - Closed scheduler: maintain independent virtual user loops and per-VU cookie jars when enabled; assign tokens deterministically ($VU_i \to \text{token}_{i \pmod N}$).
  - Open scheduler: assign tokens deterministically to stable worker lanes ($Lane_j \to \text{token}_{j \pmod N}$) rather than completion order.
- **`docs/DAEGSA_Implementation_Plan.md` §8 (HTTP Correctness)**:
  - Reuse one shared, tuned `http.Transport` across all requests and virtual users.
  - Use per-VU `http.CookieJar` instances or per-VU `http.Client` wrappers sharing the single `http.Transport` when session isolation is enabled.
- **`docs/DAEGSA_Implementation_Plan.md` §9 (Outcome and Metrics Model)**:
  - Bound memory and label cardinality; never use secrets, auth tokens, or arbitrary header values as metric labels or error samples.
- **`docs/DAEGSA_Implementation_Plan.md` §11 (Authentication, Secrets, and State)**:
  - Static Bearer, custom header, and Basic auth support.
  - Deterministic token pool assignment for open and closed workloads.
  - Per-VU cookie jar isolation for closed tests.
  - Centralized redaction covering `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, `Token`, `Secret`, password query parameters, and Bearer tokens.
  - Reports record authentication mode (`auth_mode`), token count (`token_count`), and cookie jar status (`cookie_jar_enabled`), never secret values.
- **`docs/DAEGSA_Implementation_Plan.md` §12 (Safety Controls)**:
  - Ensure `--dry-run` and `daegsa validate` print sanitized execution plans with tokens masked to `[REDACTED]`.
  - Sanitize URLs by stripping or masking userinfo and sensitive query parameters.
- **`docs/DAEGSA_Implementation_Plan.md` §13 (Reports and Reproducibility)**:
  - ANSI terminal report displays sanitized authentication summary.
  - JSON report v1 records sanitized `auth` summary (`auth_mode`, `token_count`, `cookie_jar_enabled`) matching `testdata/schemas/v1/report.schema.json`.
  - SHA256 configuration fingerprint computation sanitizes auth credentials so secret rotations do not alter test fingerprints.
- **`docs/DAEGSA_Implementation_Plan.md` §15 (Phase 5 - Authentication and Secret Handling)**:
  - Primary milestone deliverables and exit gate: "secrets cannot appear in standard outputs, fixtures, or reports."
- **`docs/DAEGSA_Implementation_Plan.md` §16 (Version Roadmap)**:
  - v0.4.0 milestone scope: Bearer/API-key auth, token pools, redaction hardening.
- **`docs/DAEGSA_Implementation_Plan.md` §19 (Acceptance Criteria for v1)**:
  - "Secrets do not appear in terminal output, errors, fingerprints, or JSON reports."

---

## Current Repository Findings

1. **`internal/config/types.go` & `testdata/schemas/v1/config.schema.json`**:
   - `Config` struct lacks an explicit `Auth AuthConfig` section. Authentication is currently handled ad-hoc via raw strings in `request.headers` (e.g. `Authorization: Bearer ${API_TOKEN}`).
   - Need structured `AuthConfig` supporting `type` (`none`, `bearer`, `custom_header`, `token_pool`, `basic`), `token`, `header_name`, `username`, `password`, `token_pool`, and `cookie_jar`.
   - `testdata/schemas/v1/config.schema.json` requires schema definition for the `auth` property.
2. **`internal/config/validate.go`**:
   - `ValidateConfig` validates request parameters, workload models, thresholds, and safety allowlists, but lacks validation rules for `auth`.
   - Needs strict validation: valid `auth.type`, non-empty tokens/headers when required, non-empty pools for `token_pool`, and restriction/handling of `cookie_jar` on closed vs open models.
3. **`internal/config/redact.go`**:
   - Implements `IsSensitiveHeader`, `IsSensitiveQueryParam`, `RedactHeaders`, `RedactHTTPHeaders`, and `RedactURL`.
   - Needs hardening to cover all standard auth headers (`Proxy-Authorization`, `X-Auth-Token`, `X-Api-Key`, `Set-Cookie`, `Cookie`, `X-Token`, `ApiKey`, `Session`, `Passwd`), Basic auth credentials (`Basic <base64>`), Bearer tokens, and arbitrary string/error redaction (`RedactString`, `RedactError`).
4. **`internal/config/fingerprint.go`**:
   - `cloneConfigSanitized` redacts `Request.URL` and `Request.Headers`, but does not yet sanitize `Auth` fields (masking `token`, `token_pool`, `password`, and `username` to `[REDACTED]`).
5. **`internal/plan/plan.go` & `internal/plan/print.go`**:
   - `Plan` struct lacks fields for resolved auth provider (`auth.TokenProvider`), auth metadata (`AuthType`, `HeaderName`, `TokenCount`), and `CookieJarEnabled`.
   - `FormatPlanSummary` in `internal/plan/print.go` needs to render sanitized authentication metadata in the execution plan summary.
6. **`internal/executor/executor.go` & `internal/executor/request.go`**:
   - `HTTPExecutor` currently maintains a single `http.Client` with a shared `http.Transport`.
   - `ExecuteRequest(ctx context.Context)` does not accept a worker/VU ID.
   - Needs extension to support `ExecuteRequest(ctx context.Context, workerID int)` (or `ExecuteRequestWithVU`), injecting deterministic auth headers and utilizing per-VU `http.CookieJar` instances while maintaining the single shared `http.Transport`.
7. **`internal/scheduler/closed.go` & `internal/scheduler/open.go`**:
   - `ClosedScheduler` spawns $N$ VUs ($0 \le i < \text{Users}$).
   - `OpenScheduler` spawns a worker pool of size $\text{MaxInFlight}$ ($0 \le j < \text{MaxInFlight}$).
   - Both schedulers currently call `executor.ExecuteRequest(ctx)` without passing their worker index. Schedulers must pass $VU_i$ or $Lane_j$ to ensure deterministic token selection and cookie jar routing.
8. **`internal/report/types.go`, `internal/report/builder.go`, & `testdata/schemas/v1/report.schema.json`**:
   - `Report` struct currently does not store auth summary metadata.
   - Need optional `auth` property in `Report` (`auth_mode`, `token_count`, `cookie_jar_enabled`) and corresponding schema definitions in `testdata/schemas/v1/report.schema.json`.
9. **`internal/testtarget/handler.go`**:
   - Currently provides endpoints for delays, drops, redirects, byte sizing, and cookies (`/cookies/set`, `/cookies/inspect`).
   - Needs authenticated endpoints (`/auth/bearer`, `/auth/header`, `/auth/basic`, `/auth/token-pool`) to facilitate contract and regression testing.

---

## Files Expected to Change

### New Files
- `internal/auth/types.go`: Canonical auth types, enums (`AuthTypeNone`, `AuthTypeBearer`, `AuthTypeCustomHeader`, `AuthTypeTokenPool`, `AuthTypeBasic`), and interfaces (`TokenProvider`, `CookieJarManager`, `Authenticator`).
- `internal/auth/provider.go`: Deterministic token pool provider implementation for closed ($VU_i \pmod N$) and open ($Lane_j \pmod N$) workloads, plus static token provider.
- `internal/auth/provider_test.go`: Unit tests for deterministic token assignment, pool wrapping, empty/single/multi-token pool behavior.
- `internal/auth/jar.go`: Per-VU cookie jar manager creating and caching isolated `net/http/cookiejar` instances.
- `internal/auth/jar_test.go`: Unit tests verifying cross-VU cookie isolation and session persistence across multiple iterations of the same VU.
- `internal/auth/authenticator.go`: Request authenticator injecting Bearer, custom header, Basic auth, or token pool headers into outgoing `*http.Request` instances.
- `internal/auth/authenticator_test.go`: Unit tests verifying header injection, case normalization, and override precedence.
- `internal/auth/redact.go`: Centralized string, header, URL, and error redaction utilities.
- `internal/auth/redact_test.go`: Comprehensive table-driven tests verifying redaction across all sensitive headers, query params, Basic/Bearer strings, error messages, and stack traces.
- `examples/authenticated-api.yaml`: Example manifest demonstrating static Bearer token authentication with environment variable reference.
- `examples/token-pool-load.yaml`: Example manifest demonstrating multi-token pool load testing.
- `examples/cookie-session-closed.yaml`: Example manifest demonstrating closed-model per-VU cookie session isolation.

### Modified Files
- `internal/config/types.go`: Add `AuthConfig` struct and `Auth AuthConfig` field to `Config`.
- `internal/config/validate.go`: Add `validateAuth` to `ValidateConfig` with strict rules for types, tokens, header names, token pools, and cookie jars.
- `internal/config/redact.go`: Extend sensitive header/query substring lists and integrate with centralized redaction engine.
- `internal/config/redact_test.go`: Add tests for extended header and query param redaction.
- `internal/config/fingerprint.go`: Sanitize `Auth` struct in `cloneConfigSanitized` ensuring tokens/passwords are masked before hashing.
- `internal/config/fingerprint_test.go`: Verify SHA256 fingerprint invariance across credential rotations.
- `internal/plan/plan.go`: Add `Auth *auth.AuthConfig`, `TokenProvider auth.TokenProvider`, `CookieJarManager *auth.CookieJarManager`, and `CookieJarEnabled bool` to `Plan`.
- `internal/plan/plan_test.go`: Verify deep cloning and immutability of auth configuration and token providers in `Plan`.
- `internal/plan/print.go`: Add sanitized authentication metadata to `FormatPlanSummary`.
- `internal/plan/print_test.go`: Verify zero credential leakage in plan summary prints.
- `internal/executor/executor.go`: Update `ExecuteRequest` to accept `workerID int`, retrieve deterministic tokens, inject auth headers, and route requests through per-VU cookie jars while sharing `http.Transport`.
- `internal/executor/executor_test.go`: Test request execution with Bearer, custom headers, Basic auth, token pools, and cookie jars.
- `internal/executor/request.go`: Support request building with injected auth headers.
- `internal/scheduler/closed.go`: Pass `workerID` ($VU_i$) from `runVU` into `executor.ExecuteRequest(ctx, workerID)`.
- `internal/scheduler/closed_test.go`: Test closed workload execution with per-VU cookie jars and token pools.
- `internal/scheduler/open.go`: Pass `workerID` ($Lane_j$) from `runWorker` into `executor.ExecuteRequest(ctx, workerID)`.
- `internal/scheduler/open_test.go`: Test open arrival-rate workload execution with deterministic token lane distribution.
- `internal/report/types.go`: Add `Auth *AuthSummary` to `Report`.
- `internal/report/builder.go`: Populate `rep.Auth` with sanitized `auth_mode`, `token_count`, and `cookie_jar_enabled`.
- `internal/report/terminal.go`: Render sanitized authentication summary in console output.
- `internal/report/terminal_test.go`: Test terminal formatting with various auth modes, asserting no secret values appear.
- `internal/report/schema_test.go`: Test JSON reports with `auth` objects against `testdata/schemas/v1/report.schema.json`.
- `testdata/schemas/v1/config.schema.json`: Add schema definition for `auth` configuration block.
- `testdata/schemas/v1/report.schema.json`: Add schema definition for `auth` report summary.
- `internal/testtarget/handler.go`: Add `/auth/bearer`, `/auth/header`, `/auth/basic`, and `/auth/token-pool` endpoints.
- `internal/testtarget/server_test.go`: Verify test target auth endpoints.
- `internal/cli/cli_test.go`: End-to-end CLI tests verifying authentication, token pools, cookie jars, dry-run sanitization, and zero secret leakage in all outputs.

---

## Implementation Checklist

### 1. Configuration & Schema Updates (`internal/config`, `testdata/schemas/v1/`)
- [ ] Update `internal/config/types.go`:
  - [ ] Define `AuthType` constants: `AuthTypeNone` (`""` or `"none"`), `AuthTypeBearer` (`"bearer"`), `AuthTypeCustomHeader` (`"custom_header"`), `AuthTypeTokenPool` (`"token_pool"`), `AuthTypeBasic` (`"basic"`).
  - [ ] Define `AuthConfig` struct:
    - `Type string` (`yaml:"type,omitempty" json:"type,omitempty"`)
    - `Token string` (`yaml:"token,omitempty" json:"token,omitempty"`)
    - `HeaderName string` (`yaml:"header_name,omitempty" json:"header_name,omitempty"`)
    - `Username string` (`yaml:"username,omitempty" json:"username,omitempty"`)
    - `Password string` (`yaml:"password,omitempty" json:"password,omitempty"`)
    - `TokenPool []string` (`yaml:"token_pool,omitempty" json:"token_pool,omitempty"`)
    - `CookieJar bool` (`yaml:"cookie_jar,omitempty" json:"cookie_jar,omitempty"`)
  - [ ] Add `Auth AuthConfig` (`yaml:"auth,omitempty" json:"auth,omitempty"`) field to `Config` struct.
- [ ] Update `internal/config/validate.go`:
  - [ ] Implement `validateAuth(cfg *Config) error`:
    - Normalize `cfg.Auth.Type` (lowercase, trimmed).
    - If `cfg.Auth.Type` is empty, treat as `AuthTypeNone`.
    - Validate `cfg.Auth.Type` is one of `none`, `bearer`, `custom_header`, `token_pool`, `basic`.
    - If `bearer`: require `token != ""`. Default `header_name` to `"Authorization"`.
    - If `custom_header`: require `token != ""` and `header_name != ""`.
    - If `basic`: require `username != ""`. Default `header_name` to `"Authorization"`.
    - If `token_pool`: require `len(token_pool) > 0` with no empty token elements. Default `header_name` to `"Authorization"` (or `"X-API-Key"` if specified).
    - If `cookie_jar == true` and `load.model == open`: allow with warning or document behavior (cookie jar operates per open worker lane or is stateless). If `load.model == closed`, enable per-VU cookie jar isolation.
  - [ ] Call `validateAuth` inside `ValidateConfig`.
- [ ] Update `testdata/schemas/v1/config.schema.json`:
  - [ ] Add `auth` property schema definition specifying `type`, `token`, `header_name`, `username`, `password`, `token_pool`, and `cookie_jar`.
- [ ] Update `internal/config/fingerprint.go`:
  - [ ] In `cloneConfigSanitized`, sanitize `cfg.Auth`:
    - If `token != ""`, set `token = RedactedPlaceholder`.
    - If `password != ""`, set `password = RedactedPlaceholder`.
    - If `username != ""`, keep username or sanitize if sensitive.
    - If `len(token_pool) > 0`, replace each element with `RedactedPlaceholder`.
  - [ ] Ensure SHA256 fingerprint remains identical when secret token values change.

### 2. Authentication Core Package (`internal/auth`)
- [ ] Create `internal/auth/types.go`:
  - [ ] Define `AuthType` type and constants matching canonical types (`AuthTypeNone`, `AuthTypeBearer`, `AuthTypeCustomHeader`, `AuthTypeTokenPool`, `AuthTypeBasic`).
  - [ ] Define `TokenProvider` interface:
    - `GetToken(workerID int) string`
    - `TokenCount() int`
  - [ ] Define `CookieJarManager` interface / struct:
    - `GetJar(vuID int) http.CookieJar`
    - `Enabled() bool`
  - [ ] Define `Authenticator` interface:
    - `AuthenticateRequest(req *http.Request, workerID int)`
    - `AuthMode() string`
    - `TokenCount() int`
- [ ] Create `internal/auth/provider.go`:
  - [ ] Implement `StaticTokenProvider`:
    - Wraps a single token string.
    - `GetToken(workerID int) string`: always returns the single token.
    - `TokenCount() int`: returns 1 (or 0 if empty).
  - [ ] Implement `TokenPoolProvider`:
    - Stores immutable slice of sanitized/cloned tokens.
    - `GetToken(workerID int) string`: returns `tokens[abs(workerID) % len(tokens)]`.
    - Handles negative `workerID` defensively (e.g. dispatcher worker ID `-1`).
    - `TokenCount() int`: returns `len(tokens)`.
  - [ ] Implement `NewTokenProvider(cfg *config.AuthConfig) (TokenProvider, error)`.
- [ ] Create `internal/auth/jar.go`:
  - [ ] Implement `VUJarManager`:
    - Thread-safe storage / pool of `net/http/cookiejar.Jar` instances keyed by VU ID ($0 \le vuID < \text{Users}$).
    - `NewVUJarManager(enabled bool, numVUs int) (*VUJarManager, error)`.
    - If enabled, pre-allocates or lazily initializes `cookiejar.New(nil)` for each VU index.
    - `GetJar(vuID int) http.CookieJar`: returns the dedicated jar for `vuID`. If disabled or `vuID < 0`, returns `nil`.
- [ ] Create `internal/auth/authenticator.go`:
  - [ ] Implement `RequestAuthenticator`:
    - Fields: `authType AuthType`, `headerName string`, `tokenProvider TokenProvider`, `basicUsername string`, `basicPassword string`.
    - `AuthenticateRequest(req *http.Request, workerID int)`:
      - If `AuthTypeBearer`: sets `req.Header.Set("Authorization", "Bearer "+token)`.
      - If `AuthTypeCustomHeader`: sets `req.Header.Set(headerName, token)`.
      - If `AuthTypeBasic`: sets `req.SetBasicAuth(username, password)`.
      - If `AuthTypeTokenPool`: retrieves token for `workerID`; if `headerName == "Authorization"` (or empty), sets `Bearer <token>`; otherwise sets `<headerName>: <token>`.
      - If `AuthTypeNone`: no-op.
  - [ ] Implement `NewAuthenticator(cfg *config.AuthConfig) (*RequestAuthenticator, error)`.

### 3. Central Redaction Hardening (`internal/config`, `internal/auth`)
- [ ] Update `internal/config/redact.go` (and create `internal/auth/redact.go` if separating boundaries):
  - [ ] Expand `sensitiveHeaderSubstrings` and exact header matching:
    - `"authorization"`, `"proxy-authorization"`, `"cookie"`, `"set-cookie"`, `"x-api-key"`, `"x-auth-token"`, `"x-token"`, `"token"`, `"secret"`, `"apikey"`, `"api-key"`, `"session"`, `"passwd"`, `"password"`.
  - [ ] Expand `sensitiveQueryParamSubstrings`:
    - `"token"`, `"secret"`, `"key"`, `"auth"`, `"password"`, `"signature"`, `"apikey"`, `"api_key"`, `"access_token"`, `"session"`, `"bearer"`, `"refresh_token"`.
  - [ ] Implement `RedactString(s string, knownSecrets []string) string`:
    - Scans arbitrary text (error messages, transport error strings, debug logs, stack traces) and replaces any occurrences of `knownSecrets` with `[REDACTED]`.
    - Detects and redacts Bearer token patterns: `(?i)Bearer\s+[A-Za-z0-9_\-\.~+/]+=*` $\to$ `Bearer [REDACTED]`.
    - Detects and redacts Basic auth patterns: `(?i)Basic\s+[A-Za-z0-9+/]+=*` $\to$ `Basic [REDACTED]`.
    - Detects and redacts sensitive query param assignments in raw URLs: `(?i)(token|key|secret|password|apikey|auth)=([^& \t\r\n]+)` $\to$ `$1=[REDACTED]`.
  - [ ] Implement `RedactError(err error, knownSecrets []string) error`:
    - Wraps/formats errors ensuring no underlying secret string is leaked in `err.Error()`.

### 4. Plan & Fingerprint Integration (`internal/plan`, `internal/config`)
- [ ] Update `internal/plan/plan.go`:
  - [ ] Add fields to `Plan`:
    - `AuthType string`
    - `AuthHeaderName string`
    - `TokenProvider auth.TokenProvider`
    - `Authenticator *auth.RequestAuthenticator`
    - `JarManager *auth.VUJarManager`
    - `CookieJarEnabled bool`
    - `KnownSecrets []string` (collected for string/error scrubbing)
  - [ ] Update `BuildPlan`:
    - Construct `auth.TokenProvider`, `auth.RequestAuthenticator`, and `auth.VUJarManager` from `cfg.Auth`.
    - Collect all plaintext tokens/passwords into `p.KnownSecrets` for scrubbing.
    - Ensure deep cloning and immutability of auth components.
- [ ] Update `internal/plan/print.go`:
  - [ ] In `FormatPlanSummary`, add dedicated `AUTHENTICATION & SECRETS` section:
    - Displays `Auth Type`, `Header Name`, `Token Pool Size`, and `Cookie Jar Isolation`.
    - Ensures all tokens/passwords are masked with `[REDACTED]`.
- [ ] Update `internal/config/fingerprint.go` & `fingerprint_test.go`:
  - [ ] Verify `ComputeFingerprint` masks `cfg.Auth` so changing secret values does not change the SHA256 digest.

### 5. Executor & Scheduler Integration (`internal/executor`, `internal/scheduler`)
- [ ] Update `internal/executor/executor.go`:
  - [ ] Store `plan *plan.Plan`, `jarManager *auth.VUJarManager`, `authenticator *auth.RequestAuthenticator`.
  - [ ] Support per-VU `http.Client` instances or dynamic per-request cookie jar routing:
    - Maintain a pool or mapping of `*http.Client` instances keyed by `workerID` sharing the exact same `*http.Transport` (`e.transport`).
    - If cookie jar is disabled, use the single default `*http.Client`.
    - If cookie jar is enabled, set `client.Jar = e.jarManager.GetJar(workerID)`.
  - [ ] Update `ExecuteRequest`:
    - Accept `workerID int` (e.g. `ExecuteRequest(ctx context.Context, workerID int)`).
    - Inside `ExecuteRequest`:
      - Build base HTTP request via `BuildHTTPRequest(reqCtx, e.plan)`.
      - Call `e.authenticator.AuthenticateRequest(req, workerID)`.
      - Execute request via the `workerID`-routed `http.Client`.
      - Scrub sensitive credentials from any returned transport `err` using `auth.RedactError(doErr, e.plan.KnownSecrets)`.
- [ ] Update `internal/scheduler/closed.go`:
  - [ ] In `runVU(ctx, workerID, wm, stopNewRequests, wg)`:
    - Pass `workerID` ($VU_i$) to `s.executor.ExecuteRequest(ctx, workerID)`.
    - Ensure each VU loop reliably routes to its own cookie jar and deterministic token ($VU_i \pmod N$).
- [ ] Update `internal/scheduler/open.go`:
  - [ ] In `runWorker(ctx, workerID, wm, dispatchChan, wg)`:
    - Pass `workerID` ($Lane_j$) to `s.executor.ExecuteRequest(job.ctx, workerID)`.
    - Ensure open worker lanes deterministically map to tokens ($Lane_j \pmod N$).

### 6. Test Target & Authenticated Endpoint Fixtures (`internal/testtarget`)
- [ ] Update `internal/testtarget/handler.go`:
  - [ ] Add `/auth/bearer` endpoint:
    - Inspects `Authorization` header.
    - If `Authorization == "Bearer valid-token"` (or matches configured expected token), returns `200 OK` with `{"authenticated": true, "mode": "bearer"}`.
    - Otherwise returns `401 Unauthorized` with `{"error": "unauthorized"}`.
  - [ ] Add `/auth/header` endpoint:
    - Inspects custom header (e.g. `X-API-Key` or `X-Auth-Token`).
    - If matching expected key, returns `200 OK` with `{"authenticated": true, "mode": "custom_header"}`.
    - Otherwise returns `401 Unauthorized`.
  - [ ] Add `/auth/basic` endpoint:
    - Inspects `Authorization: Basic <base64>`.
    - If credentials match expected user/pass, returns `200 OK`.
    - Otherwise returns `401 Unauthorized`.
  - [ ] Add `/auth/token-pool` endpoint:
    - Accepts requests with tokens from a pool.
    - Records received token in server state for verification.
    - Returns `200 OK` with `{"received_token": "<token_hash_or_index>"}`.
  - [ ] Verify `/cookies/set` and `/cookies/inspect` endpoints for per-VU session cookie validation.

### 7. Report & CLI Usability Updates (`internal/report`, `internal/cli`)
- [ ] Update `internal/report/types.go`:
  - [ ] Define `AuthReportSummary` struct:
    - `AuthMode string` (`json:"auth_mode"`: `"none"`, `"bearer"`, `"custom_header"`, `"token_pool"`, `"basic"`)
    - `TokenCount int` (`json:"token_count"`)
    - `CookieJarEnabled bool` (`json:"cookie_jar_enabled"`)
  - [ ] Add `Auth *AuthReportSummary` (`json:"auth,omitempty"`) to `Report` struct.
- [ ] Update `internal/report/builder.go`:
  - [ ] In `BuildReport`, populate `rep.Auth` from `p.Authenticator` and `p.CookieJarEnabled`.
- [ ] Update `testdata/schemas/v1/report.schema.json`:
  - [ ] Add optional `auth` property schema definition (`auth_mode`, `token_count`, `cookie_jar_enabled`).
- [ ] Update `internal/report/terminal.go`:
  - [ ] In `FormatTerminalReport`, include sanitized auth summary in the header banner (e.g. `Auth: bearer (1 token)` or `Auth: token_pool (4 tokens, cookie jar enabled)`).
  - [ ] Assert that no token strings ever appear in terminal reports.
- [ ] Update `internal/cli/validate.go` & `internal/cli/run.go`:
  - [ ] Ensure dry-run and validation prints use sanitized plan representations with zero secret leakage.

### 8. Manifest Examples & Golden Schemas (`examples/`, `testdata/schemas/v1/`)
- [ ] Create `examples/authenticated-api.yaml`:
  - [ ] Valid manifest using `auth.type: bearer` and `token: ${API_TOKEN}`.
- [ ] Create `examples/token-pool-load.yaml`:
  - [ ] Valid manifest using `auth.type: token_pool`, `token_pool: [${TOKEN_1}, ${TOKEN_2}, ${TOKEN_3}]`, and canonical thresholds.
- [ ] Create `examples/cookie-session-closed.yaml`:
  - [ ] Valid closed-model manifest using `auth.cookie_jar: true` and think time.
- [ ] Update `internal/config/schema_test.go`:
  - [ ] Validate new example manifests against `testdata/schemas/v1/config.schema.json`.

---

## Test Checklist

### Unit Tests
- [ ] `internal/config/validate_test.go`:
  - [ ] Valid auth configurations: `bearer`, `custom_header`, `basic`, `token_pool`, `none`.
  - [ ] Invalid auth configurations: unknown auth type, missing token for bearer, missing header_name for custom_header, empty token pool, missing username for basic.
- [ ] `internal/config/fingerprint_test.go`:
  - [ ] SHA256 fingerprint invariance: verify that changing secret values in `auth.token`, `auth.password`, and `auth.token_pool` does not change the calculated fingerprint.
- [ ] `internal/auth/provider_test.go`:
  - [ ] Static token provider: returns token for any worker ID; token count is 1.
  - [ ] Token pool provider (Closed model): verify $VU_i \to \text{token}_{i \pmod N}$ for $N=3$ across 10 VUs ($VU_0 \to T_0, VU_1 \to T_1, VU_2 \to T_2, VU_3 \to T_0, \dots$).
  - [ ] Token pool provider (Open model): verify $Lane_j \to \text{token}_{j \pmod N}$ across worker lanes.
  - [ ] Edge cases: pool with 1 token, large pool ($N > \text{VUs}$), negative worker IDs (e.g. dispatcher worker `-1`).
- [ ] `internal/auth/jar_test.go`:
  - [ ] Cross-VU cookie isolation: VU 0 receives cookie `session=A`, VU 1 receives cookie `session=B`. Verify VU 0 jar does not contain `session=B` and VU 1 jar does not contain `session=A`.
  - [ ] Session persistence: VU 0 makes consecutive requests to `/cookies/set` and `/cookies/inspect`; verify cookies persist across iterations.
  - [ ] Disabled jar behavior: verify requests do not store or send cookies when `cookie_jar: false`.
- [ ] `internal/auth/authenticator_test.go`:
  - [ ] Bearer auth: `Authorization: Bearer <token>` header set.
  - [ ] Custom header auth: `<header_name>: <token>` header set.
  - [ ] Basic auth: `Authorization: Basic <base64>` header set with correct decoding.
  - [ ] Token pool auth: correct token selected based on worker ID.
- [ ] `internal/auth/redact_test.go`:
  - [ ] Header redaction: verify `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`, `X-API-Key`, `X-Auth-Token`, `Secret` headers are redacted to `[REDACTED]`.
  - [ ] URL redaction: verify userinfo (`http://user:pass@host`) and sensitive query parameters (`?token=secret&apikey=12345`) are redacted.
  - [ ] String & error scrubbing: verify `RedactString` and `RedactError` mask raw tokens, passwords, and Bearer/Basic headers in error messages and stack traces.
- [ ] `internal/report/terminal_test.go`:
  - [ ] Terminal summary contains `Auth Mode` and token counts, but zero secret values.
- [ ] `internal/report/schema_test.go`:
  - [ ] Validate JSON reports containing `auth` metadata against `testdata/schemas/v1/report.schema.json`.

### Exhaustive Secret Leakage Test Suite
- [ ] `internal/auth/leakage_test.go` (or `internal/cli/leakage_test.go`):
  - [ ] Generate distinct sentinel secret strings: `SECRET_TOKEN_ALPHA_999`, `SECRET_PASS_BETA_888`, `SECRET_APIKEY_GAMMA_777`.
  - [ ] Configure load tests using these sentinels in Bearer auth, custom headers, Basic auth, and token pools.
  - [ ] Capture all output streams:
    - `stdout` logs and console output.
    - `stderr` failure messages and CI one-line summaries.
    - CLI `--dry-run` and `daegsa validate` plan outputs.
    - ANSI formatted terminal reports.
    - Exported JSON report files (`report.json`).
    - Computed SHA256 configuration fingerprints.
    - Internal error objects, transport errors, and HTTP 401 response error strings.
  - [ ] Assert that **none** of the sentinel secret strings appear anywhere in any captured output stream.

### Integration and Contract Tests against `internal/testtarget`
- [ ] `internal/executor/executor_test.go`:
  - [ ] Authenticated execution against `/auth/bearer` returning 200 on valid token, 401 on invalid token.
  - [ ] Authenticated execution against `/auth/header` returning 200 on valid header.
  - [ ] Authenticated execution against `/auth/basic` returning 200 on valid credentials.
- [ ] `internal/scheduler/closed_test.go`:
  - [ ] Closed model test with 5 VUs against `/cookies/set` and `/cookies/inspect` verifying 5 distinct isolated session states.
  - [ ] Closed model test with 6 VUs against `/auth/token-pool` with 3 tokens, verifying each token is used by exactly 2 VUs.
- [ ] `internal/scheduler/open_test.go`:
  - [ ] Open model test with 10 worker lanes against `/auth/token-pool` with 5 tokens, verifying deterministic token lane distribution.
- [ ] `internal/cli/cli_test.go`:
  - [ ] `daegsa run --config examples/authenticated-api.yaml` against `testtarget`: passes with exit code 0.
  - [ ] `daegsa run --config examples/token-pool-load.yaml` against `testtarget`: passes with exit code 0.
  - [ ] `daegsa run --config examples/cookie-session-closed.yaml` against `testtarget`: passes with exit code 0.
  - [ ] `daegsa validate --config examples/authenticated-api.yaml`: exits code 0 with sanitized output.
  - [ ] `daegsa run --dry-run --config examples/authenticated-api.yaml`: prints plan with masked credentials.

---

## Safety and Failure Behavior

1. **Zero Credential Exposure Under Failure**:
   - HTTP transport failures, connection timeouts, DNS errors, 401/403 responses, and panics must never include raw authorization tokens, cookies, or basic auth passwords in error strings, logs, or reports.
2. **Shared Transport Invariant**:
   - Per-VU cookie jars and client instances MUST share the single, tuned `*http.Transport` (`NewSharedTransport`). Under no circumstances should separate `http.Transport` instances be allocated per request or per virtual user.
3. **Deterministic Modulus & Worker Safety**:
   - Token pool indexing must safely handle negative worker IDs (e.g. dispatcher worker ID `-1`), empty pools (caught during preflight validation), and pools smaller or larger than the virtual user count without panics or index-out-of-bounds errors.
4. **Preflight Validation Failure on Missing Secrets**:
   - If an environment placeholder in `auth.token` or `auth.token_pool` cannot be resolved, configuration loading fails immediately with exit code 2 (`ExitCodeValidationFailure`) before any network traffic is initiated.
5. **Memory and State Bounds**:
   - Cookie jars and token providers are strictly bounded to the configured number of virtual users ($O(U)$) or open worker lanes ($O(M)$). Jars do not retain unbounded response payloads.

---

## Acceptance Gates

- [x] `go build ./...` compiles cleanly with 0 errors and 0 warnings.
- [x] `go vet ./...` passes with 0 diagnostics.
- [x] All unit, integration, and CLI tests pass: `go test -v ./...`.
- [ ] Race detector passes: `go test -race ./...` (unavailable in this Windows environment because CGO and a C compiler are unavailable).
- [x] JSON report serialization strictly validates against `testdata/schemas/v1/report.schema.json`.
- [x] Configuration YAML parsing strictly validates against `testdata/schemas/v1/config.schema.json`.
- [x] Exhaustive secret leakage test suite verifies zero credential presence across stdout, stderr, dry-run plans, terminal reports, JSON reports, fingerprints, and error strings.
- [x] `git diff --check` reports 0 whitespace errors or formatting issues.

---

## Explicit Non-Goals

- Multi-step login flows and automated OAuth2 token refresh (deferred to Phase 7 / §11, §16).
- JSON-path response body variable extraction and chained dynamic authentication (deferred to Phase 7 / §11).
- Non-HTTP authentication protocols (e.g. Kerberos, NTLM, mutual TLS / client certificates - deferred beyond v1 / §1).
- Ramp, stress, spike, and soak profile segment definitions (deferred to Phase 6 / §15).
- Report comparison tool (`daegsa compare`) (deferred to Phase 6 / §16).
- Distributed token synchronization across multiple nodes (deferred to v2.0.0 / §1, §16).

---

## Open Questions

- None. All authentication mechanisms (Bearer, custom header, Basic auth, token pools, per-VU cookie jars), redaction requirements, and reporting contracts are fully specified in `docs/DAEGSA_Implementation_Plan.md` §6, §7, §8, §11, §13, and §15.

---

## Handoff

### For the Plan Implementer:
1. Update `internal/config/types.go`, `internal/config/validate.go`, `internal/config/redact.go`, and `internal/config/fingerprint.go` to support the `auth` configuration block and hardened redaction.
2. Implement `internal/auth/types.go`, `internal/auth/provider.go`, `internal/auth/jar.go`, `internal/auth/authenticator.go`, and `internal/auth/redact.go`.
3. Integrate `internal/auth` into `internal/plan/plan.go` and `internal/plan/print.go`.
4. Update `internal/executor/executor.go` and `internal/executor/request.go` to inject deterministic authentication and route through per-VU cookie jars while maintaining the single shared `http.Transport`.
5. Update `internal/scheduler/closed.go` and `internal/scheduler/open.go` to pass worker/VU IDs to `executor.ExecuteRequest`.
6. Add `/auth/bearer`, `/auth/header`, `/auth/basic`, and `/auth/token-pool` endpoints to `internal/testtarget/handler.go`.
7. Update `internal/report/types.go`, `internal/report/builder.go`, `internal/report/terminal.go`, `testdata/schemas/v1/config.schema.json`, and `testdata/schemas/v1/report.schema.json`.
8. Create example manifests in `examples/` and write exhaustive unit, contract, and secret leakage test suites.

### For the Plan Tester:
1. Run `go test -v ./internal/auth/...` to verify token providers, cookie jar isolation, authenticators, and redaction logic.
2. Run `go test -v ./internal/config/...` and `go test -v ./internal/plan/...` to verify configuration validation, SHA256 fingerprint invariance, and plan printing.
3. Run `go test -v ./internal/executor/...`, `go test -v ./internal/scheduler/...`, and `go test -v ./internal/testtarget/...` to verify authenticated endpoint execution and session cookie isolation.
4. Run `go test -v ./internal/report/...` to verify schema validation against `testdata/schemas/v1/report.schema.json`.
5. Run the exhaustive secret leakage test suite in `internal/cli/cli_test.go` (or `internal/auth/leakage_test.go`) to confirm zero credential leakage.
6. Verify `go vet ./...` and `git diff --check`.

### Implementation Handoff - Phase 5 Completion

- Status: `COMMITTED`; independent verification reported `PASS` with a commit recommendation.
- Strengthened closed-model integration to prove exact `VU_i mod N` token assignment and five isolated cookie sessions persisting across iterations.
- Strengthened open-model integration to prove exact `Lane_j mod N` token assignment across ten bounded worker lanes.
- Routed Cobra command output through explicit writers and added CLI-wide credential-sentinel checks for validate, dry-run, successful terminal output, threshold-failure stderr, JSON reports, and configuration fingerprints.
- Hardened malformed-URL validation and safety errors so malformed URLs fail closed to `[REDACTED]`; URL-shaped string redaction also covers compound sensitive queries and username-only or password-bearing userinfo before CLI stderr.
- Targeted validation passed: `go test -count=1 ./internal/cli` and `go test -count=1 ./internal/scheduler`.
- Final evidence: `gofmt -l .`, `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` passed; schema and exhaustive leakage contracts passed through the repository test suite.
- Platform limitation: `go test -race ./...` is unavailable because this Windows environment has CGO disabled and no C compiler.
- Intended commit subject: `feat(auth): add secure authentication and session handling`.
- Next reader scope: Phase 6 - Profiles and Rate-Limit Analysis.
