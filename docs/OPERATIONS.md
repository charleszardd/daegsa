# DAEGSA Operator Manual

## 1. Overview and Architecture

**DAEGSA** is a portable, high-precision CLI engine engineered for repeatable REST API load, capacity, stress, soak, and rate-limit discovery testing. It provides mathematically distinct workload models, deterministic execution lifecycles, connection pool tuning, bounded memory consumption, and zero external runtime dependencies.

### Core Architectural Principles

- **Stateless Standalone Binary:** Distributes as a self-contained executable on Windows, Linux, and macOS. Requires no Go runtime, Python, Node.js, Java, or Docker.
- **Explicit Workload Semantics:** Open arrival-rate pacing and closed virtual user (VU) loops are modeled independently. Open workloads avoid coordinated omission; closed workloads provide deterministic user session concurrency.
- **Bounded Resources:** Latency histograms (HdrHistogram), streaming response bodies, error pools, and worker counts are strictly bounded to eliminate out-of-memory crashes on high-load runs.
- **Preflight Safety Gates:** Target endpoints, HTTP methods, redirect policies, and secret references are validated before any network traffic is initiated.
- **Machine-Readable Reports:** Outputs standardized JSON reports (Schema v1) alongside human-readable ASCII tables.

---

## 2. Installation and Standalone Execution

### Via `go install`

**DAEGSA CLI:**
```bash
go install github.com/charleszardd/daegsa/cmd/daegsa@latest
```

**DAEGSA Studio (GUI):**
```bash
go install github.com/charleszardd/daegsa/cmd/daegsa-gui@latest
```

Ensure your Go binary directory (`$GOPATH/bin` or `~/go/bin`, or `%USERPROFILE%\go\bin` on Windows) is in your system `PATH`.

### Standalone Executable Download

Download pre-compiled release packages from GitHub Releases or build locally from source:

- **Windows (x64):** `daegsa-v0.1.0-windows-amd64.zip` (extract `daegsa.exe`)
- **Linux (x64):** `daegsa-v0.1.0-linux-amd64.tar.gz` (extract `daegsa`)
- **macOS (Intel/Apple Silicon):** `daegsa-v0.1.0-darwin-amd64.tar.gz` / `daegsa-v0.1.0-darwin-arm64.tar.gz`

### Verifying Release Checksums

Verify release integrity using `dist/SHA256SUMS`:

```powershell
# Windows PowerShell
Get-FileHash .\daegsa.exe -Algorithm SHA256
```

```bash
# Linux / macOS
sha256sum -c SHA256SUMS
```

### PATH Setup

Place the executable in your system PATH (e.g. `C:\tools\daegsa.exe` on Windows or `/usr/local/bin/daegsa` on Linux/macOS) to run `daegsa` globally.

---

## 3. CLI Command Reference

DAEGSA provides six primary subcommands:

### `daegsa run`

Executes a load test against a target URL or using a YAML configuration file.

```bash
# Run using configuration file
daegsa run --config test-plan.yaml

# Run using CLI flags (loopback is authorized automatically for CLI-only runs)
daegsa run --url http://127.0.0.1:8080/api/items --users 10 --duration 30s

# External hosts require explicit, repeatable host-only authorization
daegsa run --url https://api.staging.example.com/items --allowed-host api.staging.example.com --model open --rate 10 --duration 30s

# Inspect compiled execution plan without generating traffic
daegsa run --config test-plan.yaml --dry-run

# Output machine-readable JSON report
daegsa run --config test-plan.yaml --output-json report.json
```

**Key Flags:**

- `--config, -c`: Path to YAML configuration file.
- `--url, -u`: Target HTTP/HTTPS endpoint URL.
- `--model, -m`: Workload model (`open` or `closed`).
- `--rate, -r`: Target requests per time unit (Open model).
- `--users`: Virtual user concurrency count (Closed model).
- `--duration, -d`: Test duration (e.g. `30s`, `5m`).
- `--think-time`: Pause between requests per VU (e.g. `50ms`).
- `--max-in-flight`: Maximum concurrent in-flight requests (Open model).
- `--allowed-host`: Authorize one exact external hostname or IP address; repeat the flag for multiple hosts. CLI values replace `safety.allowed_hosts` from YAML.
- `--output-json, -o`: File path to save the JSON execution report.
- `--dry-run`: Validate configuration and display execution plan without sending traffic.
- `--allow-destructive`: Explicitly authorize non-idempotent HTTP methods (`POST`, `PUT`, `PATCH`, `DELETE`).

---

### `daegsa validate`

Validates configuration syntax, environment variable resolution, and safety preflight checks without initiating test traffic.

```bash
daegsa validate --config test-plan.yaml
```

Returns exit code `0` on success, or exit code `2` (VALIDATION_FAILURE) / `4` (SAFETY_REFUSAL) on errors.

---

### `daegsa doctor`

Performs local system diagnostics to verify host readiness for high-precision load testing:

```bash
# Formatted terminal diagnostics
daegsa doctor

# Verbose output with detailed timing and suggestions
daegsa doctor --verbose

# JSON output for automated CI node verification
daegsa doctor --json
```

**Diagnostic Checks Performed:**

1. **Clock & Monotonic Timer Precision:** Evaluates OS timer resolution and sleep deviation.
2. **Loopback & Local DNS Resolution:** Tests localhost lookup latency and IPv4/IPv6 resolution.
3. **TLS Handshake & Root CA Store:** Validates TLS 1.2/1.3 cipher suites and system certificates.
4. **Socket & Port Allocation:** Verifies rapid loopback TCP socket binding and ephemeral port headroom.
5. **System Resources:** Verifies available CPU cores, GOMAXPROCS, and heap allocations.

---

### `daegsa self-test`

Runs an end-to-end automated verification suite against an embedded in-process HTTP server:

```bash
# Run all self-tests
daegsa self-test

# Verbose progress
daegsa self-test --verbose

# JSON report output
daegsa self-test --json
```

**Self-Test Test Suite:**

1. `Closed Workload Loop`: 5 VUs, think time pacing, latency quantile calculation.
2. `Open Arrival-Rate Pacing`: 50 RPS arrival pacing, max-in-flight bounding, dropped tick tracking.
3. `Multi-Step Scenario`: Login token extraction, header substitution, cookie jar persistence, logout.
4. `Threshold Evaluation Engine`: Validates passing and deliberate failing threshold assertions.
5. `Report Serialization`: Verifies Schema v1 JSON generation and terminal table rendering.

---

### `daegsa compare`

Compares two JSON execution reports (e.g. baseline vs candidate) and evaluates performance regression thresholds:

```bash
daegsa compare baseline.json candidate.json \
  --max-p95-regression 10% \
  --max-error-rate-increase 0.5% \
  --output-json comparison.json
```

---

### `daegsa version`

Displays DAEGSA version, Git commit SHA, build date, Go runtime, OS, and architecture.

```bash
daegsa version
```

---

## 4. Workload Model Guide

Selecting the appropriate workload model is critical for accurate capacity and rate-limit discovery.

| Feature                        | Open Arrival-Rate (`open`)                                                 | Closed VU Loop (`closed`)                                                |
| :----------------------------- | :--------------------------------------------------------------------------- | :------------------------------------------------------------------------- |
| **Pacing Driver**        | Target rate per second (RPS)                                                 | Fixed number of Virtual Users                                              |
| **System Interaction**   | Requests dispatched regardless of server latency                             | Next request issued only after previous response finishes                  |
| **Coordinated Omission** | **Immune** (accurately reveals latency degradation)                    | Susceptible (queue delays reduce overall throughput)                       |
| **Max In-Flight Safety** | Enforced via`max_in_flight` ceiling                                        | Bounded by user count                                                      |
| **Ideal For**            | Public APIs, microservices, webhook consumers, capacity knee-point discovery | User session simulation, logged-in workflows, concurrent customer journeys |

### Example: Open Capacity Test

```yaml
schema_version: 1
name: open-capacity-test
request:
  url: https://api.example.com/v1/search
  method: GET
load:
  model: open
  rate: 250
  time_unit: 1s
  duration: 60s
  max_in_flight: 500
safety:
  allowed_hosts:
    - api.example.com
```

### Example: Closed Session Test

```yaml
schema_version: 1
name: closed-session-test
request:
  url: https://api.example.com/v1/profile
  method: GET
load:
  model: closed
  users: 50
  think_time: 100ms
  duration: 2m
safety:
  allowed_hosts:
    - api.example.com
```

---

## 5. Multi-Step Scenario Authoring

Multi-step scenarios simulate stateful user workflows with dynamic value extraction and cookie persistence.

```yaml
schema_version: 1
name: e-commerce-checkout
auth:
  cookie_jar: true # Preserves session cookies across steps

scenario:
  name: purchase_flow
  steps:
    - name: authenticate
      url: https://api.example.com/auth/login
      method: POST
      body: '{"username":"${TEST_USER}","password":"${TEST_PASS}"}'
      expected_statuses: [200]
      extract:
        auth_token:
          from: json
          expression: token
      on_failure: abort_vu

    - name: view_item
      url: https://api.example.com/items/42
      method: GET
      headers:
        Authorization: "Bearer ${auth_token}"
      expected_statuses: [200]
      think_time: 250ms

    - name: submit_order
      url: https://api.example.com/orders
      method: POST
      headers:
        Authorization: "Bearer ${auth_token}"
        Content-Type: application/json
      body: '{"item_id": 42, "qty": 1}'
      expected_statuses: [201]

load:
  model: closed
  users: 10
  duration: 1m

safety:
  allowed_hosts:
    - api.example.com
  allow_non_idempotent: true
```

---

## 6. Rate-Limit Discovery & Analysis

When benchmarking rate-limited APIs, DAEGSA captures:

- HTTP `429 Too Many Requests` status codes.
- `Retry-After` headers (both integer seconds and HTTP dates).
- Standard `RateLimit-Limit`, `RateLimit-Remaining`, and `RateLimit-Reset` headers.

Set `rate_limit.treat_429_as_expected: true` to treat rate limits as expected system behavior rather than benchmark failures.

```yaml
rate_limit:
  treat_429_as_expected: true

thresholds:
  rate_limited_rate: "<= 15%"
  p95: "<= 200ms"
```

---

## 7. CI/CD Pipeline Integration

### Exit Code Contract

DAEGSA returns canonical exit codes designed for automated CI gate evaluation:

|      Code      | Status                 | Meaning                                                                     |
| :-------------: | :--------------------- | :-------------------------------------------------------------------------- |
| **`0`** | `PASS`               | Test completed successfully and all thresholds passed.                      |
| **`1`** | `FAIL_THRESHOLDS`    | Test ran, but one or more configured thresholds failed.                     |
| **`2`** | `VALIDATION_FAILURE` | Configuration syntax, environment resolution, or schema error.              |
| **`3`** | `RUNTIME_FAILURE`    | Internal failure, unrecoverable network crash, or diagnostic failure.       |
| **`4`** | `SAFETY_REFUSAL`     | Safety policy violation (disallowed host, unauthorized destructive method). |

### GitHub Actions Workflow Example

```yaml
name: API Performance Gate

on:
  pull_request:
    branches: [main]

jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Download DAEGSA
        run: |
          curl -sSL -O https://github.com/charleszardd/daegsa/releases/download/v0.1.0/daegsa-v0.1.0-linux-amd64.tar.gz
          tar -xzf daegsa-v0.1.0-linux-amd64.tar.gz
          sudo mv daegsa /usr/local/bin/

      - name: Run Host Diagnostics
        run: daegsa doctor

      - name: Run Preflight Validation
        env:
          TARGET_URL: "https://staging-api.example.com"
        run: daegsa validate --config tests/load-test.yaml

      - name: Execute Load Test
        env:
          TARGET_URL: "https://staging-api.example.com"
          API_KEY: ${{ secrets.STAGING_API_KEY }}
        run: |
          daegsa run \
            --config tests/load-test.yaml \
            --output-json results/report.json

      - name: Archive Test Artifacts
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: load-test-report
          path: results/report.json
```
