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
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

type Config struct {
	Enabled    bool
	MaxEntries int
	TTL        time.Duration
}

type Stats struct {
	Hits    int64
	Misses  int64
	Total   int64
	HitRate float64
}

type entry struct {
	key       string
	value     string
	expiresAt time.Time
}

type Cache struct {
	mu      sync.RWMutex
	config  Config
	items   map[string]*list.Element
	lruList *list.List
	hits    int64
	misses  int64
}

// New 根据配置创建一个带 TTL 与 LRU 淘汰的缓存实例。
// 若 MaxEntries 未设置或非正数，默认使用 100。
func New(config Config) *Cache {
	if config.MaxEntries <= 0 {
		config.MaxEntries = 100
	}
	return &Cache{
		config:  config,
		items:   make(map[string]*list.Element),
		lruList: list.New(),
	}
}

// GenerateKey 根据 prompt 与可选 params 生成稳定的 SHA256 缓存键。
func GenerateKey(prompt string, params map[string]interface{}) string {
	h := sha256.New()
	h.Write([]byte(prompt))
	if params != nil {
		paramBytes, _ := json.Marshal(params)
		h.Write(paramBytes)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Get 读取缓存值。命中且未过期时返回值与 true；未启用、缺失或过期时返回空串与 false。
func (c *Cache) Get(key string) (string, bool) {
	if !c.config.Enabled {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return "", false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		c.misses++
		return "", false
	}

	e := elem.Value.(*entry)
	if time.Now().After(e.expiresAt) {
		c.lruList.Remove(elem)
		delete(c.items, key)
		c.misses++
		return "", false
	}

	c.lruList.MoveToFront(elem)
	c.hits++
	return e.value, true
}

// Set 写入缓存项并刷新 TTL；超过容量时淘汰最久未访问的项。
// 未启用缓存时为空操作。
func (c *Cache) Set(key, value string) {
	if !c.config.Enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		e := elem.Value.(*entry)
		e.value = value
		e.expiresAt = time.Now().Add(c.config.TTL)
		c.lruList.MoveToFront(elem)
		return
	}

	if c.lruList.Len() >= c.config.MaxEntries {
		oldest := c.lruList.Back()
		if oldest != nil {
			oldEntry := oldest.Value.(*entry)
			delete(c.items, oldEntry.key)
			c.lruList.Remove(oldest)
		}
	}

	e := &entry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(c.config.TTL),
	}
	elem := c.lruList.PushFront(e)
	c.items[key] = elem
}

// Delete 删除指定键的缓存项，不存在时为空操作。
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.lruList.Remove(elem)
		delete(c.items, key)
	}
}

// Clear 清空所有缓存项并重置命中/未命中计数。
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lruList = list.New()
	c.hits = 0
	c.misses = 0
}

// Stats 返回当前缓存的命中、未命中、总数与命中率统计快照。
func (c *Cache) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return Stats{
		Hits:    c.hits,
		Misses:  c.misses,
		Total:   total,
		HitRate: hitRate,
	}
}

// Len 返回当前缓存中的条目数量。
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lruList.Len()
}

// cleanupExpired 移除所有已过期的缓存项。调用方必须持有 c.mu 写锁。
//
// 从 LRU 链表尾部（最久未访问）向前扫描：过期项越多越集中在尾部，
// 因此从尾部开始可在常见工作负载下提前结束扫描（未过期即可停止）。
// 这里仍完整扫描以保证正确性——TTL 相近时过期项可能散布在链表任意位置。
func (c *Cache) cleanupExpired() {
	now := time.Now()
	for elem := c.lruList.Back(); elem != nil; {
		next := elem.Prev()
		e := elem.Value.(*entry)
		if now.After(e.expiresAt) {
			delete(c.items, e.key)
			c.lruList.Remove(elem)
		}
		elem = next
	}
}

// StartCleanup 启动后台 goroutine 定期清理过期缓存项，直到 ctx 被取消。
// 调用方应以 `go c.StartCleanup(ctx, interval)` 形式启动，并通过取消 ctx
// 来停止清理（例如在应用关闭时）。
//
// 未启用缓存时为空操作直接返回，避免空转 goroutine。
//
// interval 控制清理频率；建议设为 TTL 的 1/4 ~ 1/2，在内存滞留与 CPU 开销
// 间取得平衡。interval<=0 时使用 TTL（若 TTL 也<=0 则不清理）。
//
// 注意：本方法会阻塞直到 ctx.Done()。仅依靠懒清理（Get 命中过期项时删除）
// 会导致冷 key 永久驻留 map/lruList 直到 LRU 容量满被挤出，形成内存滞留；
// 后台定期清理弥补这一缺口。
func (c *Cache) StartCleanup(ctx context.Context, interval time.Duration) {
	if !c.config.Enabled {
		return
	}
	if interval <= 0 {
		interval = c.config.TTL
	}
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			c.cleanupExpired()
			c.mu.Unlock()
		}
	}
}
