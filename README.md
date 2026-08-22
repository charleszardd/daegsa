# DAEGSA

**REST API Load, Capacity, and Rate-Limit Testing CLI**

## What is DAEGSA?

**DAEGSA** is an opinionated, deterministic Go CLI for REST API load, capacity, stress, spike, soak, authenticated endpoint, and rate-limit testing. It mathematically decouples open arrival-rate traffic from closed virtual-user traffic, enforces strict safety allowlists, and reports generator limitations instead of presenting them as server capacity.

### Name Meaning & Origin

The name **DAEGSA** is derived from a blend of concepts representing traffic load and performance cost:

- **Daega / 대가 (代價/대가)**: Korean for *price*, *cost*, or *compensation*.
- **Dagsa**: Cebuano/Bisaya for an *influx*, *surge*, or *a large number arriving*.
- **-SA**: Inspired by the suffix of *dagsa* and its crowd/influx connotation.

> **Core Concept**: *Measuring the performance cost of a large influx of API traffic.*

## Installation

### Via `go install`

**DAEGSA CLI:**
```bash
go install github.com/charleszardd/daegsa/cmd/daegsa@latest
```

**DAEGSA Studio (GUI):**
```bash
go install github.com/charleszardd/daegsa/cmd/daegsa-gui@latest
```

Ensure your Go bin directory (`$GOPATH/bin` or `~/go/bin`, or `%USERPROFILE%\go\bin` on Windows) is included in your system `PATH`.

### Pre-Built Binaries

Download pre-compiled release packages for Windows, Linux, and macOS from [GitHub Releases](https://github.com/charleszardd/daegsa/releases).

### Build from Source

```bash
git clone https://github.com/charleszardd/daegsa.git
cd daegsa

# Build CLI
make build
# or: go build -trimpath -o bin/daegsa ./cmd/daegsa

# Build GUI Studio
make build-gui
# or: go build -trimpath -o bin/daegsa-gui ./cmd/daegsa-gui
```

## Quick Start

```bash
# Verify local host readiness and diagnostics
daegsa doctor

# Run automated in-process end-to-end self-tests
daegsa self-test

# Validate configuration and safety preflight
daegsa validate --config examples/open-api-capacity.yaml

# Execute load test with JSON report output
daegsa run --config examples/open-api-capacity.yaml --output-json result.json

# Compare baseline vs candidate reports for performance regressions
daegsa compare baseline.json candidate.json
```

## Documentation

- **[Operator Manual (docs/OPERATIONS.md)](docs/OPERATIONS.md)** — Complete CLI reference, workload model guide, capacity testing workflows, CI/CD integration, and troubleshooting.
- **[Production Safety Runbook (docs/SAFETY_RUNBOOK.md)](docs/SAFETY_RUNBOOK.md)** — Preflight allowlisting, destructive method authorization, redirect policies, secret redaction, and emergency procedures.
- **[Architecture and Implementation Plan (docs/DAEGSA_Implementation_Plan.md)](docs/DAEGSA_Implementation_Plan.md)** — Workload semantics, metrics reconciliation, outcome taxonomy, and safety contracts.

## Key Capabilities

### 1. Workload Models & Arrival Pacing

- **Open Arrival-Rate (`load.model: open`)**: True rate-driven arrival pacing immune to coordinated omission. Enforces `max_in_flight` bounds and tracks dropped work explicitly without runaway catch-up bursts.
- **Closed VU Loops (`load.model: closed`)**: Deterministic concurrency with user think time pacing and isolated per-VU cookie jars.
- **Profile Stepped Ramps**: Dynamic rate step profiles with warm-up/cool-down segment metrics.

### 2. Multi-Step Scenarios

DAEGSA supports stateful multi-step workflow scenarios under closed workloads:

- **Dynamic Variable Extraction & Substitution**: Extract tokens, IDs, and headers from JSON, JSONPath (`$.token`, `items[0].id`), response headers, cookies, or regex capture groups, and substitute them dynamically into subsequent step URLs, headers, and request bodies via `${var_name}` (escaped as `$${LITERAL}`).
- **Strict Per-VU Isolation**: Each virtual user executes in its own isolated memory state and cookie jar.
- **Configurable Failure Policies**: Per-step failure behavior (`on_failure: stop`, `on_failure: abort_vu`, or `on_failure: continue`).
- **Step-Level Metrics & Thresholds**: Granular performance assertions per step (`step.<step_name>.<metric>` such as `step.login.p95: "<= 100ms"`).

See [examples/multi-step-scenario.yaml](examples/multi-step-scenario.yaml) for an example scenario.

### 3. Generator Self-Diagnostics & Automated Self-Tests

- **`daegsa doctor`**: Diagnoses timer precision, loopback/local DNS, TLS cipher suites and root CA cert pool, socket/FD headroom, CPU cores, and memory allocation with PASS/WARN/FAIL indicators and actionable advice.
- **`daegsa self-test`**: In-process end-to-end verification across closed-model loops, open arrival pacing, multi-step scenario state chaining, and threshold rule evaluation against an embedded HTTP target.

### 4. Standalone Multi-Platform Distribution

- Pure standalone binaries with `-trimpath` reproducible builds.
- Pre-built packages for Windows AMD64, Linux AMD64, macOS AMD64/ARM64 in `dist/`.
- Verifiable SHA-256 checksums (`dist/SHA256SUMS`) and CycloneDX SBOM (`dist/sbom-cyclonedx.json`).
