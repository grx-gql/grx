package http

import (
	"fmt"
	"strings"
	"sync"
)

const maxAPQQueryBytes = 64 << 10

type persistedQueryError struct {
	message string
	code    string
}

func (e *persistedQueryError) Error() string { return e.message }

type persistedQueryCache struct {
	mu      sync.RWMutex
	queries map[string]string
	order   []string
	next    int
}

func newPersistedQueryCache(capacity int) *persistedQueryCache {
	return &persistedQueryCache{queries: make(map[string]string, capacity), order: make([]string, capacity)}
}

func (c *persistedQueryCache) lookup(hash string) (string, bool) {
	c.mu.RLock()
	query, ok := c.queries[hash]
	c.mu.RUnlock()
	return query, ok
}

func (c *persistedQueryCache) store(hash, query string) error {
	if len(query) > maxAPQQueryBytes {
		return fmt.Errorf("persisted query exceeds %d byte cache limit", maxAPQQueryBytes)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.queries[hash]; exists {
		return nil
	}
	delete(c.queries, c.order[c.next])
	// Clone to avoid retaining a larger decoded request backing the hash or query.
	hash = strings.Clone(hash)
	c.queries[hash] = strings.Clone(query)
	c.order[c.next] = hash
	c.next = (c.next + 1) % len(c.order)
	return nil
}
