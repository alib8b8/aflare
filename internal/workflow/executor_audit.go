// Copyright (c) 2026 llm-box Contributors
//
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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"unicode/utf8"

	"github.com/alib8b8/llm-box/internal/history"
	"github.com/alib8b8/llm-box/internal/logger"
)

// Audit log action values for workflow-execution records. These extend the
// base set defined in internal/history without modifying that package
// (AuditAction is a string type, so new values can be declared here).
const (
	auditActionStep   history.AuditAction = "workflow_step"
	auditActionFailed history.AuditAction = "workflow_failed"
	// auditActionIdempotentHit is written when an idempotency key was served
	// from the cache (no side-effecting node ran). Financial scenarios require
	// that "a transfer was returned from cache" is itself auditable: the
	// operator must be able to answer "was this request served, and from which
	// prior run?".
	auditActionIdempotentHit history.AuditAction = "workflow_idempotent_hit"
	// auditActionIdempotentRejected is written when a same-key request was
	// suppressed because another run is in progress. The detail carries the
	// in-progress run_id so the suppression can be attributed to the live run.
	auditActionIdempotentRejected history.AuditAction = "workflow_idempotent_rejected"
)

// Audit HMAC key environment variables, mirrored from internal/history so the
// recorder can detect a missing key without that package exporting the names.
const (
	auditEnvHMACKey     = "LLM_BOX_AUDIT_HMAC_KEY"
	auditEnvSecretsPass = "LLM_BOX_SECRETS_PASSWORD"
)

// auditMaxFieldLen bounds the size of flowing input/output strings written to
// the audit log so a single huge step payload cannot blow up the audit file.
const auditMaxFieldLen = 500

// warnAuditNoKeyOnce ensures the "audit disabled (no HMAC key)" warning is
// logged at most once per process.
var warnAuditNoKeyOnce sync.Once

// auditRecorder writes tamper-evident audit records for a single workflow
// execution into the history package's HMAC hash-chain audit log.
//
// All methods are no-ops when audit is disabled. Write failures (including a
// missing HMAC key) never block workflow execution: they are logged at warn
// level and skipped. Global write ordering across concurrent Executors is
// guaranteed by the history package's internal auditWriteMu, so concurrent
// runs extend the same chain safely rather than corrupting it.
type auditRecorder struct {
	enabled      bool
	dir          string // when non-empty, overrides the history directory
	workflowName string
	steps        []WorkflowStep

	// keyAvailable caches the env-var lookup for the lifetime of this
	// recorder (i.e. a single execution). A fresh recorder per Execute call
	// means env changes between runs are picked up.
	keyAvailable bool
	keyChecked   bool
}

// newAuditRecorder builds a recorder from the Executor's audit configuration.
// When a custom dir is supplied it is applied to the history package (process
// global) so the audit log lands there; when empty the existing history
// directory is used unchanged.
//
// H-5 note: history.SetHistoryDir mutates process-global state, so concurrent
// Executors that supply different dirs clobber each other (last writer wins).
// The history package exposes no getter to detect this at runtime without
// modifying it, so this method does not attempt runtime conflict detection;
// the WithAuditLog doc comment carries the operator-facing warning, and the
// CLI uses a per-directory lock (acquireAuditLock) to prevent concurrent
// processes from corrupting the hash chain. Within a single process, use one
// audit directory for all Executors.
func (e *Executor) newAuditRecorder(wf *Workflow) *auditRecorder {
	ar := &auditRecorder{
		enabled:      e.auditEnabled,
		dir:          e.auditDir,
		workflowName: wf.Name,
		steps:        wf.Steps,
	}
	if ar.enabled && ar.dir != "" {
		history.SetHistoryDir(ar.dir)
	}
	return ar
}

// hasHMACKey reports whether a non-default HMAC key source is configured.
// When false, audit writing is skipped (graceful degradation) with a single
// warning per process; the workflow itself is never affected.
func (ar *auditRecorder) hasHMACKey() bool {
	if !ar.enabled {
		return false
	}
	if !ar.keyChecked {
		ar.keyChecked = true
		ar.keyAvailable = os.Getenv(auditEnvHMACKey) != "" || os.Getenv(auditEnvSecretsPass) != ""
		if !ar.keyAvailable {
			warnAuditNoKeyOnce.Do(func() {
				logger.Warn("audit log disabled: set LLM_BOX_AUDIT_HMAC_KEY or LLM_BOX_SECRETS_PASSWORD to enable tamper-evident audit logging")
			})
		}
	}
	return ar.keyAvailable
}

// recordStart writes the workflow_start audit record before execution begins.
func (ar *auditRecorder) recordStart() {
	if !ar.hasHMACKey() {
		return
	}
	ar.appendLog(history.AuditLog{
		Action:   history.AuditActionWorkflowStart,
		Resource: ar.workflowName,
		Success:  true,
		Detail:   ar.workflowDetail("started", "", nil),
	})
}

// recordCompletion writes one audit record per step result (in execution
// order) followed by a workflow_end (success) or workflow_failed record. It is
// called after the workflow finishes; because AppendAuditLog extends the hash
// chain sequentially, the resulting audit-file order is:
//
//	start, step..., complete/fail
//
// which matches "one record per step as it completes" for chain-integrity and
// replay purposes.
//
// trace carries the run's aggregated cost/token totals; when non-nil its
// TotalCostUSD and TotalTokens are stamped into the workflow_end/failed detail
// so each completed run's estimated LLM cost is tamper-evidently recorded
// alongside its outcome. This is the cost-attribution hook: a financial
// operator can answer "how much did this audited run cost in LLM tokens?"
// directly from the audit log without re-running the workflow or consulting a
// separate billing system. Nil trace (e.g. a panic-recovery path that never
// built one) simply omits the cost fields — the audit record still records
// the outcome.
func (ar *auditRecorder) recordCompletion(results []StepResult, trace *WorkflowTrace, runErr error) {
	if !ar.hasHMACKey() {
		return
	}
	for _, r := range results {
		ar.appendStep(r)
	}
	if runErr != nil {
		ar.appendLog(history.AuditLog{
			Action:   auditActionFailed,
			Resource: ar.workflowName,
			Success:  false,
			Detail:   ar.workflowDetail("failed", runErr.Error(), trace),
		})
	} else {
		ar.appendLog(history.AuditLog{
			Action:   history.AuditActionWorkflowEnd,
			Resource: ar.workflowName,
			Success:  true,
			Detail:   ar.workflowDetail("completed", "", trace),
		})
	}
}

// recordIdempotencyHit writes a workflow_idempotent_hit audit record when an
// idempotency key was served from the cache (no side-effecting node ran). The
// detail carries the sanitized key prefix, the cached run_id, the status, and
// a truncated final_output so an operator can correlate the cache hit with the
// original run without re-running it. This closes the audit gap on the
// idempotency fast path: previously a cache hit returned before any audit
// record was written, so a "transfer served from cache" left no trail.
func (ar *auditRecorder) recordIdempotencyHit(rec IdempotencyRecord) {
	if !ar.hasHMACKey() {
		return
	}
	ar.appendLog(history.AuditLog{
		Action:   auditActionIdempotentHit,
		Resource: ar.workflowName,
		Success:  true,
		Detail:   ar.idempotencyDetail(rec, "cache_hit"),
	})
}

// recordIdempotencyRejected writes a workflow_idempotent_rejected audit record
// when a same-key request was suppressed because another run is in progress.
// Success is false because the request was not served (no cached output was
// returned); the detail carries the in-progress run_id so the operator can
// attribute the rejection to the live run.
func (ar *auditRecorder) recordIdempotencyRejected(rec IdempotencyRecord) {
	if !ar.hasHMACKey() {
		return
	}
	ar.appendLog(history.AuditLog{
		Action:   auditActionIdempotentRejected,
		Resource: ar.workflowName,
		Success:  false,
		Detail:   ar.idempotencyDetail(rec, "rejected_concurrent"),
	})
}

// idempotencyDetail renders the detail JSON for an idempotency audit record.
// The key is hashed (only a short prefix is kept) so the raw business key
// (which may carry an account number, reference, or other PII) never reaches
// the audit log; final_output is truncated to auditMaxFieldLen to bound log
// size. The cached run_id lets an operator correlate the hit/rejection with
// the originating run's WAL, traces, and step-level audit records.
func (ar *auditRecorder) idempotencyDetail(rec IdempotencyRecord, phase string) string {
	d := map[string]interface{}{
		"workflow":        ar.workflowName,
		"phase":           phase,
		"idempotency_key": idempotencyKeyPrefix(rec.Key),
		"run_id":          rec.RunID,
		"status":          rec.Status,
		"final_output":    truncateAudit(rec.FinalOutput),
	}
	if rec.Error != "" {
		d["error"] = rec.Error
	}
	b, _ := json.Marshal(d)
	return string(b)
}

// idempotencyKeyPrefix returns a short, non-reversible prefix of the sha256 of
// the idempotency key, so an operator can correlate audit records for the same
// key across runs without the raw key (which may carry account numbers or
// other PII) being persisted to the audit log.
func idempotencyKeyPrefix(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:16]
}

// appendStep writes an audit record for a single step result, capturing
// step_index/step_name/node_name/input/output/status/error/duration. Step
// parameters are sanitized via history.SanitizeParams so secrets (keys,
// tokens, passwords) are masked as "***" before reaching the audit log;
// flowing input/output strings are truncated to bound the audit log size.
func (ar *auditRecorder) appendStep(r StepResult) {
	detail := map[string]interface{}{
		"step_index": r.StepIndex,
		"node_name":  r.NodeName,
		"input":      truncateAudit(r.Input),
		"output":     truncateAudit(r.Output),
		"status":     "success",
		"duration":   r.Duration.String(),
	}
	if r.Error != nil {
		detail["status"] = "failed"
		detail["error"] = r.Error.Error()
	}
	// step_index maps to wf.Steps for simple sequential/DAG steps. For
	// compound-step sub-results (if/loop/map/reduce branches) the StepIndex is
	// relative to the sub-workflow, so ar.steps[r.StepIndex] belongs to a
	// different step namespace and must not be attributed to this result. We
	// guard both the bounds AND a node-match check: only attribute step_name/
	// params when the resolved step's node equals the result's NodeName, so a
	// branch sub-result is never mis-attributed to an unrelated parent step.
	if r.StepIndex >= 0 && r.StepIndex < len(ar.steps) && ar.steps[r.StepIndex].Node == r.NodeName {
		step := ar.steps[r.StepIndex]
		if step.Name != "" {
			detail["step_name"] = step.Name
		}
		if len(step.Params) > 0 {
			params := make(map[string]interface{}, len(step.Params))
			for k, v := range step.Params {
				params[k] = v
			}
			// Sanitize params BEFORE writing to the audit log.
			detail["params"] = history.SanitizeParams(params)
		}
	}
	data, err := json.Marshal(detail)
	if err != nil {
		logger.Warn("failed to marshal audit step detail", "step", r.StepIndex, "node", r.NodeName, "error", err)
		return
	}
	ar.appendLog(history.AuditLog{
		Action:   auditActionStep,
		Resource: r.NodeName,
		Success:  r.Error == nil,
		Detail:   string(data),
	})
}

// workflowDetail renders the detail JSON for a workflow-level (start/end)
// audit record. When trace is non-nil, the run's aggregated estimated LLM
// cost (TotalCostUSD) and total token count (TotalTokens) are included so the
// audit log carries cost attribution for each completed run. The cost is
// rounded to 8 decimal places (sub-cent precision) — enough to represent a
// single cheap call (~$0.00001) without floating-point noise, while keeping
// the JSON readable. A zero cost (no LLM calls, or unknown models) is still
// emitted so a cost-aware alert can distinguish "ran, cost $0" from "ran but
// cost was not computed".
func (ar *auditRecorder) workflowDetail(phase, errMsg string, trace *WorkflowTrace) string {
	d := map[string]interface{}{
		"workflow": ar.workflowName,
		"steps":    len(ar.steps),
		"phase":    phase,
	}
	if errMsg != "" {
		d["error"] = errMsg
	}
	if trace != nil {
		d["cost_usd"] = roundCost(trace.TotalCostUSD)
		d["total_tokens"] = trace.TotalTokens
	}
	b, _ := json.Marshal(d)
	return string(b)
}

// roundCost truncates a USD cost to 8 decimal places for stable, readable
// audit output. It rounds half-to-even via fmt formatting to avoid binary
// float artefacts (e.g. 0.010000000000000002) appearing in the audit log.
func roundCost(c float64) float64 {
	// Multiply, round, divide to 8dp. Using math.Round on the scaled value
	// gives half-away-from-zero; for cost reporting the difference from
	// half-to-even is sub-cent and immaterial, and math.Round is simpler
	// than re-implementing banker's rounding.
	const scale = 1e8
	return math.Round(c*scale) / scale
}

// appendLog writes a single audit record. Any error or panic is swallowed and
// logged at warn level so audit failures can never block or fail the workflow.
func (ar *auditRecorder) appendLog(log history.AuditLog) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warn("audit log write panicked", "action", log.Action, "error", fmt.Sprintf("%v", r))
		}
	}()
	if err := history.AppendAuditLog(log); err != nil {
		logger.Warn("failed to write audit log", "action", log.Action, "error", err)
	}
}

// truncateAudit bounds a flowing string value written to the audit log.
//
// L-11: the cut is by BYTE length (auditMaxFieldLen), but if the cut point
// lands in the middle of a multi-byte UTF-8 sequence the result would be
// invalid UTF-8 (a dangling lead/continuation byte). Audit log detail is
// JSON-marshaled downstream; JSON encoders in Go tolerate invalid UTF-8 by
// escaping it, but downstream consumers (log aggregators, jq, audit replay
// tools) may reject or mojibake the record — unacceptable for a financial
// audit trail. Chinese / Japanese / emoji prompts are the common victims.
// We therefore walk back from the cut point to the previous UTF-8 rune
// boundary before appending the ellipsis. utf8.RuneStart reports whether a
// byte is the first byte of a rune (or a lone ASCII byte), which is exactly
// the boundary condition we need. The resulting string is always valid
// UTF-8 and never longer than auditMaxFieldLen + len("...").
func truncateAudit(s string) string {
	if len(s) <= auditMaxFieldLen {
		return s
	}
	truncated := s[:auditMaxFieldLen]
	// Back up to the last complete UTF-8 rune boundary so we do not emit a
	// truncated multi-byte sequence. utf8.RuneStart(b) is true when b is
	// the first byte of a UTF-8 encoded rune (or a single-byte ASCII char);
	// walking back until it is true lands us on a rune start.
	for len(truncated) > 0 && !utf8.RuneStart(truncated[len(truncated)-1]) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "..."
}
