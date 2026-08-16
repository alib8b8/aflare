# ADR-001: Append-Only WAL over Event Sourcing

**Status**: Accepted  
**Date**: 2026-08  
**Deciders**: aflare Contributors

## Context

Workflow checkpointing needs a durable persistence mechanism so that long-running workflows
can resume after a process crash. Two approaches were considered:

1. **Event Sourcing** — store every mutation as an immutable event, reconstruct state by replaying
   the full event stream.
2. **Append-Only Write-Ahead Log (WAL)** — store compact per-step snapshots, replay only the latest
   record on recovery.

The codebase previously used a JSON checkpoint (`os.WriteFile` per step), which was non-atomic and
could leave a corrupt state file on crash.

## Decision

We chose an **append-only WAL** with length-prefixed, CRC32-protected records.

Implementation: [`internal/workflow/wal.go`](../../internal/workflow/wal.go)

- Each step appends a single record containing the cumulative snapshot (data, step outputs, variables).
- Records are binary framed: `[4B length][JSON body][4B CRC32]`.
- A torn write from a process crash is detected by CRC mismatch; replay stops at the last complete record.
- Periodic compaction collapses the log into a single snapshot record via atomic rename.

## Rationale

**Why not Event Sourcing:**

| Concern | Event Sourcing | WAL (chosen) |
|---|---|---|
| Replay cost | O(N) for N steps — re-reads every event | O(1) — reads the single last record after compaction |
| Storage growth | Unbounded per run (every event persisted) | Compacted to a single record; compacted frequently |
| Complexity | Needs snapshot + event log dual-write | Single log, no dual-write |
| Audit trail | Events are the audit trail | WAL records are ephemeral; audit is a separate concern (see `history` package) |

For workflow checkpointing, the primary requirement is _crash recovery_ — we need to know "what was
the last stable state?" Event sourcing is over-engineered for this: it answers "what was every
intermediate state?" — a question the audit system already answers with tamper-evident HMAC chaining.

**Why WAL is sufficient:**

- The WAL's cumulative snapshot per step is a natural fit for the sequential executor's checkpoint model.
- CRC32 + length framing gives us the same crash-safety guarantee as event sourcing (torn write detection)
  with far less I/O.
- Compaction bounds replay time to a single record read, regardless of step count.
- The WAL is append-only: no in-place mutation, no partial overwrite possible.

## Consequences

**Positive:**
- Fast recovery: single record read after compaction.
- Bounded storage: compaction keeps the WAL file small.
- Simple: a single `Append` path, no dual-write coordination.

**Negative:**
- The WAL is not an audit trail — it is an internal crash-recovery mechanism. Audit is handled by
  the `history` package separately.
- Compaction is a synchronous operation that pauses appends briefly. For workflows with very short
  step durations, compaction frequency may need tuning.
- The WAL file format is binary and not human-readable (unlike the old JSON checkpoint).

**Mitigations:**
- Compaction only triggers when the log exceeds 1MB (configurable).
- The `history` package provides the human-readable, tamper-evident audit trail.

## Delivery semantics

The WAL guarantees **at-least-once** delivery of workflow steps after a crash: recovery replays
step internals, and a side effect fired just before the crash (HTTP POST, file write, etc.) may be
re-fired during replay. The WAL guarantees that workflow *state* is recoverable — it does **not**
guarantee business-level idempotency of side effects.

Exactly-once side effects therefore require the **WAL + IdempotencyKey** combination: the executor
deduplicates re-triggers against the idempotency ledger before any step runs. This separation and
its implementation live in [`internal/workflow/idempotency.go`](../../internal/workflow/idempotency.go).