package cache

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkCacheGetSet(b *testing.B) {
	c := New(Config{
		Enabled:    true,
		MaxEntries: 1000,
		TTL:        time.Minute,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10000; j++ {
			key := fmt.Sprintf("key%d", j)
			c.Set(key, "value")
			c.Get(key)
		}
	}
}
