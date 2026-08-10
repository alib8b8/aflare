# ADR-004: TLA+ Verification for DAG Scheduler Only

**Status**: Accepted  
**Date**: 2026-08  
**Deciders**: aflare Contributors

## Context

The DAG scheduler (`topoBatches` in `dag.go`) is the most algorithmically complex component
in the workflow engine. It takes a dependency graph of steps and partitions them into topological
batches where steps in the same batch have no mutual dependencies and can execute concurrently.

The scheduler must satisfy three safety invariants and one liveness property:

1. **SafeBatch** — every step in a batch has all its dependencies in prior batches.
2. **NoDoubleExec** — no step appears in two batches.
3. **AllScheduled** — every step appears in exactly one batch (completeness).
4. **EventuallyDone** — if the graph is acyclic, all steps eventually become done.

The question was: which parts of the system warrant formal verification?

## Decision

We chose to apply **TLA+ formal verification only to the DAG scheduler**.

Implementation:
- Spec: [`internal/workflow/dag_tla_spec.tla`](../../internal/workflow/dag_tla_spec.tla)
- Copy: [`docs/tla/dag_scheduler.tla`](../../docs/tla/dag_scheduler.tla)
- Executable companion: [`internal/workflow/dag_formal_test.go`](../../internal/workflow/dag_formal_test.go)

The TLA+ spec models the set of N steps, their dependency relation, and the scheduler state
(done steps, current batch). It proves the four invariants above for all acyclic graphs.

The Go test performs bounded model-checking over randomly generated DAGs, serving as an
executable companion that catches implementation bugs (e.g., off-by-one errors) that the
spec alone cannot.

## Rationale

**Why only the DAG:**

| Component | Complexity | Benefit of formal verification |
|---|---|---|
| DAG scheduler | High — topological ordering, cycle detection, batch partitioning | Critical: bugs here cause silent correctness failures (missed deps, double execution) |
| Sequential executor | Low — simple for-loop, no concurrency | Low: bugs are obvious and caught by integration tests |
| Expression engine | Medium — string parsing, template substitution | Low: bugs produce visible errors (wrong output) rather than silent corruption |
| WAL | Medium — binary framing, CRC, compaction | Medium: CRC protection catches corruption; compaction is tested with integration tests |
| Saga | Low — sequential forward + reverse compensation | Low: correctness is straightforward; best-effort semantics are intentionally loose |

The DAG scheduler is the _only_ component where a bug can produce:
- **Silently wrong results** — a step executes before its dependency, producing stale data.
- **Non-deterministic failures** — race conditions that only appear under specific concurrency.
- **Hard-to-reproduce bugs** — the batch ordering looks correct in simple cases but fails on
  specific graph topologies.

Formal verification gives us confidence that the scheduler is correct for _all_ graphs, not just
the ones we've tested.

**Why not verify everything:**

- TLA+ has a steep learning curve. Applying it to the entire system would slow development
  significantly.
- Most components are simple enough that unit tests provide adequate coverage.
- The cost-benefit ratio is highest for the DAG scheduler — it's the most complex component
  with the highest correctness risk.

## Consequences

**Positive:**
- The DAG scheduler is provably correct for all acyclic graphs.
- The executable test (`dag_formal_test.go`) catches implementation bugs before they ship.
- The TLA+ spec serves as documentation of the algorithm's invariants.

**Negative:**
- Only the DAG scheduler is formally verified. Other components rely on tests.
- The TLA+ spec must be maintained alongside the Go implementation.
- New contributors need to understand TLA+ to modify the DAG scheduler.

**Mitigations:**
- The `dag_formal_test.go` provides an approachable entry point — it's Go, not TLA+.
- The spec's comments map TLA+ concepts to Go code locations.
- Changes to the DAG scheduler trigger the formal test in CI.