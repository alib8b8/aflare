package cache

import (
	"container/list"
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

func GenerateKey(prompt string, params map[string]interface{}) string {
	h := sha256.New()
	h.Write([]byte(prompt))
	if params != nil {
		paramBytes, _ := json.Marshal(params)
		h.Write(paramBytes)
	}
	return hex.EncodeToString(h.Sum(nil))
}

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

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.lruList.Remove(elem)
		delete(c.items, key)
	}
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lruList = list.New()
	c.hits = 0
	c.misses = 0
}

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

func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lruList.Len()
}
