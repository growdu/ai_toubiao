package router

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/bidwriter/services/router-svc/internal/model"
)

// CacheEntry is a cached chat response plus its expiry and lookup key.
type CacheEntry struct {
	Key       string
	Response  *model.ChatResponse
	ExpiresAt time.Time
}

// expired reports whether the entry has passed its TTL.
func (e *CacheEntry) expired() bool {
	return time.Now().After(e.ExpiresAt)
}

// LRUCache is a thread-safe LRU keyed by (task, model, prompt-hash, temperature).
// Implementation: doubly-linked list + map. Eviction is O(1).
type LRUCache struct {
	mu       sync.Mutex
	capacity int
	order    *list.List               // front = most recently used
	index    map[string]*list.Element // key → list element
}

// NewLRUCache builds a cache with the given max entries (capacity).
func NewLRUCache(capacity int) *LRUCache {
	if capacity <= 0 {
		capacity = 1
	}
	return &LRUCache{
		capacity: capacity,
		order:    list.New(),
		index:    map[string]*list.Element{},
	}
}

// Get returns the cached response for key, or nil if absent/expired.
func (c *LRUCache) Get(key string) *model.ChatResponse {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.index[key]
	if !ok {
		return nil
	}
	entry := el.Value.(*CacheEntry)
	if entry.expired() {
		c.order.Remove(el)
		delete(c.index, key)
		return nil
	}
	c.order.MoveToFront(el)
	return entry.Response
}

// Set inserts/updates a cache entry with the given TTL.
func (c *LRUCache) Set(key string, resp *model.ChatResponse, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.index[key]; ok {
		el.Value = &CacheEntry{Key: key, Response: resp, ExpiresAt: time.Now().Add(ttl)}
		c.order.MoveToFront(el)
		return
	}
	if c.order.Len() >= c.capacity {
		// evict LRU
		tail := c.order.Back()
		if tail != nil {
			tailEntry := tail.Value.(*CacheEntry)
			c.order.Remove(tail)
			delete(c.index, tailEntry.Key)
		}
	}
	entry := &CacheEntry{Key: key, Response: resp, ExpiresAt: time.Now().Add(ttl)}
	el := c.order.PushFront(entry)
	c.index[key] = el
}

// Len returns the number of live entries.
func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Clear empties the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.order = list.New()
	c.index = map[string]*list.Element{}
}

// CacheKey builds a deterministic key for a chat request.
func CacheKey(task model.Task, modelName, prompt string, temperature float64) string {
	h := sha256.Sum256([]byte(prompt))
	return string(task) + "|" + modelName + "|" + hex.EncodeToString(h[:]) + "|" + formatTemp(temperature)
}

func formatTemp(t float64) string {
	// round to 2 decimals for cache stability
	if t == 0 {
		return "0.00"
	}
	// 2-decimal via int conversion
	return formatFloat(t, 2)
}

func formatFloat(f float64, decimals int) string {
	mult := 1.0
	for i := 0; i < decimals; i++ {
		mult *= 10
	}
	rounded := float64(int64(f*mult+0.5)) / mult
	// manual formatting to avoid strconv import in hot path
	if rounded == float64(int64(rounded)) {
		return intStr(int64(rounded)) + "." + zeros(decimals)
	}
	return floatStr(rounded, decimals)
}

func zeros(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = '0'
	}
	return string(out)
}

func intStr(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func floatStr(f float64, decimals int) string {
	intPart := int64(f)
	frac := f - float64(intPart)
	if frac < 0 {
		frac = -frac
	}
	mult := 1.0
	for i := 0; i < decimals; i++ {
		mult *= 10
	}
	fracInt := int64(frac*mult + 0.5)
	// carry
	if fracInt >= int64(mult) {
		intPart++
		fracInt -= int64(mult)
	}
	s := intStr(intPart)
	s += "."
	fracStr := intStr(fracInt)
	if len(fracStr) < decimals {
		s += zeros(decimals-len(fracStr)) + fracStr
	} else {
		s += fracStr
	}
	return s
}