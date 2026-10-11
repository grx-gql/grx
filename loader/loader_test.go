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
	ctx, err := NewScope(context.Background(), []*Loader{l})
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
	}{
		{"cardinality", func(context.Context, []Key) ([]Result, error) { return nil, nil }},
		{"batch error", func(context.Context, []Key) ([]Result, error) { return nil, lookupErr }},
		{"panic", func(context.Context, []Key) ([]Result, error) { panic("broken adapter") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, _ := New("users", tc.batch, 8)
			ctx, _ := NewScope(context.Background(), []*Loader{l})
			p, _ := Load(ctx, l, "one")
			if err := Dispatch(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := Read(p); err == nil {
				t.Fatal("batch failure was lost")
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
	}{{"", batch, 1}, {"x", nil, 1}, {"x", batch, 0}} {
		if _, err := New(tc.name, tc.batch, tc.size); err == nil {
			t.Fatalf("accepted invalid registration %+v", tc)
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
