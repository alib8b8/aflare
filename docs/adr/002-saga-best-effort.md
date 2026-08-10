# ADR-002: Best-Effort Saga Compensation

**Status**: Accepted  
**Date**: 2026-08  
**Deciders**: aflare Contributors

## Context

The saga pattern coordinates distributed transactions across multiple steps by running forward actions
and, on failure, compensating (undoing) completed steps in reverse order.

In a workflow engine that orchestrates external services (HTTP APIs, databases, LLM providers),
compensation steps themselves can fail — the external service that was called in the forward step
may be unreachable, the compensating API may return an error, or the resource may have already been
cleaned up.

Two design approaches were considered:

1. **Strong consistency** — compensation must succeed or the system enters an error state. The saga
   halts until manual intervention.
2. **Best-effort** — compensation is attempted, but failures are logged and skipped. The saga
   continues compensating earlier steps.

## Decision

We chose **best-effort compensation**.

Implementation: [`internal/workflow/executor_saga.go`](../../internal/workflow/executor_saga.go)

- Forward steps execute in order. Each step's own recovery primitives (retry, fallback, capture_error)
  run before the saga treats it as failed.
- On forward failure, completed steps are compensated in **reverse order**.
- A compensating step that fails is **logged at warn level and skipped** — the saga continues to the
  next earlier step.
- The failed forward step's output is returned as the saga output so a `capture_error` branch can
  inspect partial progress.
- The triggering error is exposed to compensating steps via `{{var.error}}`.

## Rationale

**Why not strong consistency:**

In a workflow engine orchestrating _external_ services, we cannot guarantee that a compensating
action will succeed. The external service is not under our control. A strong-consistency model
would mean:

- A single unreachable service during compensation blocks the entire rollback.
- Later steps that _could_ be compensated (e.g., releasing a database lock) are left undone.
- The operator must manually intervene for every compensation failure, even trivial ones.

This is strictly worse than best-effort: with best-effort, the operator still needs to reconcile
the _failed_ compensation, but all _other_ compensations have already run.

**Why best-effort is acceptable:**

- Compensation in a workflow engine is an _orchestration_ concern, not a _database_ concern.
  We are not implementing 2PC — we are calling external APIs in a defined order.
- The audit log records every compensation attempt and its outcome, so the operator knows exactly
  which step failed to compensate.
- The `{{var.error}}` variable lets compensating steps branch on the cause of rollback (e.g.,
  only refund if the failure was a timeout, not a validation error).
- A step with no `Compensate` declared is treated as side-effect-free — no compensation needed.

## Consequences

**Positive:**
- One unreachable service does not block the cleanup of others.
- The audit trail provides a complete record of what was and wasn't compensated.
- Operators can design compensating steps to be idempotent (safe to retry manually).

**Negative:**
- Manual reconciliation is sometimes required when compensation fails.
- The system does not guarantee that all side effects are undone — it guarantees that all
  _attemptable_ compensations were attempted.

**Mitigations:**
- Compensation failures are logged at `WARN` level with enough context for operators to act.
- The `{{var.error}}` variable enables compensating steps to implement conditional rollback.
- Workflow authors should design compensating steps to be idempotent (retry-safe).