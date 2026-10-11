package exec

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/plugins"
	"github.com/grx-gql/grx/schema"
)

func TestQueryConcurrencyBoundAndResponseOrder(t *testing.T) {
	s := roadmapAuditSchema(t)
	started := make(chan int, 4)
	releases := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})}
	for index, name := range []string{"a", "b", "c", "d"} {
		s.Query.Fields[name] = &schema.Field{Name: name, Type: s.Types["Int"], Resolver: func(ctx context.Context, _ schema.ResolveParams) (any, error) {
			started <- index
			select {
			case <-releases[index]:
				return index, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan core.Response, 1)
	go func() { done <- New(s, nil, WithQueryConcurrency(2)).Execute(ctx, core.Request{Query: `{ a b c d }`}) }()
	waitStarted := func() int {
		select {
		case index := <-started:
			return index
		case <-ctx.Done():
			t.Fatal("query worker did not start")
			return 0
		}
	}
	first := waitStarted()
	second := waitStarted()
	select {
	case extra := <-started:
		t.Fatalf("exceeded two workers: %d", extra)
	case <-time.After(10 * time.Millisecond):
	}
	close(releases[second])
	third := waitStarted()
	close(releases[third])
	fourth := waitStarted()
	close(releases[fourth])
	close(releases[first])
	r := <-done
	if got := roadmapAuditJSON(t, r.Data); got != `{"a":0,"b":1,"c":2,"d":3}` || len(r.Errors) != 0 {
		t.Fatalf("unordered response %s %+v", got, r.Errors)
	}
}

func TestQueryConcurrencySerialMutationAndDisabledOptions(t *testing.T) {
	for _, limit := range []int{-1, 0, 1, 4} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			s := roadmapAuditSchema(t)
			var count atomic.Int32
			s.Query.Fields["next"] = &schema.Field{Name: "next", Type: s.Types["Int"], Resolver: func(context.Context, schema.ResolveParams) (any, error) { return int(count.Add(1)), nil }}
			s.Mutation = s.Query
			query := `mutation { a:next b:next c:next }`
			if limit <= 1 {
				query = `{ a:next b:next c:next }`
			}
			r := New(s, nil, WithQueryConcurrency(limit)).Execute(context.Background(), core.Request{Query: query})
			if got := roadmapAuditJSON(t, r.Data); got != `{"a":1,"b":2,"c":3}` {
				t.Fatalf("serial execution changed: %s %+v", got, r.Errors)
			}
		})
	}
}

func TestQueryConcurrencyCacheAndTracing(t *testing.T) {
	s := roadmapAuditSchema(t)
	var calls atomic.Int32
	s.Query.Fields["ok"].Resolver = func(context.Context, schema.ResolveParams) (any, error) {
		calls.Add(1)
		time.Sleep(time.Millisecond)
		return "ok", nil
	}
	r := New(s, nil, WithQueryConcurrency(4), WithResolverCache(), WithApolloTracing()).Execute(context.Background(), core.Request{Query: `{ a:ok b:ok c:ok d:ok }`})
	if calls.Load() != 1 || len(r.Errors) != 0 || r.Extensions["tracing"] == nil {
		t.Fatalf("cache/tracing failed calls=%d response=%+v", calls.Load(), r)
	}
	if got := roadmapAuditJSON(t, r.Data); got != `{"a":"ok","b":"ok","c":"ok","d":"ok"}` {
		t.Fatal(got)
	}
}

func TestQueryConcurrencyCancellation(t *testing.T) {
	s := roadmapAuditSchema(t)
	started := make(chan struct{}, 2)
	var calls atomic.Int32
	s.Query.Fields["ok"].Resolver = func(ctx context.Context, _ schema.ResolveParams) (any, error) {
		calls.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan core.Response, 1)
	go func() {
		done <- New(s, nil, WithQueryConcurrency(2)).Execute(ctx, core.Request{Query: `{ a:ok b:ok c:ok d:ok }`})
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("query siblings did not start")
		}
	}
	cancel()
	select {
	case r := <-done:
		if len(r.Errors) == 0 || calls.Load() != 2 {
			t.Fatalf("cancellation failed calls=%d response=%+v", calls.Load(), r)
		}
	case <-time.After(time.Second):
		t.Fatal("workers did not finish after cancellation")
	}
}

func TestQueryConcurrencyNullPropagationAndErrorOrder(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["fail"] = &schema.Field{Name: "fail", Type: &schema.NonNull{OfType: s.Types["String"]}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return nil, errors.New("failure") }}
	r := New(s, nil, WithQueryConcurrency(3)).Execute(context.Background(), core.Request{Query: `{ a:fail ok b:fail }`})
	if !r.DataNull || r.Data != nil || len(r.Errors) != 2 || r.Errors[0].Path[0] != "a" || r.Errors[1].Path[0] != "b" {
		t.Fatalf("null/error order changed: %+v", r)
	}
}

func TestQueryConcurrencySingleField(t *testing.T) {
	r := New(roadmapAuditSchema(t), nil, WithQueryConcurrency(8)).Execute(context.Background(), core.Request{Query: `{ ok }`})
	if len(r.Errors) != 0 || roadmapAuditJSON(t, r.Data) != `{"ok":"ok"}` {
		t.Fatalf("%+v", r)
	}
}

func TestQueryConcurrencyCacheAllowsRecursiveCompletion(t *testing.T) {
	s := roadmapAuditSchema(t)
	object := &schema.Object{TypeName: "Recursive", Fields: map[string]*schema.Field{}}
	source := &struct{}{}
	var calls atomic.Int32
	object.Fields["next"] = &schema.Field{Name: "next", Type: object, Resolver: func(context.Context, schema.ResolveParams) (any, error) { calls.Add(1); return source, nil }}
	object.Fields["ok"] = s.Query.Fields["ok"]
	s.Types[object.Name()] = object
	s.Query.Fields["node"] = &schema.Field{Name: "node", Type: object, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return source, nil }}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r := New(s, nil, WithQueryConcurrency(2), WithResolverCache()).Execute(ctx, core.Request{Query: `{ a:node { next { next { ok } } } b:node { next { ok } } }`})
	if len(r.Errors) != 0 || calls.Load() != 1 {
		t.Fatalf("recursive completion cache calls=%d response=%+v", calls.Load(), r)
	}
	want := `{"a":{"next":{"next":{"ok":"ok"}}},"b":{"next":{"ok":"ok"}}}`
	if got := roadmapAuditJSON(t, r.Data); got != want {
		t.Fatal(got)
	}
}

type concurrentPanicPlugin struct{ plugins.Base }

func (concurrentPanicPlugin) FieldResolveStart(_ context.Context, field plugins.FieldContext) error {
	if field.FieldName == "broken" {
		panic("private hook panic")
	}
	return nil
}
func TestQueryConcurrencyRecoversWorkerPanics(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["broken"] = &schema.Field{Name: "broken", Type: s.Types["String"]}
	r := New(s, []plugins.Plugin{concurrentPanicPlugin{}}, WithQueryConcurrency(2), WithClientErrorMasking("hidden")).Execute(context.Background(), core.Request{Query: `{ broken ok }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"broken":null,"ok":"ok"}` || len(r.Errors) != 1 || r.Errors[0].Message != "hidden" {
		t.Fatalf("panic response %s %+v", got, r.Errors)
	}
}

func TestParallelResolverCacheCanceledWaiter(t *testing.T) {
	cache := &parallelResolverCache{entries: map[string]*parallelResolverEntry{}}
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = cache.resolve(context.Background(), "same", func() (any, error) { close(started); <-release; return "ok", nil })
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := cache.resolve(ctx, "same", func() (any, error) { t.Error("duplicate resolver"); return nil, nil })
	close(release)
	<-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter error %v", err)
	}
}
