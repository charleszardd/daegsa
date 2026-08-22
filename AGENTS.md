# DAEGSA Engineering Policy

This file defines repository-wide engineering rules for every human and agent working on DAEGSA. It applies from the repository root downward unless a more specific nested `AGENTS.md` explicitly overrides a rule for its subtree.

## Sources of truth

- Read `docs/DAEGSA_Implementation_Plan.md` before planning or implementing a phase.
- Read the active `plans/implementation-plan.md` before making phase-scoped changes.
- Preserve the approved open- and closed-workload semantics, safety contracts, outcome taxonomy, report schema, and acceptance gates.
- Do not edit the canonical implementation plan unless the user explicitly requests a design change.
- When instructions conflict, follow the user's current request first, then the closest applicable `AGENTS.md`, then the active execution plan.

## General standard

- Write production-quality, idiomatic Go.
- Prefer correctness, clarity, and measurability over cleverness or maximum request volume.
- Keep changes focused on the active phase or tranche.
- Preserve unrelated user changes and existing public behavior unless the plan explicitly changes it.
- Do not leave known broken behavior, ignored errors, unexplained TODOs, or disabled tests behind.

## Naming consistency

- Use one canonical term for each domain concept across Go code, YAML, JSON, reports, tests, and documentation.
- Use the plan's established terms: `open model`, `closed model`, `target RPS`, `achieved start rate`, `completed throughput`, `in flight`, `dropped`, `canceled`, and `rate limited`.
- Do not use synonyms interchangeably when they could imply different measurements. For example, do not call completed throughput "achieved RPS."
- Prefer descriptive names that reveal purpose and units. Use `requestTimeout`, `maxInFlight`, and `completedRequests`, not `timeout`, `max`, or `count` when the wider meaning is unclear.
- Avoid abbreviations except established domain and Go terms such as `API`, `CLI`, `CPU`, `DNS`, `HTTP`, `ID`, `JSON`, `RPS`, `TLS`, `URL`, and `VU`.
- Keep initialisms consistently capitalized in Go: `HTTPClient`, `targetURL`, `requestID`, and `maxRPS`.
- Package names must be short, lowercase, singular, and free of underscores. Do not repeat the package name in exported identifiers.
- Exported identifiers use `PascalCase`; unexported identifiers use `camelCase`.
- Boolean names should read as a condition or capability, such as `enabled`, `hasDeadline`, `canRetry`, or `shouldDrain`.
- Error sentinel names begin with `Err`; concrete error types describe the failure, such as `ConfigError` or `SafetyViolationError`.
- Interface names describe behavior. Do not prefix interfaces with `I`; use names such as `Scheduler`, `Reporter`, or `Histogram`.
- Single-letter variables are limited to conventional, narrow scopes such as loop indexes, mathematical expressions, receivers, or coordinates. Use descriptive names elsewhere.
- Preserve public YAML and JSON field names once released. A naming cleanup is not sufficient reason to break a public schema.

## Constants and magic values

- Do not embed unexplained numeric or string values that carry domain meaning, operational limits, timing behavior, protocol semantics, or repeated configuration.
- Give significant values a descriptive named constant close to their owner, such as `defaultRequestTimeout`, `maxSupportedSchemaVersion`, or `exitCodeThresholdFailure`.
- Follow idiomatic Go naming. Do not use C-style `UPPER_SNAKE_CASE`; use `DefaultRequestTimeout` for an exported constant or `defaultRequestTimeout` for an unexported constant.
- Use `const` for compile-time values and `var` only when runtime initialization is required.
- Represent time with `time.Duration`, byte sizes with an explicit byte-oriented type or name, and rates with names that state their unit or time base.
- Do not replace locally obvious structural literals such as `0`, `1`, an empty string, or a one-off test-table value with a meaningless constant. A named constant must improve understanding or consistency.
- Do not create a global constants dumping ground. Keep constants with the package and behavior they govern.
- Prefer configuration over constants for values operators must tune, but enforce documented hard safety ceilings in code.

## KISS: keep the design simple

- Choose the smallest design that fully satisfies the current requirements and acceptance gates.
- Prefer direct control flow and explicit data structures over reflection, code generation, metaprogramming, or generic frameworks.
- Do not build extension points, plugin systems, distributed coordination, or abstractions for hypothetical future requirements.
- Introduce an interface only when there is a real boundary, more than one meaningful implementation, or a deterministic test seam that cannot be achieved more simply.
- Prefer composition over inheritance-like embedding used only for reuse.
- Avoid configuration flags that create invalid combinations. Model distinct concepts with explicit types or validated structures.
- Optimize only after measuring. Keep benchmark evidence with performance-motivated complexity.

## DRY: remove duplicated knowledge, not every repeated line

- Keep each business rule, protocol classification, safety ceiling, schema definition, and outcome mapping in one authoritative place.
- Reuse shared validation and normalization when multiple paths must enforce the same invariant.
- Do not duplicate canonical strings or calculations across scheduler, metrics, threshold, and report packages.
- Do not create an abstraction merely because two short blocks look similar. Duplication is preferable when the behaviors have different reasons to change.
- Apply the rule of three for incidental code similarity unless duplicated domain knowledge could already diverge dangerously.
- Tests may repeat setup when it improves readability and isolation. Extract helpers only when their contract is clearer than the repeated setup.

## Functions and control flow

- Each function should have one clear responsibility and operate at one level of abstraction.
- Keep functions small enough to understand without repeatedly jumping through the file. There is no arbitrary line limit; split a function when it has multiple reasons to change, deep nesting, mixed abstraction levels, or a name that cannot describe all of its work.
- Prefer guard clauses and early returns over deeply nested conditionals.
- Keep the happy path visible.
- Use descriptive parameter and result names. Avoid long lists of positional parameters; introduce a focused input struct when several values form one concept.
- Avoid boolean parameters when `true` and `false` are not self-explanatory at the call site. Prefer a named option or explicit type.
- Keep side effects at package boundaries. Make parsing, validation, scheduling calculations, classification, and threshold evaluation pure where practical.
- Pass `context.Context` as the first parameter for cancellable work. Never store a request context in a long-lived struct.
- Return errors as the final result. Add useful context while preserving error identity with `%w` when callers need `errors.Is` or `errors.As`.
- Never swallow errors. If an error is deliberately best-effort, document the policy and emit an observable signal.
- Comments explain why, invariants, or non-obvious tradeoffs. Do not narrate obvious code.

## Architecture boundaries

- Keep configuration parsing, immutable planning, scheduling, HTTP execution, authentication, safety, metrics, thresholds, and reporting separate.
- Dependencies flow toward small domain contracts; reporters must not control schedulers, and metrics must not change request execution behavior.
- Build and validate an immutable execution plan before traffic begins.
- Do not let report formatting become the source of truth for metrics or outcome classification.
- Keep public report and configuration schemas versioned and backward-compatible according to the canonical plan.

## Concurrency and resource safety

- Every queue, channel, worker pool, goroutine set, response read, error sample, and metric cardinality must be bounded or proven finite by the execution plan.
- Never start fire-and-forget goroutines. Every goroutine needs an owner, cancellation path, and completion/join strategy.
- Do not use `time.Sleep` to synchronize tests or production goroutines. Use contexts, timers, channels, wait groups, or controllable clocks.
- Do not hold locks while performing network I/O, blocking sends, report rendering, or other unbounded work.
- Preserve atomic reconciliation between planned, scheduled, started, completed, canceled, and dropped counts.
- Open-model overload must drop work at the documented boundary rather than create an unbounded queue or later catch-up burst.
- Closed-model users must wait for their current iteration and preserve per-user state isolation.
- Run race detection for concurrency changes whenever the environment supports it; an unavailable race detector must be reported, not silently treated as passing.

## HTTP and security

- Reuse the shared `http.Transport`; do not create transports per request.
- Close and appropriately drain response bodies while respecting configured body limits and cancellation.
- Do not add automatic retries unless the canonical plan explicitly introduces and accounts for them.
- Revalidate every redirect target against safety policy.
- Safety refusal must occur before load traffic starts.
- Never log or report authorization values, API keys, cookies, session tokens, resolved secret environment variables, or sensitive payloads.
- Keep redaction centralized and test it across errors, logs, terminal reports, JSON reports, and configuration fingerprints.
- Treat disabling TLS verification, destructive HTTP methods, production traffic, and external side effects as explicit high-risk operations.

## Testing policy

- Every behavior change requires tests at the lowest useful layer and integration coverage where component boundaries matter.
- Prefer deterministic table-driven tests for parsing, validation, classification, scheduling calculations, threshold evaluation, and reporting.
- Timing-sensitive code needs deterministic clock-based tests plus a small number of real-clock tolerance tests.
- Test success, boundary, invalid-input, timeout, cancellation, saturation, and cleanup behavior.
- Verify invariants, not implementation text. Do not use source-fragment assertions when observable behavior can be exercised.
- Do not weaken assertions, increase arbitrary sleeps, skip tests, or reduce workload solely to make a failure disappear.
- Tests must not depend on public internet services or real production targets.
- Add benchmarks and allocation checks for hot paths. A benchmark result must state what changed and why the regression or improvement is acceptable.
- Run formatting, targeted tests, broader affected-package tests, `go test ./...`, `go vet ./...`, `go build ./...`, `go test -race ./...` when supported, and `git diff --check` in proportion to the change and execution plan.

## Dependencies and generated artifacts

- Prefer the standard library when it is clear and correct, but do not reimplement a mature, security-sensitive, or statistically complex component without evidence.
- Adding or upgrading a production dependency requires a stated reason, license and maintenance check, and focused tests.
- Keep dependencies minimal and pinned through `go.mod` and `go.sum`.
- Do not commit binaries, large generated reports, secrets, local environment files, editor state, or temporary artifacts.

## Agent workflow policy

- Use the repository agents in this order: reader, implementer, tester, committer.
- Work on one bounded phase or tranche at a time.
- Do not run write-heavy agents concurrently.
- Only the plan committer may create the phase commit, and only after independent verification reports `PASS`.
- Never push, merge, rebase, reset, stash, amend, discard unrelated changes, or bypass hooks unless the user explicitly authorizes that exact action.
- Stop for a material ambiguity, unsafe operation, possible secret exposure, unisolatable dirty-worktree conflict, or the same root blocker recurring three times.

## Definition of done

A change is done only when:

- Names and terminology are consistent.
- Significant literals are represented by clear constants or configuration.
- Functions and package boundaries remain understandable.
- Required behavior and failure paths are tested.
- Formatting, relevant tests, build, vet, and diff checks pass.
- Resource bounds, cancellation, safety, and redaction requirements are preserved.
- Documentation and schemas match actual behavior.
- Independent tester evidence passes for the exact working tree.
- No unrelated or temporary changes are included.
