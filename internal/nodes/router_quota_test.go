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

package nodes

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alib8b8/llm-box/internal/config"
)

// newQuotaTestRouter builds an LLMRouter wired to the given store and tenant
// without going through NewLLMRouterFromConfig (which reads env/config files).
// It mirrors the manual construction pattern used by the existing router
// tests so quota behavior can be exercised in isolation.
func newQuotaTestRouter(providers []RouterProvider, store QuotaStore, tenant string) *LLMRouter {
	r := &LLMRouter{
		providers:  append([]RouterProvider(nil), providers...),
		stats:      make(map[string]*ProviderStats),
		strategy:   config.RouterStrategyPriority,
		maxRetry:   3,
		tenantID:   tenant,
		quotaStore: store,
	}
	r.initQuotaPersistence()
	return r
}

// newQuotaTestRouterWithDebounce is like newQuotaTestRouter but lets the test
// pick a shorter debounce interval so the debounce test doesn't sleep for a
// full second.
func newQuotaTestRouterWithDebounce(providers []RouterProvider, store QuotaStore, tenant string, debounce time.Duration) *LLMRouter {
	r := &LLMRouter{
		providers:     append([]RouterProvider(nil), providers...),
		stats:         make(map[string]*ProviderStats),
		strategy:      config.RouterStrategyPriority,
		maxRetry:      3,
		tenantID:      tenant,
		quotaStore:    store,
		quotaDebounce: debounce,
	}
	r.initQuotaPersistence()
	return r
}

// todayStr returns the current date in the same format the router uses for
// LastResetDate, so tests can build records that the day-reset logic treats
// as "current". Uses UTC to match the router's default timezone (todayUTC).
func todayStr() string { return time.Now().UTC().Format("2006-01-02") }

// yesterdayStr returns yesterday's date string, used to simulate a stale
// (previous-day) persisted record that the router must clear on load.
// Uses UTC to match the router's default timezone.
func yesterdayStr() string {
	return time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
}

// TestQuota_PersistAcrossInstances verifies that DailyUsage survives a
// process restart: instance A records 50 tokens of usage, then a fresh
// instance B built on the same QuotaStore sees DailyUsage=50 (not 0) for
// that provider on startup. Without persistence, B would start at 0 and
// could overspend the remaining daily quota — the core gap this feature
// closes for the financial SaaS scenario.
func TestQuota_PersistAcrossInstances(t *testing.T) {
	store := NewMemoryQuotaStore()
	providers := []RouterProvider{
		{Name: "openai", Enabled: true, QuotaDaily: 1000, APIKey: "k", Priority: 1},
	}

	routerA := newQuotaTestRouter(providers, store, "default")
	defer routerA.CloseQuota()

	// Simulate a successful call consuming 50 tokens.
	routerA.recordSuccess("openai", 100, 50)
	routerA.FlushQuota()

	// usage in the store is now 50 for (default, openai).
	if usage, _, _ := store.Load("default", "openai"); usage != 50 {
		t.Fatalf("store usage after recordSuccess = %d, want 50", usage)
	}

	// Spin up a second router instance sharing the same store. Its
	// initQuotaPersistence must load the 50-token usage rather than
	// starting at 0.
	routerB := newQuotaTestRouter(providers, store, "default")
	defer routerB.CloseQuota()

	stats := routerB.GetProviderStats()
	s, ok := stats["openai"]
	if !ok {
		t.Fatal("routerB has no stats for openai after init")
	}
	if s.DailyUsage != 50 {
		t.Errorf("routerB DailyUsage = %d, want 50 (persisted across instances)", s.DailyUsage)
	}
}

// TestQuota_DailyReset verifies that a persisted record from yesterday is
// treated as stale: on startup the router must zero DailyUsage AND clear the
// on-disk entry so the new day starts fresh. Without the clear, the stale
// record would be re-loaded on the NEXT restart and re-zeroed — wasteful and
// confusing, but more importantly the day boundary must be authoritative.
func TestQuota_DailyReset(t *testing.T) {
	store := NewMemoryQuotaStore()
	// Seed a record from yesterday with non-zero usage.
	yesterday := yesterdayStr()
	if err := store.Save("default", "openai", 800, yesterday); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	providers := []RouterProvider{
		{Name: "openai", Enabled: true, QuotaDaily: 1000, APIKey: "k", Priority: 1},
	}
	router := newQuotaTestRouter(providers, store, "default")
	defer router.CloseQuota()

	// After init, the stale record must NOT have been loaded — DailyUsage
	// should be 0 because yesterday != today.
	stats := router.GetProviderStats()
	if s, ok := stats["openai"]; ok {
		if s.DailyUsage != 0 {
			t.Errorf("DailyUsage after stale load = %d, want 0 (day reset)", s.DailyUsage)
		}
		if s.LastResetDate != todayStr() {
			t.Errorf("LastResetDate = %q, want %q (today)", s.LastResetDate, todayStr())
		}
	}

	// The stale on-disk entry must have been cleared.
	if usage, day, _ := store.Load("default", "openai"); usage != 0 || day != "" {
		t.Errorf("stale store entry not cleared: usage=%d day=%q", usage, day)
	}
}

// TestQuota_TenantIsolation verifies that two tenants draw from independent
// quota pools: tenant A consuming 30 tokens must not affect tenant B's
// DailyUsage (which stays 0). This is the core multi-tenancy property for
// financial SaaS — one client's spend cannot count against another's quota.
func TestQuota_TenantIsolation(t *testing.T) {
	store := NewMemoryQuotaStore()
	providers := []RouterProvider{
		{Name: "openai", Enabled: true, QuotaDaily: 1000, APIKey: "k", Priority: 1},
	}

	routerA := newQuotaTestRouter(providers, store, "tenantA")
	defer routerA.CloseQuota()
	routerB := newQuotaTestRouter(providers, store, "tenantB")
	defer routerB.CloseQuota()

	// Tenant A consumes 30 tokens.
	routerA.recordSuccess("openai", 50, 30)
	routerA.FlushQuota()

	// Tenant A sees 30.
	if usage, _, _ := store.Load("tenantA", "openai"); usage != 30 {
		t.Errorf("tenantA usage = %d, want 30", usage)
	}
	// Tenant B sees 0 — independent pool.
	if usage, _, _ := store.Load("tenantB", "openai"); usage != 0 {
		t.Errorf("tenantB usage = %d, want 0 (tenant isolation)", usage)
	}

	// Tenant B's in-memory stats also reflect 0.
	statsB := routerB.GetProviderStats()
	if s, ok := statsB["openai"]; ok && s.DailyUsage != 0 {
		t.Errorf("tenantB in-memory DailyUsage = %d, want 0", s.DailyUsage)
	}
}

// TestQuota_PerTenantQuotaOverride verifies that a per-tenant quota override
// takes precedence over the global RouterProvider.QuotaDaily. Tenant A has
// QuotaDaily=100 (override) while the provider's global default is 1000;
// after 100 tokens, tenant A's provider must be excluded from the active
// list even though the global quota would still allow 900 more.
func TestQuota_PerTenantQuotaOverride(t *testing.T) {
	store := NewMemoryQuotaStore()
	providers := []RouterProvider{
		{Name: "openai", Enabled: true, QuotaDaily: 1000, APIKey: "k", Priority: 1},
	}

	// Override: tenantA gets only 100/day for openai.
	router := &LLMRouter{
		providers:     append([]RouterProvider(nil), providers...),
		stats:         make(map[string]*ProviderStats),
		strategy:      config.RouterStrategyPriority,
		maxRetry:      3,
		tenantID:      "tenantA",
		quotaStore:    store,
		tenantQuota:   map[string]int64{"openai": 100},
		quotaDebounce: 10 * time.Millisecond,
	}
	router.initQuotaPersistence()
	defer router.CloseQuota()

	// Verify the override is consulted (not the global 1000).
	if q := router.quotaFor(providers[0]); q != 100 {
		t.Fatalf("quotaFor = %d, want 100 (tenant override)", q)
	}

	// Consume exactly the override (100). The provider must then be
	// excluded from the active list.
	router.recordSuccess("openai", 10, 100)
	router.FlushQuota()

	active := router.getActiveProviders()
	for _, p := range active {
		if p.Name == "openai" {
			t.Error("openai should be excluded after reaching tenant quota 100, but was active")
		}
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active providers after quota exhaustion, got %d", len(active))
	}

	// The global quota (1000) would have allowed more; confirm the
	// override is what blocked it by checking the store sees usage=100.
	if usage, _, _ := store.Load("tenantA", "openai"); usage != 100 {
		t.Errorf("store usage = %d, want 100", usage)
	}
}

// TestQuota_SaveFailureDoesNotBlock verifies that a failing QuotaStore does
// not break routing decisions. When Save returns an error (simulating a
// read-only filesystem or a full disk), the router must still select
// providers and record usage in memory; the error is only logged.
func TestQuota_SaveFailureDoesNotBlock(t *testing.T) {
	store := NewMemoryQuotaStore()
	store.SetSaveError(errors.New("simulated read-only filesystem"))

	providers := []RouterProvider{
		{Name: "openai", Enabled: true, QuotaDaily: 1000, APIKey: "k", Priority: 1},
	}
	router := newQuotaTestRouterWithDebounce(providers, store, "default", 10*time.Millisecond)
	defer router.CloseQuota()

	// recordSuccess must not panic / block even though Save will fail.
	router.recordSuccess("openai", 10, 50)
	router.FlushQuota()

	// In-memory stats still reflect the usage (Save failure doesn't
	// roll back the in-memory counter).
	stats := router.GetProviderStats()
	if s, ok := stats["openai"]; !ok || s.DailyUsage != 50 {
		got := int64(-1)
		if ok {
			got = s.DailyUsage
		}
		t.Errorf("in-memory DailyUsage after failed Save = %d, want 50", got)
	}

	// The provider is still selectable (quota not exhausted, breaker
	// closed). This is the "routing still works" property.
	active := router.getActiveProviders()
	found := false
	for _, p := range active {
		if p.Name == "openai" {
			found = true
		}
	}
	if !found {
		t.Error("openai not in active providers after Save failure; routing must still work")
	}
}

// TestQuota_DebounceWrites verifies that many Save calls within one debounce
// window coalesce into a single store write. 100 rapid updates should
// produce exactly 1 Save call to the store (the last write wins, earlier
// ones are dropped from the pending map).
func TestQuota_DebounceWrites(t *testing.T) {
	store := NewMemoryQuotaStore()
	debounce := 15 * time.Millisecond

	saver := newQuotaSaver(store, debounce)
	defer saver.close()

	// Fire 100 updates in rapid succession. Without debouncing this
	// would be 100 disk writes; with debouncing it should be 1.
	for i := 0; i < 100; i++ {
		saver.Save("default", "openai", int64(i+1), todayStr())
	}

	// Wait for at least one flush window to elapse.
	time.Sleep(debounce * 3)

	// The store should have seen exactly 1 Save (the last value, 100).
	if cnt := store.SaveCount(); cnt != 1 {
		t.Errorf("store Save called %d times, want 1 (debounced)", cnt)
	}
	if usage, _, _ := store.Load("default", "openai"); usage != 100 {
		t.Errorf("store usage = %d, want 100 (last write wins)", usage)
	}
}

// TestQuota_DebounceFlushesOnClose verifies that closing the saver flushes
// any pending writes, so a clean shutdown does not lose the last debounced
// update.
func TestQuota_DebounceFlushesOnClose(t *testing.T) {
	store := NewMemoryQuotaStore()
	saver := newQuotaSaver(store, 10*time.Second) // long interval so close must flush

	saver.Save("default", "openai", 42, todayStr())
	saver.close() // must flush the pending 42

	if cnt := store.SaveCount(); cnt != 1 {
		t.Errorf("store Save called %d times after close, want 1 (flushed on close)", cnt)
	}
	if usage, _, _ := store.Load("default", "openai"); usage != 42 {
		t.Errorf("store usage = %d, want 42 (flushed on close)", usage)
	}
}

// TestQuota_FileStoreRoundTrip verifies the FileQuotaStore persists and
// reloads (usage, day) across instances, exercising the real tmp+rename
// atomic-write path. This complements the in-memory tests by confirming the
// on-disk format is round-trippable.
func TestQuota_FileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewFileQuotaStore(dir)

	today := todayStr()
	if err := store.Save("tenantA", "openai", 250, today); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// File should exist at <dir>/<hash(tenantA)>/<hash(openai)>.json. The
	// tenant/provider are hashed (not used verbatim) so on-disk paths are
	// traversal-safe; quotaHashSegment is the same helper the store uses.
	path := filepath.Join(dir, quotaHashSegment("tenantA"), quotaHashSegment("openai")+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}

	usage, day, err := store.Load("tenantA", "openai")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if usage != 250 || day != today {
		t.Errorf("Load = (%d, %q), want (250, %q)", usage, day, today)
	}

	// Clear removes the file.
	if err := store.Clear("tenantA", "openai"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file removed after Clear; stat err = %v", err)
	}

	// Load after clear returns (0, "", nil).
	usage, day, err = store.Load("tenantA", "openai")
	if err != nil || usage != 0 || day != "" {
		t.Errorf("Load after Clear = (%d, %q, %v), want (0, \"\", nil)", usage, day, err)
	}
}

// TestQuota_FileStoreTenantIsolation verifies the on-disk layout isolates
// tenants into separate subdirectories so one tenant's file cannot collide
// with or overwrite another's.
func TestQuota_FileStoreTenantIsolation(t *testing.T) {
	dir := t.TempDir()
	store := NewFileQuotaStore(dir)
	today := todayStr()

	if err := store.Save("tenantA", "openai", 100, today); err != nil {
		t.Fatalf("Save tenantA: %v", err)
	}
	if err := store.Save("tenantB", "openai", 200, today); err != nil {
		t.Fatalf("Save tenantB: %v", err)
	}

	// Two distinct files under separate (hashed) tenant subdirectories.
	pathA := filepath.Join(dir, quotaHashSegment("tenantA"), quotaHashSegment("openai")+".json")
	pathB := filepath.Join(dir, quotaHashSegment("tenantB"), quotaHashSegment("openai")+".json")
	if _, err := os.Stat(pathA); err != nil {
		t.Errorf("tenantA file missing: %v", err)
	}
	if _, err := os.Stat(pathB); err != nil {
		t.Errorf("tenantB file missing: %v", err)
	}
	if pathA == pathB {
		t.Errorf("tenantA and tenantB hashed to the same path; isolation broken: %s", pathA)
	}

	// Each tenant loads its own usage.
	if u, _, _ := store.Load("tenantA", "openai"); u != 100 {
		t.Errorf("tenantA usage = %d, want 100", u)
	}
	if u, _, _ := store.Load("tenantB", "openai"); u != 200 {
		t.Errorf("tenantB usage = %d, want 200", u)
	}
}

// TestQuota_ConcurrentSaver verifies the quotaSaver is safe under concurrent
// Save calls (run under -race). The goroutine ticker and the lock-protected
// pending map must not race with concurrent producers.
func TestQuota_ConcurrentSaver(t *testing.T) {
	store := NewMemoryQuotaStore()
	saver := newQuotaSaver(store, 5*time.Millisecond)
	defer saver.close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			saver.Save("default", "openai", int64(n), todayStr())
		}(i)
	}
	wg.Wait()
	time.Sleep(20 * time.Millisecond)
	// No data-race panic is the success criterion. Verify the last write
	// is one of the valid values (0..49).
	usage, _, _ := store.Load("default", "openai")
	if usage < 0 || usage > 49 {
		t.Errorf("usage = %d, want in [0, 49]", usage)
	}
}

// TestQuota_DefaultTenantFallback verifies that a router with an empty
// tenantID normalizes to "default" for store lookups, matching the
// pre-multi-tenant behavior.
func TestQuota_DefaultTenantFallback(t *testing.T) {
	store := NewMemoryQuotaStore()
	providers := []RouterProvider{
		{Name: "openai", Enabled: true, QuotaDaily: 1000, APIKey: "k", Priority: 1},
	}

	// Empty tenantID — should be treated as "default".
	router := newQuotaTestRouter(providers, store, "")
	defer router.CloseQuota()

	router.recordSuccess("openai", 10, 7)
	router.FlushQuota()

	// The store entry should be under "default", not "".
	if usage, _, _ := store.Load("default", "openai"); usage != 7 {
		t.Errorf("default tenant usage = %d, want 7", usage)
	}
	if r := router.effectiveTenant(); r != "default" {
		t.Errorf("effectiveTenant() = %q, want %q", r, "default")
	}
}

// TestQuota_BackwardCompat_NoStore verifies that a router constructed
// without a QuotaStore (the pre-quota-persistence pattern) still behaves
// exactly as before: recordSuccess updates DailyUsage in memory, no
// persistence is attempted, and getActiveProviders works normally.
func TestQuota_BackwardCompat_NoStore(t *testing.T) {
	providers := []RouterProvider{
		{Name: "openai", Enabled: true, QuotaDaily: 100, APIKey: "k", Priority: 1},
	}
	// Construct directly, like the existing tests — no initQuotaPersistence.
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyPriority,
		maxRetry:  3,
	}
	// quotaStore, quotaSaver are nil. recordSuccess must not call them.
	r.recordSuccess("openai", 10, 50)

	stats := r.GetProviderStats()
	if s, ok := stats["openai"]; !ok || s.DailyUsage != 50 {
		t.Errorf("DailyUsage without store = %v, want 50", stats["openai"])
	}

	// Below quota: provider is active.
	if active := r.getActiveProviders(); len(active) != 1 {
		t.Errorf("active providers = %d, want 1", len(active))
	}

	// Exhaust quota in memory; provider should be excluded.
	r.recordSuccess("openai", 10, 60) // total 110 >= 100
	if active := r.getActiveProviders(); len(active) != 0 {
		t.Errorf("active providers after quota exhaustion = %d, want 0", len(active))
	}
}

// TestQuota_PathTraversalRejected verifies that crafted tenant/provider
// values containing path-traversal sequences ("../../etc", "../../etc/passwd")
// cannot escape the store's base directory. With the hash-based pathFor,
// the resolved path is always <base>/<hex>/<hex>.json under base, and the
// defense-in-depth safePathFor containment check rejects any escape.
//
// Regression: the original pathFor used tenant/provider verbatim, so an
// attacker-supplied tenant="../../etc" or provider="../../etc/passwd" could
// write arbitrary files outside base — a Critical path-traversal issue for
// the financial SaaS multi-tenant scenario.
func TestQuota_PathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	store := NewFileQuotaStore(dir)

	// Malicious tenant + provider that would escape base under the old
	// (verbatim) path scheme. Under the hash scheme the write must
	// succeed (the values are valid, just hashed) AND land strictly
	// inside base.
	maliciousTenant := "../../etc"
	maliciousProvider := "../../etc/passwd"
	if err := store.Save(maliciousTenant, maliciousProvider, 1, "2026-01-01"); err != nil {
		t.Fatalf("Save of traversal-shaped values returned error (hash scheme should accept them): %v", err)
	}

	// The escape target (dir/../../etc) must NOT have been created
	// outside base.
	escapeTarget := filepath.Join(dir, maliciousTenant)
	if _, err := os.Stat(escapeTarget); err == nil {
		t.Errorf("path traversal escaped base: %s exists outside store base", escapeTarget)
	}

	// Every written file must live strictly under base. Walk base and
	// assert each file's path is contained within dir.
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk base: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected exactly 1 file under base, got %d: %v", len(files), files)
	}
	written := files[0]
	rel, err := filepath.Rel(dir, written)
	if err != nil {
		t.Fatalf("filepath.Rel(%s, %s): %v", dir, written, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == "." {
		t.Errorf("written file escapes base: rel=%q (file=%s)", rel, written)
	}

	// The hashed path round-trips: Load returns the saved value,
	// proving the same hash is used for write and read.
	usage, day, err := store.Load(maliciousTenant, maliciousProvider)
	if err != nil {
		t.Fatalf("Load of traversal-shaped values: %v", err)
	}
	if usage != 1 || day != "2026-01-01" {
		t.Errorf("Load = (%d, %q), want (1, \"2026-01-01\")", usage, day)
	}

	// Clear must also stay within base (remove the hashed file).
	if err := store.Clear(maliciousTenant, maliciousProvider); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(written); !os.IsNotExist(err) {
		t.Errorf("expected hashed file removed after Clear; stat err = %v", err)
	}
}

// TestQuota_SymlinkAttackRejected verifies the H-2 symlink-attack defense.
// Although pathFor hashes every path segment (so ".." / "/" can never appear
// in a segment), an attacker who can write under a hashed directory could
// plant a symlink at the quota file path — e.g.
// base/<tenantHash>/<provider>.json -> /etc/cron.d/backdoor — and let the
// next Save's tmp+rename atomic write follow the symlink and overwrite the
// target. Save must refuse to write through the symlink, and the outside
// target must not be created or modified.
//
// This complements TestQuota_PathTraversalRejected: that test covers
// path-traversal via crafted tenant/provider names (blocked by hashing);
// this test covers symlink planting under an already-hashed directory
// (blocked by validateNoSymlink + the pre-rename Lstat check).
func TestQuota_SymlinkAttackRejected(t *testing.T) {
	dir := t.TempDir()
	store := NewFileQuotaStore(dir)

	// Outside target the attacker hopes Save will overwrite. A fresh temp
	// dir lets us assert it is never created.
	outsideRoot := t.TempDir()
	outsideTarget := filepath.Join(outsideRoot, "outside.txt")

	// Construct the exact hashed path the store targets for (tenantA, openai).
	tenantDir := filepath.Join(dir, quotaHashSegment("tenantA"))
	finalPath := filepath.Join(tenantDir, quotaHashSegment("openai")+".json")
	if err := os.MkdirAll(tenantDir, 0o700); err != nil {
		t.Fatalf("mkdir tenant dir: %v", err)
	}
	// Plant the symlink: finalPath -> outsideTarget (outside base).
	if err := os.Symlink(outsideTarget, finalPath); err != nil {
		t.Fatalf("plant symlink: %v", err)
	}

	// Save must reject rather than write through the symlink.
	err := store.Save("tenantA", "openai", 999, "2026-01-01")
	if err == nil {
		t.Fatalf("Save through symlink should have been rejected, got nil")
	}

	// The outside target must NOT have been created or overwritten.
	if _, err := os.Stat(outsideTarget); err == nil {
		t.Errorf("outside target was created/overwritten via symlink: %s exists", outsideTarget)
	}

	// Load through a symlink that resolves to an EXISTING outside file must
	// be rejected, otherwise the attacker could read an outside file's
	// contents as quota data. Point the symlink at a real outside file with
	// sentinel content and confirm Load refuses (and never returns the
	// sentinel).
	outsideExisting := filepath.Join(outsideRoot, "secret.txt")
	if err := os.WriteFile(outsideExisting, []byte(`{"usage":9999}`), 0o600); err != nil {
		t.Fatalf("write outside sentinel: %v", err)
	}
	if err := os.Remove(finalPath); err != nil {
		t.Fatalf("remove dangling symlink: %v", err)
	}
	if err := os.Symlink(outsideExisting, finalPath); err != nil {
		t.Fatalf("replant symlink at existing outside file: %v", err)
	}
	if usage, _, err := store.Load("tenantA", "openai"); err == nil {
		t.Errorf("Load through symlink to existing outside file should have been rejected, got usage=%d nil err", usage)
	}

	// After the operator removes the planted symlink, a legitimate Save
	// must succeed (the defense refuses only symlinks, not normal writes).
	if err := os.Remove(finalPath); err != nil {
		t.Fatalf("remove planted symlink: %v", err)
	}
	if err := store.Save("tenantA", "openai", 42, "2026-01-01"); err != nil {
		t.Errorf("Save after removing symlink should succeed: %v", err)
	}
	if usage, day, err := store.Load("tenantA", "openai"); err != nil || usage != 42 || day != "2026-01-01" {
		t.Errorf("Load after clean Save = (%d, %q, %v), want (42, \"2026-01-01\", nil)", usage, day, err)
	}
}

// TestQuota_SymlinkParentDirRejected verifies the H-2 defense against a
// symlink planted on a parent directory (base/<tenantHash> -> /tmp/outside),
// which MkdirAll would follow and which the per-file Lstat check alone would
// not catch. The post-MkdirAll validateNoSymlink on the parent directory
// must reject the Save before any file is written.
func TestQuota_SymlinkParentDirRejected(t *testing.T) {
	dir := t.TempDir()
	store := NewFileQuotaStore(dir)

	// Outside directory the attacker wants writes redirected into.
	outsideRoot := t.TempDir()

	// Plant a symlink at the hashed tenant directory pointing outside base.
	tenantDir := filepath.Join(dir, quotaHashSegment("tenantA"))
	if err := os.Symlink(outsideRoot, tenantDir); err != nil {
		t.Fatalf("plant parent symlink: %v", err)
	}

	// Save must reject because the parent dir resolves outside base.
	err := store.Save("tenantA", "openai", 999, "2026-01-01")
	if err == nil {
		t.Fatalf("Save through parent-dir symlink should have been rejected, got nil")
	}

	// No file should have been created inside the outside target.
	// outsideRoot is a temp dir we own and should be fully walkable, so any
	// walk error indicates a broken test setup — surface it via t.Fatalf
	// instead of swallowing it, consistent with the other walk handlers in
	// this file.
	var leaked []string
	if err := filepath.Walk(outsideRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			leaked = append(leaked, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk outside root: %v", err)
	}
	if len(leaked) != 0 {
		t.Errorf("file was written outside base via parent-dir symlink: %v", leaked)
	}
}

// TestQuota_EmptyTenantProviderRejected verifies that Load/Save/Clear reject
// empty tenant or provider strings with an error. Empty values are never
// legitimate (the router normalizes empty tenant to "default" before calling
// the store), and accepting them would produce a hash-keyed but meaningless
// entry that could shadow real data.
func TestQuota_EmptyTenantProviderRejected(t *testing.T) {
	dir := t.TempDir()
	store := NewFileQuotaStore(dir)

	checks := []struct {
		name     string
		tenant   string
		provider string
	}{
		{"empty tenant", "", "openai"},
		{"empty provider", "default", ""},
		{"both empty", "", ""},
	}
	for _, c := range checks {
		if _, _, err := store.Load(c.tenant, c.provider); err == nil {
			t.Errorf("Load(%q,%q): expected error for %s, got nil", c.tenant, c.provider, c.name)
		}
		if err := store.Save(c.tenant, c.provider, 1, "2026-01-01"); err == nil {
			t.Errorf("Save(%q,%q): expected error for %s, got nil", c.tenant, c.provider, c.name)
		}
		if err := store.Clear(c.tenant, c.provider); err == nil {
			t.Errorf("Clear(%q,%q): expected error for %s, got nil", c.tenant, c.provider, c.name)
		}
	}

	// Sanity: no files written by the rejected Save calls.
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			t.Errorf("unexpected file written after empty-value rejections: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// TestQuota_DailyResetUTC verifies that the default daily-reset boundary is
// UTC (not host local time), so cross-timezone financial deployments reset
// at a consistent instant. It uses two fixed timezones whose dates ALWAYS
// differ at any instant (UTC+14 and UTC-12) to make the assertion robust
// regardless of when the test runs or the host's local TZ.
//
// A record stamped with the "behind" (UTC-12) date is stale relative to the
// "ahead" (UTC+14) today, so a router configured for the ahead timezone must
// reset DailyUsage to 0 on load; and vice-versa. If the router ignored the
// configured timezone and used host local time, one of these directions
// would fail whenever the host's local date disagreed with the configured
// timezone's date.
func TestQuota_DailyResetUTC(t *testing.T) {
	// ahead (UTC+14, e.g. Line Islands) always has a date >= behind's.
	ahead := time.FixedZone("AHEAD14", 14*3600)
	behind := time.FixedZone("BEHIND12", -12*3600)

	todayAhead := time.Now().In(ahead).Format("2006-01-02")
	todayBehind := time.Now().In(behind).Format("2006-01-02")
	if todayAhead == todayBehind {
		t.Skipf("UTC+14 and UTC-12 dates coincide (%s); cannot test tz-sensitive reset reliably", todayAhead)
	}

	// Default router (no WithTimeZone) must compute "today" as UTC.
	defaultRouter := &LLMRouter{}
	if got, want := defaultRouter.todayUTC(), time.Now().UTC().Format("2006-01-02"); got != want {
		t.Errorf("default todayUTC() = %q, want UTC %q", got, want)
	}

	providers := []RouterProvider{
		{Name: "openai", Enabled: true, QuotaDaily: 1000, APIKey: "k", Priority: 1},
	}

	// Router A: ahead timezone. Seed with the behind date (stale in
	// ahead tz) → must reset to 0.
	storeA := NewMemoryQuotaStore()
	if err := storeA.Save("default", "openai", 800, todayBehind); err != nil {
		t.Fatalf("seed A: %v", err)
	}
	routerA := &LLMRouter{
		providers:  append([]RouterProvider(nil), providers...),
		stats:      make(map[string]*ProviderStats),
		strategy:   config.RouterStrategyPriority,
		maxRetry:   3,
		tenantID:   "default",
		quotaStore: storeA,
		timeZone:   ahead,
	}
	routerA.initQuotaPersistence()
	defer routerA.CloseQuota()

	if got := routerA.todayUTC(); got != todayAhead {
		t.Errorf("routerA todayUTC() = %q, want ahead-tz %q", got, todayAhead)
	}
	sA := routerA.GetProviderStats()["openai"]
	if sA.DailyUsage != 0 {
		t.Errorf("routerA DailyUsage = %d, want 0 (behind-date is stale in ahead tz)", sA.DailyUsage)
	}
	if sA.LastResetDate != todayAhead {
		t.Errorf("routerA LastResetDate = %q, want ahead-tz %q", sA.LastResetDate, todayAhead)
	}

	// Router B: behind timezone. Seed with the ahead date (stale in
	// behind tz) → must reset to 0.
	storeB := NewMemoryQuotaStore()
	if err := storeB.Save("default", "openai", 900, todayAhead); err != nil {
		t.Fatalf("seed B: %v", err)
	}
	routerB := &LLMRouter{
		providers:  append([]RouterProvider(nil), providers...),
		stats:      make(map[string]*ProviderStats),
		strategy:   config.RouterStrategyPriority,
		maxRetry:   3,
		tenantID:   "default",
		quotaStore: storeB,
		timeZone:   behind,
	}
	routerB.initQuotaPersistence()
	defer routerB.CloseQuota()

	if got := routerB.todayUTC(); got != todayBehind {
		t.Errorf("routerB todayUTC() = %q, want behind-tz %q", got, todayBehind)
	}
	sB := routerB.GetProviderStats()["openai"]
	if sB.DailyUsage != 0 {
		t.Errorf("routerB DailyUsage = %d, want 0 (ahead-date is stale in behind tz)", sB.DailyUsage)
	}
	if sB.LastResetDate != todayBehind {
		t.Errorf("routerB LastResetDate = %q, want behind-tz %q", sB.LastResetDate, todayBehind)
	}
}

// TestQuota_WithTimeZone verifies that WithTimeZone configures the business
// timezone used for daily quota reset. Uses UTC+8 (a common trading-center
// timezone) as requested; the reset must follow that timezone rather than
// host local time. The stale-record seed uses yesterday in the business
// timezone, which is always != business-today, so the reset deterministically
// fires.
func TestQuota_WithTimeZone(t *testing.T) {
	biz := time.FixedZone("BIZ", 8*3600)
	todayBiz := time.Now().In(biz).Format("2006-01-02")
	yesterdayBiz := time.Now().In(biz).AddDate(0, 0, -1).Format("2006-01-02")
	if todayBiz == yesterdayBiz {
		t.Skipf("business-tz today and yesterday coincide (%s)", todayBiz)
	}

	providers := []RouterProvider{
		{Name: "openai", Enabled: true, QuotaDaily: 1000, APIKey: "k", Priority: 1},
	}

	// Seed a stale (yesterday in biz tz) record.
	store := NewMemoryQuotaStore()
	if err := store.Save("default", "openai", 800, yesterdayBiz); err != nil {
		t.Fatalf("seed: %v", err)
	}

	router := &LLMRouter{
		providers:  append([]RouterProvider(nil), providers...),
		stats:      make(map[string]*ProviderStats),
		strategy:   config.RouterStrategyPriority,
		maxRetry:   3,
		tenantID:   "default",
		quotaStore: store,
	}
	WithTimeZone(biz)(router)
	router.initQuotaPersistence()
	defer router.CloseQuota()

	// todayUTC must reflect the business timezone, not host local / UTC.
	if got := router.todayUTC(); got != todayBiz {
		t.Errorf("todayUTC() with WithTimeZone(BIZ) = %q, want %q", got, todayBiz)
	}

	s, ok := router.GetProviderStats()["openai"]
	if !ok {
		t.Fatal("no stats for openai after init")
	}
	if s.DailyUsage != 0 {
		t.Errorf("DailyUsage = %d, want 0 (biz-yesterday is stale in biz tz)", s.DailyUsage)
	}
	if s.LastResetDate != todayBiz {
		t.Errorf("LastResetDate = %q, want biz-tz %q", s.LastResetDate, todayBiz)
	}

	// A current (biz-today) record must NOT be reset: it loads usage.
	store2 := NewMemoryQuotaStore()
	if err := store2.Save("default", "openai", 123, todayBiz); err != nil {
		t.Fatalf("seed2: %v", err)
	}
	router2 := &LLMRouter{
		providers:  append([]RouterProvider(nil), providers...),
		stats:      make(map[string]*ProviderStats),
		strategy:   config.RouterStrategyPriority,
		maxRetry:   3,
		tenantID:   "default",
		quotaStore: store2,
	}
	WithTimeZone(biz)(router2)
	router2.initQuotaPersistence()
	defer router2.CloseQuota()

	s2 := router2.GetProviderStats()["openai"]
	if s2.DailyUsage != 123 {
		t.Errorf("DailyUsage for current biz-today record = %d, want 123 (not reset)", s2.DailyUsage)
	}
	if s2.LastResetDate != todayBiz {
		t.Errorf("LastResetDate = %q, want biz-tz %q", s2.LastResetDate, todayBiz)
	}
}
