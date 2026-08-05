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

// This file implements workflow idempotency: a mechanism that ensures
// re-triggering the same workflow with the same Idempotency-Key does not
// re-execute side-effecting nodes (HTTP POST transfers, file writes, etc.).
//
// WAL persistence (see wal.go) only guarantees that workflow *state* can be
// recovered after a crash; it does NOT guarantee business idempotency, because
// crash recovery replays step internals and may re-fire side effects. This
// package layer adds the missing business-level deduplication.
//
// Design:
//   - IdempotencyStore is a key→record ledger. The default FileIdempotencyStore
//     persists each key as a single JSON file at
//     ~/.config/llm-box/idempotency/<sha256(key)>.json, written atomically
//     (tmp + rename).
//   - The Executor (see executor.go) consults the store before executing when
//     a key has been set via WithIdempotencyKey. On a "completed" hit it
//     returns the cached final output together with ErrIdempotencyHit and
//     runs no step. On a miss (or a failed/in-progress prior record) it
//     executes normally and records the new run_id + result, so the next
//     trigger for the same key becomes a cache hit.
//   - Idempotency is OFF by default; it only activates when the caller sets
//     a key. Existing callers that do not set a key see no behaviour change.
//   - run_id is a UUID v4 generated with crypto/rand (no external dependency)
//     and is exposed on the returned WorkflowTrace.RunID so callers can
//     correlate WAL files, audit records, etc.

package workflow

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
)

// ErrIdempotencyHit is returned by Executor.ExecuteWithTrace (and Execute)
// when an idempotency key was found in the store with a "completed" status
// and the run was therefore skipped. The cached final output is still
// returned as the first result value, so callers that only need the output
// can ignore this error. Callers that need to distinguish a real execution
// from a dedup hit should check errors.Is(err, ErrIdempotencyHit).
var ErrIdempotencyHit = errors.New("workflow skipped: idempotency key already completed")

// ErrIdempotencyInProgress is returned when an idempotency key already has an
// in_progress run (another goroutine/process is mid-execution for the same
// key). The caller must NOT re-execute: doing so would duplicate side effects
// such as a duplicate money transfer. The run should be rejected and the
// caller is expected to retry later or surface the conflict to the user.
var ErrIdempotencyInProgress = errors.New("workflow skipped: idempotency key has an in-progress run")

// ErrIdempotencyLockTimeout is returned by Reserve when the cross-process lock
// for a key could not be acquired within crossProcessLockTimeout. This usually
// means another process is stuck mid-Reserve; the caller should retry.
var ErrIdempotencyLockTimeout = errors.New("idempotency: timed out acquiring cross-process lock")

// Idempotency status values stored in IdempotencyRecord.Status.
const (
	idempotencyStatusInProgress = "in_progress"
	idempotencyStatusCompleted  = "completed"
	idempotencyStatusFailed     = "failed"
)

// defaultIdempotencyTTL is the age after which a persisted idempotency record
// is considered expired and reaped on the next Check. 7 days bounds disk
// growth while remaining well beyond any reasonable retry window for a
// financial transfer workflow.
const defaultIdempotencyTTL = 7 * 24 * time.Hour

// Cross-process lock tuning for FileIdempotencyStore.Reserve. The lock is held
// only for the read-then-write of the in_progress placeholder (milliseconds),
// never for the duration of the workflow, so these bounds are very generous:
//
//   - crossProcessLockTimeout: how long Reserve polls for a busy lock before
//     giving up and returning ErrIdempotencyLockTimeout.
//   - crossProcessLockStale: a lock sentinel older than this is assumed
//     orphaned (the holder crashed between creating and removing it) and is
//     reclaimed so the key does not get stuck forever.
//   - crossProcessLockPollInterval: the retry cadence while waiting.
const (
	crossProcessLockTimeout      = 5 * time.Second
	crossProcessLockStale        = 30 * time.Second
	crossProcessLockPollInterval = 5 * time.Millisecond
)

// IdempotencyRecord is the persisted state of a single idempotency key. It is
// the unit of deduplication: a "completed" record makes the next trigger for
// the same key a cache hit.
type IdempotencyRecord struct {
	Key   string `json:"key"`
	RunID string `json:"run_id"`
	// WorkflowPath is the source workflow identifier (best-effort: the
	// workflow Name is used when the original file path is unavailable). It
	// is recorded for auditability and is not used for matching — the Key is
	// the sole identity.
	WorkflowPath string `json:"workflow_path,omitempty"`
	// Status is one of idempotencyStatusInProgress/Completed/Failed.
	Status      string    `json:"status"`
	FinalOutput string    `json:"final_output,omitempty"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// HMAC is a keyed MAC (HMAC-SHA256) over all other fields that lets the
	// store detect tampering with a persisted record — most importantly with
	// FinalOutput, which is served verbatim to a cache-hit caller. A financial
	// attacker who rewrites FinalOutput on disk could otherwise inject a forged
	// result (e.g. a fake transfer confirmation) that the next trigger returns
	// as if it were the cached outcome of a real run. When an HMAC key is
	// configured (LLM_BOX_AUDIT_HMAC_KEY or LLM_BOX_SECRETS_PASSWORD), Check
	// rejects a record whose HMAC does not verify, forcing re-execution instead
	// of returning a forged result. When no key is configured the field is left
	// empty and verification is skipped (graceful degradation), preserving
	// backward compatibility.
	HMAC string `json:"hmac,omitempty"`
}

// idempotencyHMACKeyEnvVars are the environment variables consulted, in order,
// for the idempotency record HMAC key. They mirror the audit recorder so a
// single deployment secret protects both audit and idempotency records; no
// new environment variable is introduced.
var idempotencyHMACKeyEnvVars = []string{
	"LLM_BOX_AUDIT_HMAC_KEY",
	"LLM_BOX_SECRETS_PASSWORD",
}

// warnIdempotencyNoKeyOnce ensures the "no HMAC key" warning is logged at most
// once per process. When the key is absent, signing is skipped (HMAC left
// empty) and verification is skipped on read, so tamper-evidence degrades to
// crash-only durability — the workflow itself is unaffected.
var warnIdempotencyNoKeyOnce sync.Once

// idempotencyHMACKey returns the HMAC key used to sign idempotency records,
// or nil when no key source is configured. The same secret backing the audit
// log chain is reused so operators only configure one secret per deployment.
// Raw bytes from the env var are used directly (the PBKDF2 derivation in the
// history package is an audit-log concern; idempotency only requires that sign
// and verify agree, which they do since both read the same env).
func idempotencyHMACKey() []byte {
	for _, env := range idempotencyHMACKeyEnvVars {
		if v := os.Getenv(env); v != "" {
			return []byte(v)
		}
	}
	warnIdempotencyNoKeyOnce.Do(func() {
		logger.Warn("idempotency HMAC key not configured; set LLM_BOX_AUDIT_HMAC_KEY or LLM_BOX_SECRETS_PASSWORD to enable tamper-evident idempotency records")
	})
	return nil
}

// signRecord computes the HMAC-SHA256 of the record's tamper-relevant fields
// using idempotencyHMACKey. It returns the empty string when no key is
// configured (graceful degradation: the HMAC field is left empty and
// verification is skipped on read). The MAC covers every field that influences
// a cache-hit decision or that an attacker could profitably forge (Key, RunID,
// WorkflowPath, Status, FinalOutput, Error, and the timestamps); the HMAC
// field itself is naturally excluded. Fields are joined with NUL separators so
// concatenation is unambiguous (a NUL cannot appear inside any of the string
// fields in normal operation).
func signRecord(rec *IdempotencyRecord) string {
	key := idempotencyHMACKey()
	if key == nil {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d\x00%d",
		rec.Key, rec.RunID, rec.WorkflowPath, rec.Status,
		rec.FinalOutput, rec.Error,
		rec.CreatedAt.UnixNano(), rec.UpdatedAt.UnixNano())
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyRecord reports whether rec.HMAC matches a freshly computed MAC. When
// no HMAC key is configured it returns true (verification skipped, graceful
// degradation). When a key is configured but rec.HMAC is empty — a legacy
// record written before this field existed, or a record whose HMAC was stripped
// during tampering — it returns false so the caller treats the record as
// not-found and re-executes rather than trusting an unsigned cached result.
// Comparison uses hmac.Equal to avoid timing side channels.
func verifyRecord(rec *IdempotencyRecord) bool {
	if idempotencyHMACKey() == nil {
		return true
	}
	if rec.HMAC == "" {
		return false
	}
	expected := signRecord(rec)
	return hmac.Equal([]byte(rec.HMAC), []byte(expected))
}

// IdempotencyStore is the ledger used by the Executor to deduplicate workflow
// executions by key. Implementations must be safe for concurrent use by
// multiple goroutines within a single process.
type IdempotencyStore interface {
	// Check returns the record for the given key. found is false when no
	// record exists or when an existing record has expired (and was reaped).
	Check(key string) (rec IdempotencyRecord, found bool, err error)
	// Record persists rec, creating or overwriting the entry for rec.Key.
	Record(rec IdempotencyRecord) error
	// Reserve atomically claims an in_progress placeholder for key so that a
	// concurrent same-key execution cannot also start. It is the correctness
	// backbone of idempotency: Check is a non-authoritative read, so the
	// Executor calls Reserve immediately before running side-effecting nodes.
	//
	// Return contract:
	//   - key absent or status=failed: an in_progress record (run_id = runID)
	//     is written and (rec, true, nil) is returned. The caller owns the run
	//     and MUST later Record the final outcome (completed or failed).
	//   - status=completed: the existing record is returned as (rec, false,
	//     nil) so the caller can serve the cached output.
	//   - status=in_progress: the existing record is returned as (rec, false,
	//     ErrIdempotencyInProgress) so the caller rejects the duplicate run.
	//
	// Implementations must make the read-then-write atomic across goroutines
	// (and, for file-backed stores, across processes sharing the directory).
	Reserve(key string, runID string) (rec IdempotencyRecord, reserved bool, err error)
	// Clear removes the record for the key. Removing a non-existent key is
	// not an error.
	Clear(key string) error
}

// FileIdempotencyStore is the default IdempotencyStore. Each key is persisted
// as a single JSON file at <dir>/<sha256(key)>.json, written atomically via a
// tmp file + rename so a crash mid-write never leaves a partial record.
//
// A sync.Mutex serializes operations within a single process. Cross-process
// safety is NOT provided; processes sharing a directory must use disjoint key
// namespaces or wrap the store with an external file lock.
//
// A positive ttl reaps records whose UpdatedAt is older than ttl from now on
// Check, so the directory does not grow without bound. A zero ttl disables
// expiry (records persist until explicitly Cleared).
type FileIdempotencyStore struct {
	dir string
	ttl time.Duration
	mu  sync.Mutex
}

// NewFileIdempotencyStore returns a FileIdempotencyStore rooted at dir. The
// directory is created lazily on the first write. A zero or negative ttl
// enables the default TTL (defaultIdempotencyTTL); pass a custom duration to
// override (e.g. a short TTL for tests).
func NewFileIdempotencyStore(dir string, ttl time.Duration) *FileIdempotencyStore {
	if dir == "" {
		dir = defaultIdempotencyDir()
	}
	if ttl <= 0 {
		ttl = defaultIdempotencyTTL
	}
	return &FileIdempotencyStore{dir: dir, ttl: ttl}
}

// DefaultIdempotencyDir returns the default on-disk directory for the file
// idempotency store: <os.UserConfigDir>/llm-box/idempotency. The directory is
// created lazily on the first write. This is the location used when
// WithIdempotencyKey is called without an explicit WithIdempotencyStore.
func DefaultIdempotencyDir() string { return defaultIdempotencyDir() }

func defaultIdempotencyDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "llm-box", "idempotency")
}

// Check implements IdempotencyStore. If the stored record has expired (its
// UpdatedAt is older than ttl from now), it is reaped and reported as not
// found so the caller proceeds with a fresh execution.
func (s *FileIdempotencyStore) Check(key string) (IdempotencyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok, err := s.readLocked(key)
	if err != nil {
		return IdempotencyRecord{}, false, err
	}
	if !ok {
		return IdempotencyRecord{}, false, nil
	}
	if s.ttl > 0 && time.Since(rec.UpdatedAt) > s.ttl {
		// Expired: reap and report not found so the next run re-executes.
		_ = os.Remove(s.pathFor(key))
		return IdempotencyRecord{}, false, nil
	}
	return rec, true, nil
}

// Record implements IdempotencyStore. The write is atomic (tmp + rename) and
// the parent directory is created with mode 0700 if missing; the record file
// itself is mode 0600.
func (s *FileIdempotencyStore) Record(rec IdempotencyRecord) error {
	if rec.Key == "" {
		return fmt.Errorf("idempotency: empty key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return fmt.Errorf("idempotency: mkdir: %w", err)
	}
	return s.writeLocked(rec.Key, rec)
}

// writeLocked writes rec atomically (tmp + rename) for the given key. The
// caller MUST hold s.mu and MUST have ensured s.dir exists. Zero CreatedAt /
// UpdatedAt are stamped with the current time. The record is fsynced before
// the rename and the parent directory is fsynced after, so a crash never
// leaves a 0-byte record that would break the next Load.
func (s *FileIdempotencyStore) writeLocked(key string, rec IdempotencyRecord) error {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	// Stamp the tamper-evidence MAC before serializing. signRecord returns ""
	// when no key is configured, leaving the field empty (graceful degrade).
	rec.HMAC = signRecord(&rec)
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("idempotency: marshal: %w", err)
	}
	finalPath := s.pathFor(key)
	tmpPath := finalPath + ".tmp." + randomSuffix()
	// Write → fsync → close → rename → fsync parent dir so a crash never
	// leaves a 0-byte record that would break the next Load. Mirrors the
	// durability pattern in internal/nodes/router_quota.go.
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("idempotency: write tmp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("idempotency: write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("idempotency: fsync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("idempotency: close tmp: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("idempotency: rename: %w", err)
	}
	// Best-effort fsync of the parent directory so the rename itself is
	// durable. A failure here is non-fatal: at worst a crash could lose the
	// rename, leaving the tmp file behind (never read) and the previous
	// record intact — the safe direction.
	if dir, err := os.Open(filepath.Dir(finalPath)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

// Reserve implements IdempotencyStore. It atomically claims an in_progress
// placeholder for key so a concurrent same-key request cannot also begin
// executing. This closes the check-then-act race in ExecuteWithTrace.
//
// Cross-process safety: a sibling "<key>.json.lock" sentinel file is created
// with O_CREATE|O_EXCL to serialise Reserve across processes that share the
// directory. The lock is held only for the read-then-write of the placeholder
// (milliseconds), never for the duration of the workflow. In-process
// concurrency is serialised by s.mu. See acquireCrossProcessLock for the
// orphaned-sentinel reclamation policy.
func (s *FileIdempotencyStore) Reserve(key string, runID string) (IdempotencyRecord, bool, error) {
	if key == "" {
		return IdempotencyRecord{}, false, fmt.Errorf("idempotency: empty key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return IdempotencyRecord{}, false, fmt.Errorf("idempotency: mkdir: %w", err)
	}

	release, err := s.acquireCrossProcessLock(key)
	if err != nil {
		return IdempotencyRecord{}, false, fmt.Errorf("idempotency: lock: %w", err)
	}
	defer release()

	// Re-read under both locks; this is the authoritative view used to decide
	// whether to claim or yield.
	rec, ok, err := s.readLocked(key)
	if err != nil {
		return IdempotencyRecord{}, false, fmt.Errorf("idempotency: read: %w", err)
	}
	if ok {
		// Reap expired records (including a stale in_progress left behind by a
		// crashed holder) so the key is not blocked forever.
		if s.ttl > 0 && time.Since(rec.UpdatedAt) > s.ttl {
			_ = os.Remove(s.pathFor(key))
			rec, ok = IdempotencyRecord{}, false
		}
	}
	if ok {
		switch rec.Status {
		case idempotencyStatusCompleted:
			return rec, false, nil
		case idempotencyStatusInProgress:
			return rec, false, ErrIdempotencyInProgress
		case idempotencyStatusFailed:
			// fall through: overwrite with a fresh in_progress placeholder so
			// a previously failed run can be retried.
		}
	}

	now := time.Now().UTC()
	newRec := IdempotencyRecord{
		Key:       key,
		RunID:     runID,
		Status:    idempotencyStatusInProgress,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.writeLocked(key, newRec); err != nil {
		return IdempotencyRecord{}, false, err
	}
	return newRec, true, nil
}

// acquireCrossProcessLock obtains a cross-process exclusive lock for key by
// atomically creating a "<key>.json.lock" sentinel file. The returned release
// function removes the sentinel. If the sentinel already exists, the caller
// polls until it disappears (up to crossProcessLockTimeout); sentinels older
// than crossProcessLockStale are treated as orphaned (the holder crashed
// between creating and removing it) and are reclaimed. The PID + creation
// time are stamped into the sentinel for diagnosability.
func (s *FileIdempotencyStore) acquireCrossProcessLock(key string) (release func(), err error) {
	lockPath := s.pathFor(key) + ".lock"
	deadline := time.Now().Add(crossProcessLockTimeout)
	for {
		f, oerr := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if oerr == nil {
			_, _ = fmt.Fprintf(f, "%d\n%d\n", os.Getpid(), time.Now().UnixNano())
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(oerr, os.ErrExist) {
			return nil, oerr
		}
		// Sentinel exists. Reclaim it if stale (the holder crashed mid-Reserve).
		if info, statErr := os.Stat(lockPath); statErr == nil &&
			time.Since(info.ModTime()) > crossProcessLockStale {
			if rmErr := os.Remove(lockPath); rmErr == nil || errors.Is(rmErr, os.ErrNotExist) {
				continue // retry the O_EXCL create immediately
			}
		}
		if time.Now().After(deadline) {
			return nil, ErrIdempotencyLockTimeout
		}
		time.Sleep(crossProcessLockPollInterval)
	}
}

// Clear implements IdempotencyStore. Removing a non-existent key is a no-op.
func (s *FileIdempotencyStore) Clear(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.pathFor(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("idempotency: clear: %w", err)
	}
	return nil
}

// pathFor returns the on-disk path for a key. The key is hashed (not used
// directly) so arbitrary user-provided keys cannot escape the directory or
// collide with filesystem naming rules.
func (s *FileIdempotencyStore) pathFor(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

func (s *FileIdempotencyStore) readLocked(key string) (IdempotencyRecord, bool, error) {
	data, err := os.ReadFile(s.pathFor(key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return IdempotencyRecord{}, false, nil
		}
		return IdempotencyRecord{}, false, fmt.Errorf("idempotency: read: %w", err)
	}
	// An empty file is the post-crash residue of an interrupted write that
	// the fsync+rename pattern is designed to prevent; treat it as not-found
	// (rather than a parse error) so the next run re-executes instead of
	// being blocked. A non-empty but unparseable file is genuine corruption
	// and is surfaced as an error.
	if len(data) == 0 {
		return IdempotencyRecord{}, false, nil
	}
	var rec IdempotencyRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return IdempotencyRecord{}, false, fmt.Errorf("idempotency: parse: %w", err)
	}
	// Reject tampered records: a bad/missing HMAC means FinalOutput (or any
	// other field) may have been forged, so we must not serve it as a cache
	// hit. Treat as not-found so the caller re-executes. When no HMAC key is
	// configured, verifyRecord returns true (graceful degrade).
	if !verifyRecord(&rec) {
		return IdempotencyRecord{}, false, nil
	}
	return rec, true, nil
}

// randomSuffix returns 8 hex chars from crypto/rand for unique tmp-file names.
// On the (extremely unlikely) read failure it falls back to a timestamp so the
// tmp path is still unique within the process.
func randomSuffix() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().Format("150405.000000")
	}
	return hex.EncodeToString(b[:])
}

// newRunID returns a version-4 UUID string generated with crypto/rand. It
// introduces no external dependency. On the (extremely unlikely) read failure
// it falls back to a time-based hex string so the call never errors.
//
// The run_id is also used to correlate WAL files: callers combining
// idempotency with WAL checkpointing should name the WAL file with the run_id
// so an in-progress run can be resumed after a crash (see Executor.WithWAL).
func newRunID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("run-%x", time.Now().UnixNano())
	}
	// RFC 4122 v4: set version (4) and variant (10xx) bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]))
}

// MemoryIdempotencyStore is an in-memory IdempotencyStore useful for tests and
// for single-process deployments that do not need persistence across restarts.
// It is safe for concurrent use by multiple goroutines. It implements the same
// Reserve contract as FileIdempotencyStore (atomic in_progress placeholder)
// but, since there is no shared filesystem state, it does NOT provide
// cross-process safety — only in-process mutual exclusion via sync.Mutex.
//
// A positive ttl reaps records whose UpdatedAt is older than ttl from now on
// Check/Reserve, mirroring FileIdempotencyStore; a zero ttl disables expiry.
type MemoryIdempotencyStore struct {
	mu   sync.Mutex
	recs map[string]IdempotencyRecord
	ttl  time.Duration
}

// NewMemoryIdempotencyStore returns an empty in-memory idempotency store. A
// zero or negative ttl disables expiry (records persist until Cleared).
func NewMemoryIdempotencyStore(ttl time.Duration) *MemoryIdempotencyStore {
	if ttl < 0 {
		ttl = 0
	}
	return &MemoryIdempotencyStore{recs: make(map[string]IdempotencyRecord), ttl: ttl}
}

// Check implements IdempotencyStore.
func (s *MemoryIdempotencyStore) Check(key string) (IdempotencyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.recs[key]
	if !ok {
		return IdempotencyRecord{}, false, nil
	}
	if s.ttl > 0 && time.Since(rec.UpdatedAt) > s.ttl {
		delete(s.recs, key)
		return IdempotencyRecord{}, false, nil
	}
	// Tampered or unsigned-when-key-set: drop and report not-found so the
	// caller re-executes rather than trusting a forged cached result. (In
	// memory this cannot happen across processes, but verifying keeps the
	// contract identical to FileIdempotencyStore so callers do not have to
	// reason about store-specific tamper behaviour.)
	if !verifyRecord(&rec) {
		delete(s.recs, key)
		return IdempotencyRecord{}, false, nil
	}
	return rec, true, nil
}

// Record implements IdempotencyStore.
func (s *MemoryIdempotencyStore) Record(rec IdempotencyRecord) error {
	if rec.Key == "" {
		return fmt.Errorf("idempotency: empty key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	rec.HMAC = signRecord(&rec)
	s.recs[rec.Key] = rec
	return nil
}

// Reserve implements IdempotencyStore. The read-then-write is atomic under
// s.mu, so concurrent goroutines with the same key are serialised: exactly one
// obtains reserved=true.
func (s *MemoryIdempotencyStore) Reserve(key string, runID string) (IdempotencyRecord, bool, error) {
	if key == "" {
		return IdempotencyRecord{}, false, fmt.Errorf("idempotency: empty key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.recs[key]; ok {
		switch {
		case s.ttl > 0 && time.Since(rec.UpdatedAt) > s.ttl:
			delete(s.recs, key)
		case !verifyRecord(&rec):
			// Tampered: drop and fall through to claim a fresh placeholder.
			delete(s.recs, key)
		default:
			switch rec.Status {
			case idempotencyStatusCompleted:
				return rec, false, nil
			case idempotencyStatusInProgress:
				return rec, false, ErrIdempotencyInProgress
			case idempotencyStatusFailed:
				// fall through: overwrite with a fresh placeholder so the
				// previously failed run can be retried.
			}
		}
	}
	now := time.Now().UTC()
	rec := IdempotencyRecord{
		Key:       key,
		RunID:     runID,
		Status:    idempotencyStatusInProgress,
		CreatedAt: now,
		UpdatedAt: now,
	}
	rec.HMAC = signRecord(&rec)
	s.recs[key] = rec
	return rec, true, nil
}

// Clear implements IdempotencyStore. Removing a non-existent key is a no-op.
func (s *MemoryIdempotencyStore) Clear(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.recs, key)
	return nil
}

// Compile-time checks that both stores satisfy IdempotencyStore (including the
// new Reserve method).
var (
	_ IdempotencyStore = (*FileIdempotencyStore)(nil)
	_ IdempotencyStore = (*MemoryIdempotencyStore)(nil)
)
