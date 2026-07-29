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

package cache

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSetAndGet(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 10,
		TTL:        time.Minute,
	})

	c.Set("key1", "value1")
	c.Set("key2", "value2")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}

	val, ok = c.Get("key2")
	if !ok {
		t.Fatal("expected key2 to be found")
	}
	if val != "value2" {
		t.Errorf("expected value2, got %s", val)
	}
}

func TestGet_Miss(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 10,
		TTL:        time.Minute,
	})

	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected miss for nonexistent key")
	}
}

func TestSet_UpdateExisting(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 10,
		TTL:        time.Minute,
	})

	c.Set("key1", "value1")
	c.Set("key1", "value2")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if val != "value2" {
		t.Errorf("expected value2, got %s", val)
	}

	if c.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", c.Len())
	}
}

func TestDelete(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 10,
		TTL:        time.Minute,
	})

	c.Set("key1", "value1")
	c.Delete("key1")

	_, ok := c.Get("key1")
	if ok {
		t.Error("expected key1 to be deleted")
	}
}

func TestClear(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 10,
		TTL:        time.Minute,
	})

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Clear()

	if c.Len() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", c.Len())
	}

	stats := c.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("expected stats to be reset, got hits=%d misses=%d", stats.Hits, stats.Misses)
	}
}

func TestLRUEviction(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 3,
		TTL:        time.Minute,
	})

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")

	c.Get("key1")

	c.Set("key4", "value4")

	if c.Len() != 3 {
		t.Errorf("expected 3 entries, got %d", c.Len())
	}

	_, ok := c.Get("key2")
	if ok {
		t.Error("expected key2 to be evicted")
	}

	_, ok = c.Get("key1")
	if !ok {
		t.Error("expected key1 to still exist")
	}

	_, ok = c.Get("key3")
	if !ok {
		t.Error("expected key3 to still exist")
	}

	_, ok = c.Get("key4")
	if !ok {
		t.Error("expected key4 to exist")
	}
}

func TestTTLExpiration(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 10,
		TTL:        50 * time.Millisecond,
	})

	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("key1")
	if ok {
		t.Error("expected key1 to be expired")
	}
}

func TestDisabled(t *testing.T) {
	c := New(Config{
		Enabled:    false,
		MaxEntries: 10,
		TTL:        time.Minute,
	})

	c.Set("key1", "value1")

	_, ok := c.Get("key1")
	if ok {
		t.Error("expected miss when cache is disabled")
	}

	if c.Len() != 0 {
		t.Errorf("expected 0 entries when disabled, got %d", c.Len())
	}
}

func TestStats(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 10,
		TTL:        time.Minute,
	})

	c.Set("key1", "value1")

	c.Get("key1")
	c.Get("key1")
	c.Get("nonexistent")

	stats := c.Stats()

	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.Total != 3 {
		t.Errorf("expected 3 total, got %d", stats.Total)
	}
	expectedRate := 2.0 / 3.0
	if stats.HitRate != expectedRate {
		t.Errorf("expected hit rate %f, got %f", expectedRate, stats.HitRate)
	}
}

func TestGenerateKey(t *testing.T) {
	key1 := GenerateKey("hello", map[string]interface{}{"model": "gpt-4"})
	key2 := GenerateKey("hello", map[string]interface{}{"model": "gpt-4"})
	key3 := GenerateKey("world", map[string]interface{}{"model": "gpt-4"})
	key4 := GenerateKey("hello", map[string]interface{}{"model": "gpt-3.5"})
	key5 := GenerateKey("hello", nil)

	if key1 != key2 {
		t.Error("same inputs should produce same key")
	}
	if key1 == key3 {
		t.Error("different prompts should produce different keys")
	}
	if key1 == key4 {
		t.Error("different params should produce different keys")
	}
	if key1 == key5 {
		t.Error("nil params should produce different key")
	}

	if len(key1) != 64 {
		t.Errorf("expected 64-char hex string, got length %d", len(key1))
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 100,
		TTL:        time.Minute,
	})

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "key" + string(rune('0'+id%10))
				c.Set(key, "value")
				c.Get(key)
			}
		}(i)
	}

	wg.Wait()

	stats := c.Stats()
	if stats.Total == 0 {
		t.Error("expected some operations")
	}
}

func TestDefaultMaxEntries(t *testing.T) {
	c := New(Config{
		Enabled: true,
		TTL:     time.Minute,
	})

	for i := 0; i < 200; i++ {
		key := "key" + string(rune('0'+i%10))
		c.Set(key, "value")
	}

	if c.Len() > 100 {
		t.Errorf("expected at most 100 entries (default), got %d", c.Len())
	}
}

// TestStartCleanup_RemovesExpired verifies the background goroutine evicts
// expired entries even when they are never accessed again (the lazy-cleanup
// path in Get would otherwise leave them resident until LRU pressure evicts
// them — the exact memory-retention gap StartCleanup closes).
func TestStartCleanup_RemovesExpired(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 100,
		TTL:        40 * time.Millisecond,
	})

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	if c.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", c.Len())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.StartCleanup(ctx, 20*time.Millisecond)

	// Wait past TTL plus at least one cleanup tick. Poll rather than fixed
	// sleep to keep the test fast on quick hosts while staying robust on
	// slow CI (ticker resolution is coarse under load).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Len() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if c.Len() != 0 {
		t.Errorf("expected all expired entries to be cleaned up, got %d remaining", c.Len())
	}
}

// TestStartCleanup_StopsOnCancel verifies the goroutine exits when ctx is
// cancelled, so it does not leak for the lifetime of the process.
func TestStartCleanup_StopsOnCancel(t *testing.T) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 10,
		TTL:        time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.StartCleanup(ctx, 10*time.Millisecond)
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// goroutine exited as expected
	case <-time.After(2 * time.Second):
		t.Error("StartCleanup did not stop after ctx cancellation")
	}
}

// TestStartCleanup_DisabledNoop verifies StartCleanup returns immediately
// when the cache is disabled, so no goroutine is left running.
func TestStartCleanup_DisabledNoop(t *testing.T) {
	c := New(Config{
		Enabled:    false,
		MaxEntries: 10,
		TTL:        time.Minute,
	})

	done := make(chan struct{})
	go func() {
		c.StartCleanup(context.Background(), 10*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		// returned immediately as expected
	case <-time.After(time.Second):
		t.Error("StartCleanup should return immediately when cache is disabled")
	}
}
