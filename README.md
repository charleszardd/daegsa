# DAEGSA

DAEGSA is a deterministic Go CLI for REST API load, capacity, stress, spike, soak, and rate-limit testing. It keeps open arrival-rate traffic distinct from closed virtual-user traffic and reports generator limitations instead of presenting them as server capacity.

## Run

```text
daegsa run --config examples/open-api-capacity.yaml
daegsa run --config examples/profile-rate-limit.yaml --output-json result.json
daegsa compare baseline.json candidate.json
```

Schema-v1 configuration remains supported for constant open and closed workloads. Schema v2 adds ordered open-model profile segments. A segment is either constant (`rate`) or a deterministic stepped ramp (`start_rate`, `end_rate`, and `steps`). Warm-up and cool-down traffic is retained in reports but excluded from thresholds and default pass/fail evaluation.

The report root reconciles all traffic. Schema-v2 reports also include compiled segments, per-segment metrics, a measured-only summary, bounded rate-limit header consistency observations, first-throttle context, and conservative generator calibration. “No throttling observed” is evidence only for the tested rates, not a guaranteed safe production limit.

`compare` accepts complete v1 and v2 reports up to 10 MiB. It prints factual single-run deltas and comparability warnings; it does not claim statistical significance or invent regression thresholds.

See [docs/DAEGSA_Implementation_Plan.md](docs/DAEGSA_Implementation_Plan.md) for workload semantics and safety contracts.
