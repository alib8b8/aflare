package cache

import (
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
