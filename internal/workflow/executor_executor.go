// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​‌​​​​​‌​​​‌​​​​​‌‌​​​‌‌‌​​​​‌​‌‌​‌‌‌‌​‌​‌​​​​‌​​‌​‌‌​​​​​​​​​​​​​​​​​​​‌‌​​​‌​​‌‌​‌​⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package workflow

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/metrics"
	"github.com/alib8b8/aflare/internal/nodes"
	tea "github.com/charmbracelet/bubbletea"
)

// WithCheckpoint configures the Executor to persist per-step checkpoints to
// the given path and resume from it on the next run if it exists. Returns the
// receiver for chaining.
func (e *Executor) WithCheckpoint(path string) *Executor {
	e.statePath = path
	return e
}

// WithWAL configures the Executor to use an append-only Write-Ahead Log at
// the given path for durable per-step checkpointing. This is preferred over
// WithCheckpoint for long-running workflows because:
//   - Each step appends a single record (no full-state rewrite).
//   - Records are CRC32-protected against torn tail writes from crashes.
//   - Periodic compaction bounds replay time.
//
// Resume on the next run reads the WAL via LoadStateWAL. When both WithWAL
// and WithCheckpoint are set, WithWAL takes precedence.
func (e *Executor) WithWAL(path string) *Executor {
	e.walPath = path
	return e
}

// WithTimeout configures the overall workflow timeout applied to the derived
// context for this Executor's runs. Returns the receiver for chaining.
//
// Use this instead of mutating the package-level WorkflowTimeout global,
// which is unsafe under parallel tests (t.Parallel) and deprecated.
func (e *Executor) WithTimeout(d time.Duration) *Executor {
	e.workflowTimeout = d
	return e
}

// WithAuditLog enables tamper-evident audit logging of every workflow
// execution into the history package's HMAC hash-chain audit log. For each
// run the recorder writes: a workflow_start record, one workflow_step record
// per completed step (with sanitized params and truncated input/output), and
// a workflow_end (or workflow_failed) record.
//
// When dir is non-empty it overrides the history/audit directory; when empty
// the history package default (~/.config/aflare/history) is used. Audit is
// off by default and must be explicitly enabled.
//
// The signing key is resolved entirely by the history package (env key >
// password-derived > per-install random key file, auto-generated on first
// append on new chains; legacy pre-0.9.0 chains continue under the public
// default key with a one-time warning) — no environment variable is required
// to enable audit writing. Any audit write failure is logged at warn level
// and never blocks execution. Returns the receiver for chaining.
//
// IMPORTANT (H-5): auditDir is process-global state via
// history.SetHistoryDir. Do NOT configure different auditDir values across
// concurrent Executor instances — the last SetHistoryDir call wins, so
// concurrent Executors with different dirs would silently bleed each other's
// audit records into whichever directory was set most recently. Use a single
// audit directory for all workflows in a process, or disable audit
// per-Executor (WithAuditLog(false, "")). The CLI additionally guards
// against cross-process hash-chain corruption via an audit-directory lock
// (see cmd/aflare acquireAuditLock).
func (e *Executor) WithAuditLog(enabled bool, dir string) *Executor {
	e.auditEnabled = enabled
	e.auditDir = dir
	return e
}

// WithIdempotencyKey enables workflow idempotency for this Executor's runs
// using the given key (e.g. an Idempotency-Key header from an incoming HTTP
// request). When set, ExecuteWithTrace consults the configured
// IdempotencyStore before executing:
//
//   - If a record exists for the key with status "completed", the cached
//     final output is returned together with ErrIdempotencyHit and NO step
//     is re-run. This prevents duplicate side effects (e.g. duplicate money
//     transfers in financial flows) when the same workflow is triggered
//     multiple times with the same key.
//   - Otherwise (no record, or a prior "failed"/"in_progress" record) the
//     workflow executes normally and the new run_id + result are recorded so
//     the next trigger for the same key becomes a cache hit.
//
// If no store has been configured via WithIdempotencyStore, a default
// FileIdempotencyStore at DefaultIdempotencyDir() (~/.config/aflare/
// idempotency) is used. Idempotency is otherwise OFF by default, so existing
// callers that do not set a key see no behaviour change.
//
// The generated run_id (one per non-cached execution) is exposed on the
// returned WorkflowTrace.RunID so callers can correlate WAL files, audit
// records, etc. Callers combining idempotency with WAL crash-resume should
// name the WAL file with the run_id so an in-progress run can be resumed.
//
// Returns the receiver for chaining.
func (e *Executor) WithIdempotencyKey(key string) *Executor {
	e.idempotencyKey = key
	if e.idempotencyStore == nil {
		e.idempotencyStore = NewFileIdempotencyStore(defaultIdempotencyDir(), defaultIdempotencyTTL)
	}
	return e
}

// WithIdempotencyStore overrides the IdempotencyStore used for idempotency
// checks. This is primarily a testing hook (e.g. to point at a temp dir or a
// short TTL). The key itself must still be set via WithIdempotencyKey to
// activate idempotency. Returns the receiver for chaining.
func (e *Executor) WithIdempotencyStore(store IdempotencyStore) *Executor {
	e.idempotencyStore = store
	return e
}

// WithWorkflowPath sets the original workflow file path for pause-resume
// metadata. When a resumable step is paused, this path is stored in the
// run metadata so the resume command can locate the original workflow.
func (e *Executor) WithWorkflowPath(path string) *Executor {
	e.wfPath = path
	return e
}

// WithSafeMode records whether this Executor runs under the strict (safe)
// policy. It does NOT enforce anything by itself — enforcement is the
// PolicyExecutor's job (see NewPolicyExecutor). The flag is stamped into the
// pause RunMeta so `aflare resume` re-applies the same policy class the run
// started under, instead of silently resuming a --safe run without
// restrictions. Returns the receiver for chaining.
func (e *Executor) WithSafeMode(mode bool) *Executor {
	e.safeMode = mode
	return e
}

// WithProgress registers a StepProgressFunc callback that is invoked at each
// step lifecycle event (started/completed/failed/skipped) during sequential
// workflow execution (断点13: 实时进度输出). This is intended for the CLI's
// non-interactive RunCLI path to print real-time progress like:
//
//	[1/5] ✓ http_request  → CoinGecko API          (0.3s)
//	[2/5] ✗ agent          → FAILED
//
// The callback is called synchronously from the executor goroutine and must
// be non-blocking. Pass nil to disable. Returns the receiver for chaining.
func (e *Executor) WithProgress(cb StepProgressFunc) *Executor {
	e.progressCB = cb
	return e
}

// SetupShutdown registers OS signal handlers (SIGINT, SIGTERM) for standalone
// CLI use. When a signal is received, SignalShutdown is called to mark the
// global shutdown flag, which causes all running workflow executions to stop
// starting new steps after the current one completes. Deferred cleanup (WAL
// flush, audit finalization) runs when each execution's function returns.
//
// This is intended for standalone CLI use (aflare run). When the Executor is
// used through the HTTP server, the server's own signal handler calls
// SignalShutdown and srv.Shutdown, so this method is not needed.
//
// It is safe to call SetupShutdown multiple times on the same Executor.
func (e *Executor) SetupShutdown() {
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("executor received shutdown signal, stopping gracefully...")
		SignalShutdown()
		e.Shutdown()
	}()
}

// Shutdown waits for all in-flight executions tracked by this Executor to
// complete (current step finishes, WAL flushed, audit finalized), then
// returns. It is called automatically by SetupShutdown's signal handler or
// can be called directly by the caller.
func (e *Executor) Shutdown() {
	logger.Info("executor shutting down, waiting for current steps to complete...")
	e.wg.Wait()
	logger.Info("executor shutdown complete")
}

// Execute runs the workflow without a TUI program. It is the checkpoint-aware
// equivalent of ExecuteWorkflow.
func (e *Executor) Execute(ctx context.Context, wf *Workflow, reg *nodes.Registry) (string, []StepResult, error) {
	output, results, _, err := e.ExecuteWithTrace(ctx, wf, reg, nil)
	return output, results, err
}

// ExecuteWithTrace runs the workflow and returns a detailed per-step trace.
// It is the checkpoint-aware equivalent of ExecuteWorkflowWithTrace.
func (e *Executor) ExecuteWithTrace(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, *WorkflowTrace, error) {
	// Track this execution in the WaitGroup so Shutdown can wait for it.
	e.wg.Add(1)
	defer e.wg.Done()

	// Idempotency: guard against duplicate side effects when the same
	// Idempotency-Key is re-triggered, including concurrently. The previous
	// implementation did a non-atomic Check → execute → Record: two concurrent
	// requests with the same key could both observe "not found" and both
	// execute (e.g. double-charging a transfer). We now atomically Reserve an
	// in_progress placeholder before executing, so a concurrent same-key
	// request is rejected (ErrIdempotencyInProgress) or served from cache
	// (ErrIdempotencyHit) instead of re-running side-effecting nodes.
	runID := ""
	reserved := false
	// The audit recorder is built before the idempotency check so that a
	// cache hit or a concurrent-run rejection — both of which return before
	// the normal recordStart/recordCompletion path — can still leave an
	// audit trail. In financial scenarios "a transfer was served from cache"
	// and "a duplicate trigger was suppressed" are themselves auditable
	// events. recordStart is still only called for real executions below.
	audit := e.newAuditRecorder(wf)
	if e.idempotencyKey != "" && e.idempotencyStore != nil {
		// 1. Fast path: a completed record is served from cache without
		//    acquiring the cross-process lock; an in_progress record means
		//    another run is mid-flight and this request is rejected. A failed
		//    record (or no record) falls through to Reserve, which re-reads
		//    authoritatively under the lock. A Check read failure is non-fatal:
		//    we log and proceed to Reserve, the single source of truth.
		if rec, found, cerr := e.idempotencyStore.Check(e.idempotencyKey); cerr != nil {
			logger.Warn("idempotency check failed, proceeding to reserve", "key", e.idempotencyKey, "error", cerr)
		} else if found {
			switch rec.Status {
			case idempotencyStatusCompleted:
				logger.Info("idempotency hit, returning cached result", "key", e.idempotencyKey, "run_id", rec.RunID)
				audit.recordIdempotencyHit(rec)
				trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
				trace.RunID = rec.RunID
				trace.IdempotencyHit = true
				trace.finish(time.Now())
				return rec.FinalOutput, nil, trace, ErrIdempotencyHit
			case idempotencyStatusInProgress:
				logger.Info("idempotency in-progress, rejecting concurrent run", "key", e.idempotencyKey, "run_id", rec.RunID)
				audit.recordIdempotencyRejected(rec)
				trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
				trace.RunID = rec.RunID
				trace.finish(time.Now())
				return "", nil, trace, ErrIdempotencyInProgress
			}
		}

		// 2. Atomic placeholder: prevents a concurrent same-key request from
		//    also executing. Reserve is the authoritative check — it re-reads
		//    under the lock and wins or loses atomically, closing the race that
		//    a standalone Check leaves open.
		runID = newRunID()
		rec, ok, rerr := e.idempotencyStore.Reserve(e.idempotencyKey, runID)
		if rerr != nil {
			// ErrIdempotencyInProgress (a run started between our Check and
			// Reserve) or a real store error: either way we must not execute.
			// A completed record that appeared in the race window is surfaced
			// as a cache hit.
			if rec.Status == idempotencyStatusCompleted {
				logger.Info("idempotency hit after reserve race, returning cached result", "key", e.idempotencyKey, "run_id", rec.RunID)
				audit.recordIdempotencyHit(rec)
				trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
				trace.RunID = rec.RunID
				trace.IdempotencyHit = true
				trace.finish(time.Now())
				return rec.FinalOutput, nil, trace, ErrIdempotencyHit
			}
			if rec.Status == idempotencyStatusInProgress {
				logger.Info("idempotency in-progress after reserve, rejecting concurrent run", "key", e.idempotencyKey, "run_id", rec.RunID)
				audit.recordIdempotencyRejected(rec)
			} else {
				logger.Warn("idempotency reserve failed, rejecting run", "key", e.idempotencyKey, "error", rerr)
			}
			trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
			trace.RunID = rec.RunID
			trace.finish(time.Now())
			return "", nil, trace, rerr
		}
		if !ok {
			// Lost the reservation race; rec holds the winning record.
			if rec.Status == idempotencyStatusCompleted {
				logger.Info("idempotency hit after reserve race, returning cached result", "key", e.idempotencyKey, "run_id", rec.RunID)
				audit.recordIdempotencyHit(rec)
				trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
				trace.RunID = rec.RunID
				trace.IdempotencyHit = true
				trace.finish(time.Now())
				return rec.FinalOutput, nil, trace, ErrIdempotencyHit
			}
			logger.Info("idempotency in-progress after reserve race, rejecting concurrent run", "key", e.idempotencyKey, "run_id", rec.RunID)
			audit.recordIdempotencyRejected(rec)
			trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
			trace.RunID = rec.RunID
			trace.finish(time.Now())
			return "", nil, trace, ErrIdempotencyInProgress
		}
		reserved = true
	}

	audit.recordStart()

	// Ops gauge: count only real executions — the idempotency short-circuits
	// above (cache hit / in-progress rejection) never reach this point, so
	// they are not counted as active runs.
	metrics.IncActiveRuns()
	defer metrics.DecActiveRuns()

	var (
		out     string
		results []StepResult
		trace   *WorkflowTrace
		err     error
	)
	if hasDAGDeclarations(wf.Steps) {
		// DAG mode supports JSON checkpoint resume (completed-node set).
		// The WAL is a sequential-path construct (linear step cursor);
		// it stays unsupported here.
		if e.walPath != "" {
			logger.Warn("WAL checkpoint/resume is not supported in DAG mode, ignoring walPath", "path", e.walPath)
		}
		walPath := e.walPath
		statePath := e.statePath
		if walPath != "" {
			statePath = "" // WAL path would be the sequential source of truth; do not double-read it as a DAG checkpoint
		}
		out, results, trace, err = executeWorkflowDAG(ctx, wf, reg, program, e.workflowTimeout, statePath)
	} else {
		// WAL takes precedence over JSON checkpoint when both are configured.
		statePath := e.statePath
		walPath := e.walPath
		if walPath != "" {
			statePath = "" // WAL path is the source of truth
		}
		out, results, trace, err = executeWorkflowSequential(ctx, wf, reg, program, statePath, walPath, e.wfPath, e.safeMode, e.workflowTimeout, e.progressCB)
	}
	recordWorkflowMetrics(trace, err)
	audit.recordCompletion(results, trace, err)

	// Persist the idempotency outcome so a repeat trigger for this key is a
	// cache hit. The run_id is stamped on the trace for correlation. Only the
	// run that won the Reserve writes the final record; a failed workflow
	// records status=failed so the next trigger may retry. Record failures are
	// non-fatal: the workflow has already run, so we log and move on (the next
	// trigger will simply re-execute).
	if reserved {
		if trace != nil {
			trace.RunID = runID
		}
		status := idempotencyStatusCompleted
		errMsg := ""
		if err != nil {
			status = idempotencyStatusFailed
			errMsg = err.Error()
		}
		now := time.Now().UTC()
		rec := IdempotencyRecord{
			Key:          e.idempotencyKey,
			RunID:        runID,
			WorkflowPath: wf.Name,
			Status:       status,
			FinalOutput:  out,
			Error:        errMsg,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if rerr := e.idempotencyStore.Record(rec); rerr != nil {
			logger.Warn("idempotency record failed, next trigger will re-execute", "key", e.idempotencyKey, "error", rerr)
		}
	}
	return out, results, trace, err
}
