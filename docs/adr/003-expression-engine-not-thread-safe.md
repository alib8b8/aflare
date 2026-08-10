# ADR-003: ExpressionEngine is NOT Thread-Safe (by Design)

**Status**: Accepted  
**Date**: 2026-08  
**Deciders**: aflare Contributors

## Context

The `ExpressionEngine` evaluates `{{...}}` template expressions throughout workflow execution.
It holds mutable state: step outputs, workflow variables, and loop variables. This state is
read and written by every step's condition evaluation, parameter expansion, and output expression.

When implementing the DAG executor (concurrent step execution), we faced a choice:

1. **Add a mutex** to `ExpressionEngine` so it can be safely shared across goroutines.
2. **Keep it single-goroutine** and enforce serialized access through architectural design.

## Decision

We chose to **keep `ExpressionEngine` single-goroutine and enforce serialization at the
architectural level**.

Implementation: [`internal/workflow/expression.go`](../../internal/workflow/expression.go)

The doc comment explicitly states:

> WARNING: ExpressionEngine is NOT thread-safe. It holds mutable state (stepOutputs, variables,
> loopVars). All Evaluate/Set*/Clear* calls must be serialized on a single goroutine.

The DAG executor enforces this by splitting execution into two phases:

1. **Phase 1 (main goroutine)**: Pre-evaluate all conditions and parameters for the batch.
2. **Phase 2 (worker goroutines)**: Execute `node.Execute()` — workers never touch the engine.

When concurrent evaluation is needed (map iterations, parallel steps), each goroutine creates
its own `ExpressionEngine` instance via `SnapshotVars()` to copy the parent state.

## Rationale

**Why not add a mutex:**

| Concern | Add Mutex | Keep Single-Goroutine (chosen) |
|---|---|---|
| Contention | Every expression evaluation contends on the mutex — this is the hottest path in the system | No contention |
| Deadlock risk | Nested evaluation (sub-workflows calling parent engine) can deadlock | No locks, no deadlock |
| Complexity | Every method needs lock/unlock; subtle bugs from missed unlocks | Simple: "don't share" is the rule |
| Performance | Mutex overhead on every `{{step.X}}` reference | Zero synchronization overhead |

The expression engine is the **hottest path** in workflow execution. Every step's condition,
parameters, and output expressions go through it. Adding a mutex would serialize _all_ concurrent
work, defeating the purpose of the DAG executor.

**Why the architectural approach is better:**

- The DAG executor's two-phase design naturally separates evaluation (serial) from execution
  (concurrent).
- When true concurrency is needed (map iterations), each goroutine gets its own engine — no
  sharing, no contention, the same pattern Go encourages for most mutable state.
- The "not thread-safe" doc comment and the architectural invariant are simple to understand
  and verify: "if it touches the engine, it runs on the main goroutine."

## Consequences

**Positive:**
- Zero lock contention on the hottest path.
- No risk of deadlock from nested evaluation.
- Clear, simple invariant: one goroutine owns the engine.

**Negative:**
- The two-phase design constrains the DAG executor: all evaluation must happen before any
  execution begins in a batch.
- A step's condition cannot depend on the output of another step in the same batch (this is
  already enforced by the DAG dependency model — intra-batch steps have no mutual dependencies).
- New contributors must understand the thread-safety invariant before modifying the engine.

**Mitigations:**
- The doc comment on `ExpressionEngine` is the first thing a reader sees.
- The DAG executor's code explicitly documents the two-phase split.
- `SnapshotVars()` provides a safe way to create independent engine copies.