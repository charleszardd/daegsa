# DAEGSA Production Safety Runbook

## 1. Safety Architecture and Core Tenets

DAEGSA employs a **defense-in-depth safety architecture** to ensure load tests cannot accidentally overwhelm unintended infrastructure, execute destructive API mutations in production environments, leak sensitive authentication tokens, or crash test runners.

### Five Core Safety Invariants

1. **Preflight Enforcement:** All safety checks (host allowlists, method permissions, DNS resolution, and credential syntax) are evaluated before establishing any HTTP connection.
2. **Explicit Authorization for Destructive Actions:** Non-idempotent HTTP methods (`POST`, `PUT`, `PATCH`, `DELETE`) require dual authorization (`safety.allow_non_idempotent: true` in config and `--allow-destructive` via CLI).
3. **Zero Secret Leakage:** Authorization headers, cookies, API tokens, and environment variables are automatically masked in reports, terminal logs, and error outputs.
4. **Hard Immutable Ceilings:** Absolute upper bounds on concurrency, duration, RPS, and body sizes are enforced at compile time.
5. **Deterministic Emergency Drain:** Immediate graceful drain and teardown upon receiving interrupt signals (`SIGINT`, `SIGTERM`, Ctrl+C).

---

## 2. Host Allowlisting (`safety.allowed_hosts`)

To prevent misdirected traffic or accidental Denial of Service (DoS) against external third-party services, all target hosts must be explicitly authorized. YAML plans use `safety.allowed_hosts`; ad-hoc CLI runs use the repeatable `--allowed-host` flag. An empty allowlist authorizes no external host.

```yaml
safety:
  allowed_hosts:
    - 127.0.0.1
    - localhost
    - api.staging.internal
```

For a CLI-only external run:

```bash
daegsa run --url https://api.staging.internal/items \
  --allowed-host api.staging.internal \
  --model open --rate 10 --duration 30s
```

Repeat `--allowed-host` for each exact hostname or IP used by the target, scenario steps, or permitted cross-origin redirects. CLI entries replace, rather than append to, YAML `safety.allowed_hosts`. CLI wildcard entries are rejected.

### Allowlist Evaluation Rules

- **Canonical Host Match:** DNS names are case-insensitive and a terminal dot is normalized. Entries contain only a DNS hostname or IP address—never a scheme, credentials, port, path, query, or fragment.
- **Loopback Convenience:** CLI-only `--url` runs automatically authorize the explicit `localhost` or loopback IP literal in that URL. YAML plans still require their loopback hosts in `safety.allowed_hosts`.
- **Port Independence:** Authorization matches the URL hostname or IP; the URL may still select a port.
- **DNS Preflight Validation:** If a hostname cannot be resolved during preflight, DAEGSA exits immediately with exit code `4` (`SAFETY_REFUSAL`).

---

## 3. Destructive HTTP Method Safeguards

By default, DAEGSA only permits safe, idempotent HTTP methods (`GET`, `HEAD`, `OPTIONS`).

### Permitting Mutation Methods

To execute `POST`, `PUT`, `PATCH`, or `DELETE`:
1. In YAML configuration:
   ```yaml
   safety:
     allow_non_idempotent: true
   ```
2. When executing via CLI flags without a config file, supply `--allow-destructive`:
   ```bash
   daegsa run --url https://api.staging.internal/items --allowed-host api.staging.internal --method POST --allow-destructive
   ```

If a destructive method is requested without authorization, DAEGSA halts immediately:
```text
daegsa: safety refusal: destructive HTTP method "POST" is not authorized (set safety.allow_non_idempotent: true or pass --allow-destructive)
```

---

## 4. Redirect Safety Policies

Automated HTTP redirects can expose tokens or redirect load to unintended external endpoints. DAEGSA enforces three redirect policies:

```yaml
request:
  redirects: same-origin # "same-origin" (default), "none", or "all"
```

- **`same-origin` (Recommended Default):** Follows redirects only if scheme, host, and port match the original target. Cross-origin redirects are blocked and recorded.
- **`none`:** Captures `3xx` responses as completed requests without following redirects.
- **`all`:** Follows cross-origin redirects only if the new destination host is explicitly in `safety.allowed_hosts`.

---

## 5. Credential & Secret Protection

### Environment Variable Placeholders

Never commit plaintext credentials into YAML files. Use `${ENV_VAR}` placeholders:

```yaml
auth:
  type: bearer
  token: ${API_TOKEN}
```

If `${API_TOKEN}` is unset or empty, DAEGSA halts during preflight validation with exit code `2` (`VALIDATION_FAILURE`).

### Automatic Redaction

DAEGSA automatically sanitizes:
- `Authorization` headers (`Bearer **********`).
- `Cookie` / `Set-Cookie` session headers.
- Custom authentication header values configured in `auth.header_name`.
- Configured secret values referenced in the execution plan.

---

## 6. Hard Safety Ceilings

DAEGSA enforces hard architectural limits to prevent generator exhaustion:

| Parameter | Safety Limit | Rationale |
| :--- | :--- | :--- |
| **Max Response Body Limit** | 50 MiB | Prevents heap exhaustion during large payload benchmarking |
| **Max Scenario Steps** | 50 steps | Keeps scenario state machines bounded and deterministic |
| **Max In-Flight Requests** | 100,000 | Protects against socket exhaustion and kernel file descriptor limits |
| **Max Recorded Memory Errors**| 100 unique samples | Prevents unbounded error string allocations |

---

## 7. Emergency Stop and Incident Procedures

### Emergency Abort (Ctrl+C / SIGINT)

1. Press `Ctrl+C` once in the terminal.
2. DAEGSA immediately stops dispatching new requests.
3. In-flight requests are allowed a graceful drain window (configured via `load.graceful_stop`, default `5s`).
4. Latency quantiles and outcome metrics recorded up to the interruption are printed.
5. Exit code `3` (`RUNTIME_FAILURE`) is returned to indicate the test was incomplete.

### Dry-Run Verification

Before running any high-volume test against staging or internal services, always execute `--dry-run`:

```bash
daegsa run --config load-test.yaml --dry-run
```

Verify:
- Target URL and resolved IP addresses.
- Workload model and planned throughput.
- Authorized methods and host allowlists.
