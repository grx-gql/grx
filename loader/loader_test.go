package loader

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestBatchDeduplicationOrderAndLimit(t *testing.T) {
	var batches [][]Key
	l, err := New("users", func(_ context.Context, keys []Key) ([]Result, error) {
		batches = append(batches, append([]Key(nil), keys...))
		results := make([]Result, len(keys))
		for i, key := range keys {
			results[i].Value = string(key)
		}
		return results, nil
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := NewScope(context.Background(), []*Loader{l, l})
	if err != nil {
		t.Fatal(err)
	}
	var pending []*Pending
	for _, key := range []Key{"b", "a", "b", "c"} {
		p, err := Load(ctx, l, key)
		if err != nil {
			t.Fatal(err)
		}
		pending = append(pending, p)
	}
	if pending[0] != pending[2] {
		t.Fatal("duplicate lookup was not shared")
	}
	if _, err := Read(pending[0]); err == nil {
		t.Fatal("read before dispatch succeeded")
	}
	if err := Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(batches, [][]Key{{"b", "a"}, {"c"}}) {
		t.Fatalf("batches %v", batches)
	}
	for i, want := range []string{"b", "a", "b", "c"} {
		got, err := Read(pending[i])
		if got != want || err != nil {
			t.Fatalf("got %v, %v; want %s", got, err, want)
		}
	}
	if _, err := Load(ctx, l, "a"); err != nil {
		t.Fatal(err)
	}
	if err := Dispatch(ctx); err != nil || len(batches) != 2 {
		t.Fatalf("cache redispatched: %v %v", batches, err)
	}
}

func TestScopeAndLoaderIsolation(t *testing.T) {
	var calls int
	batch := func(_ context.Context, keys []Key) ([]Result, error) {
		calls++
		return []Result{{Value: calls}}, nil
	}
	one, _ := New("same-name", batch, 1)
	two, _ := New("same-name", batch, 1)
	for range 2 {
		ctx, err := NewScope(context.Background(), []*Loader{one, two})
		if err != nil {
			t.Fatal(err)
		}
		a, _ := Load(ctx, one, "id")
		b, _ := Load(ctx, two, "id")
		if err := Dispatch(ctx); err != nil {
			t.Fatal(err)
		}
		av, _ := Read(a)
		bv, _ := Read(b)
		if av == bv {
			t.Fatal("different loaders shared a value")
		}
	}
	if calls != 4 {
		t.Fatalf("calls %d", calls)
	}
}

func TestBatchFailures(t *testing.T) {
	lookupErr := errors.New("missing user")
	for _, tc := range []struct {
		name  string
		batch BatchFunc
		want  string
	}{
		{"cardinality", func(context.Context, []Key) ([]Result, error) { return nil, nil }, `loader "users" batch ["one"]: returned 0 results for 1 keys`},
		{"batch error", func(context.Context, []Key) ([]Result, error) { return nil, lookupErr }, "missing user"},
		{"panic", func(context.Context, []Key) ([]Result, error) { panic("broken adapter") }, `loader "users" batch ["one"]: adapter panicked: broken adapter`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, _ := New("users", tc.batch, 8)
			ctx, _ := NewScope(context.Background(), []*Loader{l})
			p, _ := Load(ctx, l, "one")
			if err := Dispatch(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := Read(p); err == nil || err.Error() != tc.want {
				t.Fatalf("batch error = %v, want %q", err, tc.want)
			}
		})
	}
	l, _ := New("users", func(context.Context, []Key) ([]Result, error) {
		return []Result{{Err: lookupErr}, {Value: "ok"}}, nil
	}, 8)
	ctx, _ := NewScope(context.Background(), []*Loader{l})
	a, _ := Load(ctx, l, "bad")
	b, _ := Load(ctx, l, "good")
	if err := Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(a); !errors.Is(err, lookupErr) {
		t.Fatal(err)
	}
	if value, err := Read(b); value != "ok" || err != nil {
		t.Fatalf("%v %v", value, err)
	}
}

func TestCancellationPreventsDispatch(t *testing.T) {
	l, _ := New("users", func(context.Context, []Key) ([]Result, error) {
		t.Fatal("canceled batch executed")
		return nil, nil
	}, 8)
	ctx, cancel := context.WithCancel(context.Background())
	ctx, _ = NewScope(ctx, []*Loader{l})
	p, _ := Load(ctx, l, "one")
	cancel()
	if err := Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(p); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := Load(ctx, l, "two"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestRegistrationErrors(t *testing.T) {
	batch := func(context.Context, []Key) ([]Result, error) { return nil, nil }
	for _, tc := range []struct {
		name  string
		batch BatchFunc
		size  int
		want  string
	}{{"", batch, 1, `loader "": name must not be empty`}, {"x", nil, 1, `loader "x": batch function must not be nil`}, {"x", batch, 0, `loader "x": maxBatchSize must be positive`}} {
		_, err := New(tc.name, tc.batch, tc.size)
		var configErr *ConfigError
		if !errors.As(err, &configErr) || err.Error() != tc.want {
			t.Fatalf("registration error = %v, want %q", err, tc.want)
		}
	}
	l, _ := New("users", batch, 8)
	if _, err := Load(context.Background(), l, "one"); err == nil {
		t.Fatal("accepted missing scope")
	}
	if err := Dispatch(context.Background()); err == nil {
		t.Fatal("accepted missing scope")
	}
	if _, err := NewScope(context.Background(), []*Loader{nil}); err == nil {
		t.Fatal("accepted nil registration")
	}
	ctx, _ := NewScope(context.Background(), nil)
	if _, err := Load(ctx, l, "one"); err == nil {
		t.Fatal("accepted unregistered loader")
	}
}

func TestNilLookupArgumentsReturnScopeErrors(t *testing.T) {
	_, loadErr := Load(context.Background(), nil, "one")
	_, readErr := Read(nil)
	for _, tc := range []struct {
		err  error
		want string
	}{
		{loadErr, `loader scope "": loader must not be nil`},
		{readErr, `loader scope "": pending lookup must not be nil`},
	} {
		var scopeErr *ScopeError
		if !errors.As(tc.err, &scopeErr) || tc.err.Error() != tc.want {
			t.Fatalf("scope error = %v, want %q", tc.err, tc.want)
		}
	}
}

func TestBatchCallbacksCannotEnqueueOrDispatch(t *testing.T) {
	var l *Loader
	var err error
	l, err = New("users", func(ctx context.Context, keys []Key) ([]Result, error) {
		_, loadErr := Load(ctx, l, "nested")
		var scopeErr *ScopeError
		if !errors.As(loadErr, &scopeErr) || loadErr.Error() != `loader scope "users": batch callbacks must not enqueue lookups` {
			t.Errorf("nested lookup error = %v", loadErr)
		}
		dispatchErr := Dispatch(ctx)
		if !errors.As(dispatchErr, &scopeErr) || dispatchErr.Error() != `loader scope "": dispatch is already running` {
			t.Errorf("nested dispatch error = %v", dispatchErr)
		}
		return []Result{{Value: string(keys[0])}}, nil
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := NewScope(context.Background(), []*Loader{l})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []Key{"one", "two"} {
		pending, err := Load(ctx, l, key)
		if err != nil {
			t.Fatal(err)
		}
		if err := Dispatch(ctx); err != nil {
			t.Fatal(err)
		}
		if value, err := Read(pending); err != nil || value != string(key) {
			t.Fatalf("outer lookup value = %v, error = %v", value, err)
		}
	}
}

func TestCancellationDuringBatchDiscardsResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l, err := New("users", func(context.Context, []Key) ([]Result, error) {
		cancel()
		return []Result{{Value: "stale result"}}, nil
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err = NewScope(ctx, []*Loader{l})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := Load(ctx, l, "one")
	if err != nil {
		t.Fatal(err)
	}
	if err := Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	if value, err := Read(pending); value != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled lookup value = %v, error = %v", value, err)
	}
}

func TestConcurrentEnqueue(t *testing.T) {
	var count int
	l, _ := New("users", func(_ context.Context, keys []Key) ([]Result, error) {
		count += len(keys)
		return []Result{{Value: "one"}}, nil
	}, 8)
	ctx, _ := NewScope(context.Background(), []*Loader{l})
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			if _, err := Load(ctx, l, "one"); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	if err := Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicate lookups %d", count)
	}
}
