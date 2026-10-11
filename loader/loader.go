// Package loader batches resolver lookups within an execution context.
// Register loaders with exec.WithLoaders; return Load's pending value from a
// schema.Resolver. The executor dispatches ready lookups before completing fields.
package loader

import (
	"context"
	"fmt"
	"sync"
)

// Key identifies one lookup within a Loader. Encode compound keys unambiguously.
type Key string

// Result holds one lookup's concrete schema-compatible value or error.
// Value must not be a thunk or another Pending lookup. A nil Value
// without an Err represents a missing nullable value.
type Result struct {
	Value any
	Err   error
}

// BatchFunc returns exactly one Result per key, in input order. A returned error
// fails every key in the batch. Database/service retries belong in this adapter,
// where the application can determine whether repeating the call is safe.
// The callback must not enqueue or dispatch further loader work.
type BatchFunc func(context.Context, []Key) ([]Result, error)

// Loader is an immutable registration for an external lookup adapter. Identity,
// rather than Name, separates caches. It may be shared by concurrent requests.
type Loader struct {
	name         string
	batch        BatchFunc
	maxBatchSize int
}

// ConfigError reports an invalid loader registration.
type ConfigError struct {
	Name   string
	Reason string
}

func (e *ConfigError) Error() string { return fmt.Sprintf("loader %q: %s", e.Name, e.Reason) }

// ScopeError reports usage outside the loader's registered execution scope.
type ScopeError struct {
	Name   string
	Reason string
}

func (e *ScopeError) Error() string { return fmt.Sprintf("loader scope %q: %s", e.Name, e.Reason) }

// BatchError reports a broken batch contract, including keys for diagnosis.
type BatchError struct {
	Name   string
	Keys   []Key
	Reason string
}

func (e *BatchError) Error() string {
	return fmt.Sprintf("loader %q batch %q: %s", e.Name, e.Keys, e.Reason)
}

// Pending is a queued lookup. Return it directly from a schema.Resolver; the
// executor replaces it with its value after dispatch. Use Read only after Dispatch
// when driving a scope manually. Pending values must not outlive their scope.
type Pending struct {
	mu     sync.RWMutex
	result Result
	ready  bool
}

type scopeKey struct{}
type queue struct {
	loader  *Loader
	entries map[Key]*Pending
	keys    []Key
}
type scope struct {
	mu       sync.Mutex
	dispatch sync.Mutex
	queues   map[*Loader]*queue
	order    []*queue
	active   bool
}

// New validates a loader registration. name must be nonempty, batch must be
// non-nil, and maxBatchSize must be positive. There is no implicit size limit.
func New(name string, batch BatchFunc, maxBatchSize int) (*Loader, error) {
	if name == "" {
		return nil, &ConfigError{Name: name, Reason: "name must not be empty"}
	}
	if batch == nil {
		return nil, &ConfigError{Name: name, Reason: "batch function must not be nil"}
	}
	if maxBatchSize <= 0 {
		return nil, &ConfigError{Name: name, Reason: "maxBatchSize must be positive"}
	}
	return &Loader{name: name, batch: batch, maxBatchSize: maxBatchSize}, nil
}

// NewScope returns a child context with fresh queues and caches for registrations.
// It never changes the parent scope. Executors create one scope per query, per
// mutation root field, and per subscription event. Repeated registrations share
// one cache; nil or zero-value registrations are errors.
func NewScope(ctx context.Context, registrations []*Loader) (context.Context, error) {
	s := &scope{queues: make(map[*Loader]*queue, len(registrations))}
	for _, l := range registrations {
		if l == nil || l.batch == nil || l.maxBatchSize <= 0 {
			return nil, &ConfigError{Reason: "register a loader returned by New"}
		}
		if _, exists := s.queues[l]; exists {
			continue
		}
		q := &queue{loader: l, entries: make(map[Key]*Pending)}
		s.queues[l] = q
		s.order = append(s.order, q)
	}
	return context.WithValue(ctx, scopeKey{}, s), nil
}

// Load queues key once in the registered scope and returns its shared pending
// result. It performs no I/O. Concurrent resolvers may call Load safely. Results,
// including errors, are cached only for this scope.
func Load(ctx context.Context, l *Loader, key Key) (*Pending, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if l == nil {
		return nil, &ScopeError{Reason: "loader must not be nil"}
	}
	s, ok := ctx.Value(scopeKey{}).(*scope)
	if !ok {
		return nil, &ScopeError{Name: l.name, Reason: "no execution scope; register with exec.WithLoaders or call NewScope"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return nil, &ScopeError{Name: l.name, Reason: "batch callbacks must not enqueue lookups"}
	}
	q, ok := s.queues[l]
	if !ok {
		return nil, &ScopeError{Name: l.name, Reason: "loader is not registered in this execution scope"}
	}
	if p, ok := q.entries[key]; ok {
		return p, nil
	}
	p := &Pending{}
	q.entries[key] = p
	q.keys = append(q.keys, key)
	return p, nil
}

// Read returns a dispatched lookup's value or error. Reading a nil or unresolved
// Pending is an explicit error, rather than a blocking wait.
func Read(p *Pending) (any, error) {
	if p == nil {
		return nil, &ScopeError{Reason: "pending lookup must not be nil"}
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.ready {
		return nil, &ScopeError{Reason: "lookup has not been dispatched"}
	}
	return p.result.Value, p.result.Err
}

// Dispatch resolves queued lookups in registration order and bounded batches.
// Batch failures are stored in Pending results so unrelated fields can succeed.
// Cancellation fails unstarted batches without calling their adapters. Call only
// after ready resolvers have finished enqueueing, never from a BatchFunc.
func Dispatch(ctx context.Context) error {
	s, ok := ctx.Value(scopeKey{}).(*scope)
	if !ok {
		return &ScopeError{Reason: "no execution scope"}
	}
	if !s.dispatch.TryLock() {
		return &ScopeError{Reason: "dispatch is already running"}
	}
	defer s.dispatch.Unlock()
	s.mu.Lock()
	s.active = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.active = false; s.mu.Unlock() }()
	for _, q := range s.order {
		for start := 0; start < len(q.keys); start += q.loader.maxBatchSize {
			end := min(start+q.loader.maxBatchSize, len(q.keys))
			keys := q.keys[start:end]
			results, err := callBatch(ctx, q.loader, keys)
			for i, key := range keys {
				result := Result{Err: err}
				if err == nil {
					result = results[i]
				}
				p := q.entries[key]
				p.mu.Lock()
				p.result, p.ready = result, true
				p.mu.Unlock()
			}
		}
		q.keys = nil
	}
	return nil
}

func callBatch(ctx context.Context, l *Loader, keys []Key) (results []Result, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &BatchError{Name: l.name, Keys: append([]Key(nil), keys...), Reason: fmt.Sprintf("adapter panicked: %v", recovered)}
		}
	}()
	// The adapter owns its input slice; mutation must not corrupt queue identity.
	results, err = l.batch(ctx, append([]Key(nil), keys...))
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(results) != len(keys) {
		return nil, &BatchError{Name: l.name, Keys: append([]Key(nil), keys...), Reason: fmt.Sprintf("returned %d results for %d keys", len(results), len(keys))}
	}
	return results, nil
}
