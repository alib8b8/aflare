// Copyright (c) 2026 aflare Contributors
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

// This file implements LLM router quota persistence and multi-tenant quota
// isolation for financial SaaS scenarios. It addresses two gaps in the
// in-memory-only quota system:
//
//  1. Persistence: DailyUsage is persisted to a QuotaStore so a process
//     restart resumes from the correct count instead of zero (which would
//     allow over-spending the remaining daily quota).
//  2. Multi-tenancy: each router instance carries a tenantID, and the
//     QuotaStore is keyed by (tenant, provider) so different tenants draw
//     from independent quota pools. A per-tenant quota override table lets
//     tenant A have a different QuotaDaily than the global default.
//
// Design notes:
//   - Backward compatible: when no QuotaStore is configured, the router is
//     memory-only, exactly as before. When tenantID is empty, "default" is
//     used, matching prior single-tenant behavior.
//   - Writes are debounced (1s default) via a goroutine so high-frequency
//     DailyUsage increments don't translate to high-frequency disk I/O.
//   - Save failures only log a warning; they never block routing decisions.
//   - FileQuotaStore uses tmp + atomic rename (mirroring wal.go / idempotency.go).

package nodes

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
)

// defaultQuotaTenant is the tenant ID used when none is configured. It keeps
// the on-disk layout stable for single-tenant deployments and matches the
// pre-multi-tenant behavior.
const defaultQuotaTenant = "default"

// defaultQuotaDebounce is the coalescing window for quota writes. Frequent
// DailyUsage increments (one per successful LLM call) are aggregated so the
// store sees at most one Save per provider per window.
const defaultQuotaDebounce = 1 * time.Second

// QuotaStore persists per-(tenant, provider) daily quota usage so the router
// recovers correct DailyUsage after a process restart. All methods must be
// safe for concurrent use.
//
// Contract:
//   - Load reports a missing entry as (0, "", nil) — NOT an error — so
//     callers can treat "first run" and "exists with zero usage" uniformly.
//   - Save should be atomic (tmp + rename) so a crash mid-write cannot
//     leave a corrupt record.
//   - Clear on a missing entry is a no-op (returns nil).
type QuotaStore interface {
	Load(tenant, provider string) (usage int64, day string, err error)
	Save(tenant, provider string, usage int64, day string) error
	Clear(tenant, provider string) error
}

// quotaRecord is the on-disk JSON representation of a persisted quota entry.
type quotaRecord struct {
	Tenant    string    `json:"tenant"`
	Provider  string    `json:"provider"`
	Usage     int64     `json:"usage"`
	Day       string    `json:"day"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FileQuotaStore is the default QuotaStore: each (tenant, provider) pair is
// persisted as a single JSON file at <base>/<tenant>/<provider>.json, written
// atomically via tmp + rename. The base directory defaults to
// <os.UserConfigDir>/aflare/quota.
type FileQuotaStore struct {
	mu   sync.Mutex
	base string
}

// DefaultQuotaDir returns the default on-disk directory for the file quota
// store: <os.UserConfigDir>/aflare/quota. Falls back to os.TempDir() when
// the user config directory cannot be determined.
func DefaultQuotaDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "aflare", "quota")
}

// NewFileQuotaStore creates a FileQuotaStore rooted at base. When base is
// empty, DefaultQuotaDir() is used. The directory is created lazily on the
// first write, so constructing the store is always cheap and never fails.
func NewFileQuotaStore(base string) *FileQuotaStore {
	if base == "" {
		base = DefaultQuotaDir()
	}
	return &FileQuotaStore{base: base}
}

// quotaHashLen is the number of hex characters retained from the sha256 of a
// tenant/provider when forming its on-disk path segment. 16 hex chars (8 bytes
// of entropy) keeps directories short and human-scannable while making
// path-traversal via crafted names infeasible — the segment is a fixed-width
// hex string that can never contain "..", "/", or any other path
// metacharacter. Mirrors the FileIdempotencyStore.pathFor pattern in
// internal/workflow/idempotency.go.
const quotaHashLen = 16

// quotaHashSegment returns a fixed-width hex path segment derived from s.
// Hashing (rather than using s verbatim) guarantees the segment contains only
// [0-9a-f] characters, so no tenant or provider value can ever escape the
// base directory via ".." or separators.
func quotaHashSegment(s string) string {
	sum := sha256.Sum256([]byte(s))
	enc := hex.EncodeToString(sum[:])
	if len(enc) > quotaHashLen {
		enc = enc[:quotaHashLen]
	}
	return enc
}

func (s *FileQuotaStore) pathFor(tenant, provider string) string {
	return filepath.Join(s.base, quotaHashSegment(tenant), quotaHashSegment(provider)+".json")
}

// safePathFor returns the on-disk path for (tenant, provider) after verifying
// the resolved path stays within base. The hash-based pathFor makes traversal
// impossible by construction; this containment check is defense-in-depth
// against future regressions that might reintroduce user-controlled path
// segments. Returns an error if the resolved path escapes base.
func (s *FileQuotaStore) safePathFor(tenant, provider string) (string, error) {
	cleaned := filepath.Clean(s.pathFor(tenant, provider))
	rel, err := filepath.Rel(s.base, cleaned)
	if err != nil {
		return "", fmt.Errorf("quota: resolve path for %s/%s: %w", tenant, provider, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("quota: path for %s/%s escapes base directory", tenant, provider)
	}
	return cleaned, nil
}

// validateNoSymlink is the H-2 defense against symlink attacks on the quota
// store. Even though pathFor hashes every path segment (so ".." / "/" can
// never appear in a segment), an attacker who can write under a hashed
// directory could plant a symlink — e.g. base/<tenantHash>/<provider>.json ->
// /etc/cron.d/backdoor — and let the next Save's rename overwrite the symlink
// target. validateNoSymlink resolves all symlinks in path and confirms the
// real location still lives under base, blocking such escapes.
//
// A non-existent path is allowed (returns nil): the common case for a fresh
// Save, and for Load/Clear on a never-written entry. When the path (or a
// parent directory) does exist and resolves outside base via a symlink, an
// error is returned so the caller refuses the operation.
func validateNoSymlink(path, base string) error {
	// Resolve base too, so a base that itself contains a symlink (e.g. a
	// tmp dir under a symlinked parent) does not cause false positives.
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		realBase = base
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // path (or a component) does not exist yet — OK
		}
		return err
	}
	if real == "" {
		return nil
	}
	rel, err := filepath.Rel(realBase, real)
	if err != nil {
		return fmt.Errorf("quota: path escapes base via symlink")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("quota: path escapes base via symlink")
	}
	return nil
}

// Load reads the persisted (usage, day) for the (tenant, provider) pair.
// A missing file OR a 0-byte file (the post-crash residue of an interrupted
// write) is reported as (0, "", nil) so the router treats a half-written
// record as a fresh start rather than a parse failure. A non-empty but
// unparseable file is genuine corruption and is surfaced as an error.
func (s *FileQuotaStore) Load(tenant, provider string) (int64, string, error) {
	if tenant == "" || provider == "" {
		return 0, "", fmt.Errorf("quota: empty tenant or provider")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.safePathFor(tenant, provider)
	if err != nil {
		return 0, "", err
	}
	if err := validateNoSymlink(path, s.base); err != nil {
		return 0, "", fmt.Errorf("quota: load %s/%s: %w", tenant, provider, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, "", nil
		}
		return 0, "", fmt.Errorf("quota: load %s/%s: %w", tenant, provider, err)
	}
	if len(data) == 0 {
		return 0, "", nil
	}
	var rec quotaRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return 0, "", fmt.Errorf("quota: parse %s/%s: %w", tenant, provider, err)
	}
	return rec.Usage, rec.Day, nil
}

// Save persists (usage, day) atomically: the record is marshaled to JSON,
// written to a tmp file, fsynced, then renamed over the final path, and the
// parent directory is fsynced so the rename itself is durable. A failure
// between the write and the rename leaves only the tmp file behind (which is
// never read), so the previous record — if any — stays intact. The fsync of
// the data file and the directory closes the window where a crash could leave
// a 0-byte final file that would otherwise break the next Load.
func (s *FileQuotaStore) Save(tenant, provider string, usage int64, day string) error {
	if tenant == "" || provider == "" {
		return fmt.Errorf("quota: empty tenant or provider")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	finalPath, err := s.safePathFor(tenant, provider)
	if err != nil {
		return err
	}
	// H-2: refuse to operate through a symlink that escapes base. The
	// hash-based pathFor blocks ".." traversal, but an attacker who can
	// write under a hashed directory could plant a symlink; this check
	// (plus the parent-dir check after MkdirAll below) blocks that.
	if err := validateNoSymlink(finalPath, s.base); err != nil {
		return fmt.Errorf("quota: save %s/%s: %w", tenant, provider, err)
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return fmt.Errorf("quota: mkdir %s/%s: %w", tenant, provider, err)
	}
	// After MkdirAll, re-verify the parent directory was not redirected
	// outside base via a planted symlink (e.g. base/<tenantHash> ->
	// /tmp/outside). MkdirAll follows symlinks, so the parent must be
	// checked again once it exists.
	if err := validateNoSymlink(filepath.Dir(finalPath), s.base); err != nil {
		return fmt.Errorf("quota: save %s/%s: %w", tenant, provider, err)
	}
	rec := quotaRecord{
		Tenant:    tenant,
		Provider:  provider,
		Usage:     usage,
		Day:       day,
		UpdatedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("quota: marshal %s/%s: %w", tenant, provider, err)
	}
	// Refuse to rename over a symlink at the final path. An attacker who
	// planted base/<tenantHash>/<provider>.json -> /etc/cron.d/backdoor
	// would otherwise have the rename follow the symlink and overwrite the
	// target. Lstat (not Stat) does not follow the final component, so a
	// symlink at finalPath is detected even when its target is missing.
	if fi, err := os.Lstat(finalPath); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("quota: refusing to write through symlink at %s", finalPath)
	}
	tmpPath := finalPath + ".tmp." + quotaTmpSuffix()
	// O_EXCL: the tmp name is random, so a collision means tampering —
	// fail rather than overwrite. quotaOpenNoFollow (O_NOFOLLOW on Unix, 0
	// on Windows): refuse if the tmp path itself is a symlink
	// (defense-in-depth; the random suffix makes this near-impossible but
	// the flag is free on Unix).
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL|quotaOpenNoFollow, 0o600)
	if err != nil {
		return fmt.Errorf("quota: write tmp %s/%s: %w", tenant, provider, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("quota: write tmp %s/%s: %w", tenant, provider, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("quota: fsync tmp %s/%s: %w", tenant, provider, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("quota: close tmp %s/%s: %w", tenant, provider, err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("quota: rename %s/%s: %w", tenant, provider, err)
	}
	// Best-effort fsync of the parent directory so the rename is durable. A
	// failure here is non-fatal: at worst a crash loses the rename, leaving
	// the tmp file (never read) and the previous record intact — the safe
	// direction.
	if dir, err := os.Open(filepath.Dir(finalPath)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

// Clear removes the persisted entry. A missing file is not an error.
func (s *FileQuotaStore) Clear(tenant, provider string) error {
	if tenant == "" || provider == "" {
		return fmt.Errorf("quota: empty tenant or provider")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.safePathFor(tenant, provider)
	if err != nil {
		return err
	}
	if err := validateNoSymlink(path, s.base); err != nil {
		return fmt.Errorf("quota: clear %s/%s: %w", tenant, provider, err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("quota: clear %s/%s: %w", tenant, provider, err)
	}
	return nil
}

// MemoryQuotaStore is an in-memory QuotaStore intended for tests. It is
// concurrency-safe and counts Save calls so debounce behavior can be
// asserted without timing-sensitive file I/O.
type MemoryQuotaStore struct {
	mu      sync.Mutex
	data    map[string]quotaRecord
	saveCnt int
	saveErr error // when non-nil, Save returns this error
}

// NewMemoryQuotaStore returns an empty in-memory quota store.
func NewMemoryQuotaStore() *MemoryQuotaStore {
	return &MemoryQuotaStore{data: make(map[string]quotaRecord)}
}

func (s *MemoryQuotaStore) key(tenant, provider string) string {
	return tenant + "\x00" + provider
}

// Load returns the persisted (usage, day) for the (tenant, provider) pair.
// A missing entry is reported as (0, "", nil).
func (s *MemoryQuotaStore) Load(tenant, provider string) (int64, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.data[s.key(tenant, provider)]
	if !ok {
		return 0, "", nil
	}
	return rec.Usage, rec.Day, nil
}

// Save stores (usage, day) and increments the Save call counter. When
// SetSaveError has been called with a non-nil error, Save returns that error
// without mutating the store, simulating a failing backing store.
func (s *MemoryQuotaStore) Save(tenant, provider string, usage int64, day string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveCnt++
	if s.saveErr != nil {
		return s.saveErr
	}
	s.data[s.key(tenant, provider)] = quotaRecord{
		Tenant:    tenant,
		Provider:  provider,
		Usage:     usage,
		Day:       day,
		UpdatedAt: time.Now().UTC(),
	}
	return nil
}

// Clear removes the entry for the (tenant, provider) pair.
func (s *MemoryQuotaStore) Clear(tenant, provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, s.key(tenant, provider))
	return nil
}

// SaveCount returns the number of Save calls received. Used by debounce
// tests to assert that many updates coalesced into one write.
func (s *MemoryQuotaStore) SaveCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveCnt
}

// SetSaveError injects an error to be returned by subsequent Save calls.
// Pass nil to clear. Used to verify routing decisions survive store failures.
func (s *MemoryQuotaStore) SetSaveError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveErr = err
}

// quotaSaver coalesces frequent Save calls into at most one store write per
// interval per (tenant, provider) key. It runs a single goroutine that ticks
// every interval and flushes whatever has accumulated in the pending map.
//
// Rationale: a successful LLM call increments DailyUsage once, and a busy
// router may see dozens of calls per second. Writing to disk on each
// increment would saturate the filesystem; the debounce window trades at
// most `interval` of potential over-spending on crash for a ~Nx reduction in
// write traffic.
type quotaSaver struct {
	store    QuotaStore
	interval time.Duration

	mu      sync.Mutex
	pending map[string]quotaUpdate

	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
	// closed is set true by close() before the done channel is closed, so
	// a Save racing with shutdown can detect the saver is no longer
	// accepting writes and return false instead of silently enqueuing an
	// update the already-exited loop goroutine would never flush. Without
	// this guard, a late Save would mutate the pending map and the
	// persisted usage would silently diverge from in-memory usage — a
	// correctness hazard in financial quota scenarios.
	closed atomic.Bool
}

type quotaUpdate struct {
	tenant   string
	provider string
	usage    int64
	day      string
	// clear, when true, makes flush call store.Clear instead of store.Save.
	// It is set by scheduleQuotaClearLocked on cross-day reset so the
	// persisted entry is removed without holding statsMu during file IO
	// (M-2). Because EnqueueClear overwrites any pending Save for the same
	// (tenant, provider) key, a stale Save enqueued before the day reset
	// can no longer be flushed back to the store after the reset — closing
	// the M-6 race without needing a separate clearPending step.
	clear bool
}

// newQuotaSaver starts a debounced writer that flushes to store every
// interval. The returned saver must be closed (via close) to stop the
// goroutine and flush pending writes.
func newQuotaSaver(store QuotaStore, interval time.Duration) *quotaSaver {
	if interval <= 0 {
		interval = defaultQuotaDebounce
	}
	q := &quotaSaver{
		store:    store,
		interval: interval,
		pending:  make(map[string]quotaUpdate),
		done:     make(chan struct{}),
	}
	q.wg.Add(1)
	go q.loop()
	return q
}

// Save queues an update. Multiple updates for the same (tenant, provider)
// within one interval overwrite each other in the pending map, so only the
// latest usage survives to the store. This method never blocks: the pending
// map grows unboundedly only between ticks, and a tick flushes it entirely.
//
// Returns true when the update was enqueued, and false once close() has been
// called — a late Save racing with shutdown must not silently mutate the
// pending map after the loop goroutine has exited, otherwise the write would
// be lost without any signal to the caller (M-3). Callers that treat quota
// persistence as best-effort (the router does) may ignore the return value.
func (q *quotaSaver) Save(tenant, provider string, usage int64, day string) bool {
	if q.closed.Load() {
		return false
	}
	key := tenant + "\x00" + provider
	q.mu.Lock()
	q.pending[key] = quotaUpdate{tenant: tenant, provider: provider, usage: usage, day: day}
	q.mu.Unlock()
	return true
}

// EnqueueClear queues a Clear for (tenant, provider). It overwrites any
// pending Save for the same key, which closes the M-6 race: a stale Save
// enqueued before a cross-day reset can no longer be flushed back to the
// store after the reset, because the clear replaces it in the pending map.
// Like Save, it returns false once close() has been called.
//
// M-2: EnqueueClear exists so the router's cross-day reset path does NOT
// perform synchronous store.Clear (a file IO) under statsMu. The clear is
// debounced and run by the saver goroutine, off the routing critical
// section. The in-memory DailyUsage is already zeroed by the caller, so
// routing decisions are correct immediately; the on-disk cleanup is
// best-effort and reconciled on the next loadQuotaLocked anyway.
func (q *quotaSaver) EnqueueClear(tenant, provider string) bool {
	if q.closed.Load() {
		return false
	}
	key := tenant + "\x00" + provider
	q.mu.Lock()
	q.pending[key] = quotaUpdate{tenant: tenant, provider: provider, clear: true}
	q.mu.Unlock()
	return true
}

// clearPending drops any queued update (Save or Clear) for (tenant, provider).
// It is a defensive belt-and-suspenders for the M-6 race: EnqueueClear
// already overwrites a stale Save, so this is not strictly required for
// correctness, but it lets a caller explicitly void a pending entry — for
// example, between detecting a day rollover and enqueueing the fresh Save —
// so a flush tick landing in that window cannot observe either the stale
// Save or the just-queued Clear. Returns true if an entry was removed.
func (q *quotaSaver) clearPending(tenant, provider string) bool {
	key := tenant + "\x00" + provider
	q.mu.Lock()
	_, ok := q.pending[key]
	if ok {
		delete(q.pending, key)
	}
	q.mu.Unlock()
	return ok
}

func (q *quotaSaver) loop() {
	defer q.wg.Done()
	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()
	for {
		select {
		case <-q.done:
			q.flush()
			return
		case <-ticker.C:
			q.flush()
		}
	}
}

// flush writes all pending updates to the store and clears the pending map.
// Save/Clear failures are logged but do not stop the flush — each update is
// independent, and a failing one should not block the others.
//
// M-2: clear updates (u.clear == true) call store.Clear instead of store.Save,
// so the cross-day-reset file removal happens here, on the saver goroutine,
// not under the router's statsMu.
func (q *quotaSaver) flush() {
	q.mu.Lock()
	if len(q.pending) == 0 {
		q.mu.Unlock()
		return
	}
	pending := q.pending
	q.pending = make(map[string]quotaUpdate)
	q.mu.Unlock()

	for _, u := range pending {
		if u.clear {
			if err := q.store.Clear(u.tenant, u.provider); err != nil {
				logger.Warn("quota store clear failed (debounced)",
					"tenant", u.tenant,
					"provider", u.provider,
					"error", err,
				)
			}
			continue
		}
		if err := q.store.Save(u.tenant, u.provider, u.usage, u.day); err != nil {
			logger.Warn("quota store save failed (debounced)",
				"tenant", u.tenant,
				"provider", u.provider,
				"error", err,
			)
		}
	}
}

// close stops the goroutine and flushes any pending writes. Safe to call
// multiple times; subsequent calls are no-ops. closed is set BEFORE closing
// the done channel so a Save racing with shutdown observes closed==true and
// returns false instead of enqueuing an update the exiting loop will never
// flush (M-3).
func (q *quotaSaver) close() {
	q.closeOnce.Do(func() {
		q.closed.Store(true)
		close(q.done)
		q.wg.Wait()
	})
}

// quotaTmpSuffix returns 8 hex chars from crypto/rand for unique tmp-file
// names. On the (extremely unlikely) read failure it falls back to a
// timestamp so the tmp path is still unique within the process.
func quotaTmpSuffix() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// RouterOption configures an LLMRouter at construction time. Options are
// applied in order; later options win when they touch the same field.
type RouterOption func(*LLMRouter)

// WithTenant sets the tenant ID for quota tracking. An empty string is
// normalized to "default" at lookup time, so callers can pass "" safely.
// Used to isolate per-tenant quota pools in the QuotaStore.
//
// SECURITY (M-1): tenantID MUST be set server-side from an authenticated
// context and NEVER derived from client-supplied input (e.g. an HTTP
// request header, query parameter, or JWT claim that the caller controls).
// The router does not — and cannot, without an auth dependency — verify
// the provenance of tenantID; a forged tenant="tenantB" would let an
// attacker draw from another tenant's quota pool. In a multi-tenant SaaS
// deployment, the recommended pattern is to resolve the tenant once at
// the request entrypoint from the authenticated principal and propagate
// it via context, then construct the router per request (or per tenant)
// using a thin wrapper, e.g.:
//
//	func WithTenantFromContext(ctx context.Context) RouterOption {
//	    // tenant must be injected by the auth middleware, not read from
//	    // the request body / headers a client can forge.
//	    tenant, _ := ctx.Value(authTenantKey{}).(string)
//	    return WithTenant(tenant)
//	}
//
// Without an auth layer, tenantID is caller-trusted and the multi-tenant
// quota isolation is advisory, not enforced.
func WithTenant(id string) RouterOption {
	return func(r *LLMRouter) {
		r.tenantID = id
	}
}

// WithQuotaStore injects a QuotaStore for persisting daily usage. When set,
// the router loads persisted usage on startup (so a restart resumes from the
// correct DailyUsage) and debounces writes on each usage change. When nil
// (the default), the router is memory-only — identical to pre-quota-store
// behavior.
func WithQuotaStore(s QuotaStore) RouterOption {
	return func(r *LLMRouter) {
		r.quotaStore = s
	}
}

// WithTenantQuota overrides RouterProvider.QuotaDaily per provider for this
// router's tenant. A provider absent from the map falls back to its
// configured QuotaDaily. This is how a tenant gets a different daily cap
// than the global default.
func WithTenantQuota(quota map[string]int64) RouterOption {
	return func(r *LLMRouter) {
		r.tenantQuota = quota
	}
}

// WithQuotaDebounce overrides the quota write debounce interval. The default
// is 1s; tests typically pass a shorter value (e.g. 5ms) so debounce
// behavior can be asserted without slowing the suite.
func WithQuotaDebounce(d time.Duration) RouterOption {
	return func(r *LLMRouter) {
		r.quotaDebounce = d
	}
}

// WithTimeZone configures the timezone used to compute the "today" date
// string that drives daily quota reset. The default is UTC, so daily quotas
// reset at a consistent instant across deployments regardless of host TZ —
// important for financial cross-timezone deployments where a local-midnight
// reset would let one region overspend. Pass a business timezone (e.g.
// time.LoadLocation("Asia/Shanghai")) when quotas must reset at local
// midnight in a specific trading center. A nil location reverts to UTC.
func WithTimeZone(tz *time.Location) RouterOption {
	return func(r *LLMRouter) {
		r.timeZone = tz
	}
}

// NewLLMRouter builds an LLMRouter from the loaded config with the given
// options applied. This is the entry point for callers that need quota
// persistence or multi-tenant isolation. GetGlobalRouter() returns a
// memory-only default router for backward compatibility and should be
// preferred when persistence is not required.
//
// Options are applied after the config-derived providers are populated, so
// WithTenantQuota / WithQuotaStore see the final provider list. The
// persisted quota is loaded eagerly for every enabled provider, so the first
// routing decision observes the correct DailyUsage.
//
// L-6: when WithQuotaStore is supplied, NewLLMRouter starts a long-lived
// quotaSaver goroutine (via initQuotaPersistence) that debounces disk
// writes. The caller MUST call r.CloseQuota() when the router is no longer
// needed (e.g. at process shutdown or when a per-request router goes out of
// scope) to stop that goroutine and flush pending writes; otherwise it
// leaks for the lifetime of the process. CloseQuota is idempotent and a
// no-op when no store was configured, so calling it unconditionally on
// shutdown is safe. The process-wide singleton returned by GetGlobalRouter
// is intentionally exempt: its saver lives for the lifetime of the process
// and is reaped by OS shutdown, so leaking it is harmless.
func NewLLMRouter(opts ...RouterOption) *LLMRouter {
	r := NewLLMRouterFromConfig()
	for _, opt := range opts {
		opt(r)
	}
	r.initQuotaPersistence()
	return r
}

// initQuotaPersistence wires up the debounced writer and eagerly loads
// persisted quota for every enabled provider. It is a no-op when no
// QuotaStore is configured, so manually-constructed routers (which leave
// quotaStore nil) behave exactly as before.
//
// Must be called at most once, after providers and quotaStore are set.
func (r *LLMRouter) initQuotaPersistence() {
	if r.quotaStore == nil {
		return
	}
	debounce := r.quotaDebounce
	if debounce <= 0 {
		debounce = defaultQuotaDebounce
	}
	r.quotaSaver = newQuotaSaver(r.quotaStore, debounce)

	// Eagerly load persisted quota for each enabled provider so the first
	// routing decision sees the correct DailyUsage instead of zero.
	r.statsMu.Lock()
	defer r.statsMu.Unlock()
	for _, p := range r.providers {
		if !p.Enabled {
			continue
		}
		if _, exists := r.stats[p.Name]; exists {
			continue // already populated (e.g. by a prior init call)
		}
		stats := newProviderStats(r.todayUTC())
		r.loadQuotaLocked(p.Name, stats)
		r.stats[p.Name] = stats
	}
}

// effectiveTenant returns the tenant ID used for quota lookups, normalizing
// the empty string to "default".
func (r *LLMRouter) effectiveTenant() string {
	if r.tenantID == "" {
		return defaultQuotaTenant
	}
	return r.tenantID
}

// quotaFor returns the effective daily quota for a provider under this
// router's tenant: the per-tenant override when present, otherwise the
// provider's configured QuotaDaily.
func (r *LLMRouter) quotaFor(p RouterProvider) int64 {
	if r.tenantQuota != nil {
		if q, ok := r.tenantQuota[p.Name]; ok {
			return q
		}
	}
	return p.QuotaDaily
}

// loadQuotaLocked populates stats.DailyUsage / stats.LastResetDate from the
// quota store. Called under r.statsMu. A missing entry leaves stats at zero
// (the newProviderStats default). A stale (previous-day) entry is cleared so
// the new day starts clean on disk too. Load errors are logged but do not
// mutate stats — the router falls back to zero usage, which is the safe
// direction (under-counting risks over-spend, but a load failure means the
// store is unavailable and there is no better estimate).
func (r *LLMRouter) loadQuotaLocked(name string, stats *ProviderStats) {
	if r.quotaStore == nil {
		return
	}
	tenant := r.effectiveTenant()
	usage, day, err := r.quotaStore.Load(tenant, name)
	if err != nil {
		logger.Warn("quota store load failed",
			"tenant", tenant, "provider", name, "error", err)
		return
	}
	today := r.todayUTC()
	if day == today {
		stats.DailyUsage = usage
		stats.LastResetDate = day
	} else if day != "" {
		// Stale (previous day): clear the persisted entry so the new
		// day starts at zero on disk as well as in memory.
		if err := r.quotaStore.Clear(tenant, name); err != nil {
			logger.Warn("quota store clear (stale) failed",
				"tenant", tenant, "provider", name, "error", err)
		}
	}
}

// scheduleQuotaClearLocked queues an async Clear of the persisted entry for
// (tenant, provider). Called under r.statsMu on cross-day reset in the hot
// routing path (recordSuccess / getActiveProviders) so the new day starts
// clean on disk WITHOUT holding statsMu across the file IO (M-2).
//
// M-6: EnqueueClear overwrites any pending Save for the same (tenant,
// provider) key. This closes the race where a Save enqueued before the day
// reset (carrying yesterday's usage/day) would otherwise be flushed back to
// the store after the reset, restoring stale data. We additionally call
// clearPending first so a flush tick landing between the reset detection
// and this enqueue observes an empty pending entry rather than the stale
// Save. The fresh Save with today's usage is enqueued immediately after by
// the caller via scheduleQuotaSaveLocked.
//
// When no saver is configured (memory-only router, or a manually-constructed
// router with quotaStore set but initQuotaPersistence not called), this
// falls back to a synchronous Clear so the on-disk state is still cleaned
// up — at the cost of holding statsMu during the IO, matching the
// pre-M-2 behaviour for that edge case.
func (r *LLMRouter) scheduleQuotaClearLocked(name string) {
	if r.quotaStore == nil {
		return
	}
	tenant := r.effectiveTenant()
	if r.quotaSaver == nil {
		// Fallback for the rare manually-constructed case: synchronous clear.
		if err := r.quotaStore.Clear(tenant, name); err != nil {
			logger.Warn("quota store clear failed",
				"tenant", tenant, "provider", name, "error", err)
		}
		return
	}
	// M-6: drop any stale pending Save for this key BEFORE enqueuing the
	// clear, so a concurrent flush cannot observe the stale Save in the
	// window between detecting the day rollover and replacing the entry.
	r.quotaSaver.clearPending(tenant, name)
	// M-2: queue the clear; the actual store.Clear runs on the saver
	// goroutine, off the statsMu critical section. Best-effort: returns
	// false once the saver has been closed (after CloseQuota), which we
	// ignore — quota persistence is advisory.
	_ = r.quotaSaver.EnqueueClear(tenant, name)
}

// scheduleQuotaSaveLocked queues a debounced write of the current
// (DailyUsage, LastResetDate). Called under r.statsMu after DailyUsage
// changes. The saver coalesces rapid updates into one store write per
// debounce window, so this is cheap to call on every successful request.
func (r *LLMRouter) scheduleQuotaSaveLocked(name string, stats *ProviderStats) {
	if r.quotaSaver == nil {
		return
	}
	// Best-effort: Save returns false once the saver has been closed (e.g.
	// after CloseQuota). Quota persistence is advisory and must never block
	// a routing decision, so the return value is intentionally ignored.
	_ = r.quotaSaver.Save(r.effectiveTenant(), name, stats.DailyUsage, stats.LastResetDate)
}

// FlushQuota forces any pending debounced quota writes to flush immediately.
// Mainly useful in tests; production code rarely needs to call this since the
// debounce window is short and Close flushes on shutdown.
func (r *LLMRouter) FlushQuota() {
	if r.quotaSaver == nil {
		return
	}
	r.quotaSaver.flush()
}

// CloseQuota shuts down the quota persistence goroutine and flushes pending
// writes. After CloseQuota, the router still routes but no longer persists
// new usage. Safe to call multiple times and safe to call when no store is
// configured (no-op).
func (r *LLMRouter) CloseQuota() {
	if r.quotaSaver != nil {
		r.quotaSaver.close()
	}
}
