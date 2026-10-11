package exec

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/loader"
	"github.com/grx-gql/grx/plugins"
	"github.com/grx-gql/grx/schema"
)

func loaderSchema(t *testing.T, l *loader.Loader) *schema.Schema {
	t.Helper()
	s := roadmapAuditSchema(t)
	user := &schema.Object{TypeName: "BatchUser", Fields: map[string]*schema.Field{}}
	user.Fields["name"] = &schema.Field{Name: "name", Type: s.Types["String"], Resolver: func(ctx context.Context, p schema.ResolveParams) (any, error) {
		return loader.Load(ctx, l, p.Source.(loader.Key))
	}}
	user.Fields["id"] = &schema.Field{Name: "id", Type: s.Types["String"], Resolver: func(_ context.Context, p schema.ResolveParams) (any, error) {
		return string(p.Source.(loader.Key)), nil
	}}
	s.Types[user.Name()] = user
	s.Query.Fields["users"] = &schema.Field{Name: "users", Type: &schema.List{OfType: user}, Resolver: func(context.Context, schema.ResolveParams) (any, error) {
		return []loader.Key{"2", "1", "2"}, nil
	}}
	return s
}

func TestLoadersBatchSuccessiveObjectLookups(t *testing.T) {
	var names [][]loader.Key
	namesLoader, _ := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		names = append(names, append([]loader.Key(nil), keys...))
		results := make([]loader.Result, len(keys))
		for i, key := range keys {
			results[i].Value = string(key)
		}
		return results, nil
	}, 32)
	var users [][]loader.Key
	usersLoader, _ := loader.New("users", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		users = append(users, append([]loader.Key(nil), keys...))
		results := make([]loader.Result, len(keys))
		for i, key := range keys {
			results[i].Value = key
		}
		return results, nil
	}, 32)
	s := loaderSchema(t, namesLoader)
	user := s.Types["BatchUser"].(*schema.Object)
	user.Fields["friend"] = &schema.Field{Name: "friend", Type: user, Resolver: func(ctx context.Context, p schema.ResolveParams) (any, error) {
		return loader.Load(ctx, usersLoader, p.Source.(loader.Key))
	}}
	r := New(s, nil, WithLoaders([]*loader.Loader{namesLoader, usersLoader})).Execute(context.Background(), core.Request{Query: `{ users { friend { name } } }`})
	if len(r.Errors) != 0 || roadmapAuditJSON(t, r.Data) != `{"users":[{"friend":{"name":"2"}},{"friend":{"name":"1"}},{"friend":{"name":"2"}}]}` {
		t.Fatal(r)
	}
	if !reflect.DeepEqual(users, [][]loader.Key{{"2", "1"}}) || !reflect.DeepEqual(names, users) {
		t.Fatalf("users=%v names=%v", users, names)
	}
}

func TestLoadersConcurrentRootsShareCache(t *testing.T) {
	var batchCalls, resolverCalls atomic.Int32
	l, _ := loader.New("root", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		batchCalls.Add(1)
		return []loader.Result{{Value: "ok"}}, nil
	}, 8)
	s := roadmapAuditSchema(t)
	s.Query.Fields["ok"].Resolver = func(ctx context.Context, _ schema.ResolveParams) (any, error) {
		resolverCalls.Add(1)
		return loader.Load(ctx, l, "same")
	}
	e := New(s, nil, WithLoaders([]*loader.Loader{l}), WithQueryConcurrency(2), WithResolverCache(), WithApolloTracing())
	r := e.Execute(context.Background(), core.Request{Query: `{ a:ok b:ok c:ok }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"a":"ok","b":"ok","c":"ok"}` || len(r.Errors) != 0 {
		t.Fatal(got, r.Errors)
	}
	if resolverCalls.Load() != 1 || batchCalls.Load() != 1 || r.Extensions["tracing"] == nil {
		t.Fatalf("resolvers=%d batches=%d response=%+v", resolverCalls.Load(), batchCalls.Load(), r)
	}
}

func TestLoadersResolverCacheSurvivesParallelDeferredPatch(t *testing.T) {
	l, _ := loader.New("unused", func(context.Context, []loader.Key) ([]loader.Result, error) { return nil, nil }, 1)
	s := roadmapAuditSchema(t)
	var calls atomic.Int32
	s.Query.Fields["ok"].Resolver = func(context.Context, schema.ResolveParams) (any, error) { return strconv.Itoa(int(calls.Add(1))), nil }
	e := New(s, nil, WithLoaders([]*loader.Loader{l}), WithResolverCache(), WithQueryConcurrency(2))
	initial, patches := e.ExecuteIncremental(context.Background(), core.Request{Query: `{ a:ok ... @defer { b:ok c:ok } }`})
	if len(initial.Errors) != 0 || len(patches) != 1 {
		t.Fatalf("%+v %+v", initial, patches)
	}
	if got := roadmapAuditJSON(t, patches[0].Data); got != `{"b":"1","c":"1"}` || calls.Load() != 1 {
		t.Fatalf("%s calls=%d", got, calls.Load())
	}
}

func TestLoadersConcurrentRootErrorsAreAllReported(t *testing.T) {
	l, _ := loader.New("unused", func(context.Context, []loader.Key) ([]loader.Result, error) { return nil, nil }, 1)
	s := roadmapAuditSchema(t)
	s.Query.Fields["ok"].Type = &schema.NonNull{OfType: s.Types["String"]}
	s.Query.Fields["ok"].Resolver = func(context.Context, schema.ResolveParams) (any, error) { return nil, errors.New("failed") }
	r := New(s, nil, WithLoaders([]*loader.Loader{l}), WithQueryConcurrency(2)).Execute(context.Background(), core.Request{Query: `{ a:ok b:ok }`})
	if !r.DataNull || len(r.Errors) != 2 || !reflect.DeepEqual(r.Errors[0].Path, []any{"a"}) || !reflect.DeepEqual(r.Errors[1].Path, []any{"b"}) {
		t.Fatalf("%+v", r)
	}
}

func TestLoadersRootConcurrencyBound(t *testing.T) {
	l, _ := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		results := make([]loader.Result, len(keys))
		for i := range keys {
			results[i].Value = "ok"
		}
		return results, nil
	}, 8)
	s := loaderSchema(t, l)
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	s.Query.Fields["users"].Resolver = func(ctx context.Context, _ schema.ResolveParams) (any, error) {
		started <- struct{}{}
		select {
		case <-release:
			return []loader.Key{"1"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan core.Response, 1)
	go func() {
		done <- New(s, nil, WithLoaders([]*loader.Loader{l}), WithQueryConcurrency(2)).Execute(ctx, core.Request{Query: `{ a:users { name } b:users { name } c:users { name } }`})
	}()
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("root resolvers did not run concurrently")
		}
	}
	select {
	case <-started:
		t.Fatal("exceeded root concurrency bound")
	default:
	}
	close(release)
	select {
	case r := <-done:
		if len(r.Errors) != 0 {
			t.Fatal(r.Errors)
		}
	case <-ctx.Done():
		t.Fatal("execution did not finish")
	}
}

func TestLoadersStreamInitialBatchAndCancellation(t *testing.T) {
	var batches [][]loader.Key
	l, _ := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		batches = append(batches, append([]loader.Key(nil), keys...))
		results := make([]loader.Result, len(keys))
		for i := range keys {
			results[i].Value = "ok"
		}
		return results, nil
	}, 8)
	e := New(loaderSchema(t, l), nil, WithLoaders([]*loader.Loader{l}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := e.ExecuteIncrementalStream(ctx, core.Request{Query: `{ users @stream(initialCount: 2) { name } }`}, func(r core.Response) error {
		if len(r.Errors) != 0 || !reflect.DeepEqual(batches, [][]loader.Key{{"2", "1"}}) {
			t.Fatalf("initial: %+v batches=%v", r, batches)
		}
		cancel()
		return nil
	}, func(core.IncrementalPayload, bool) error { t.Fatal("patch emitted after cancellation"); return nil })
	if !errors.Is(err, context.Canceled) || len(batches) != 1 {
		t.Fatalf("%v %v", batches, err)
	}
}

func TestLoadersAuthorizationPreventsLookup(t *testing.T) {
	l, _ := loader.New("names", func(context.Context, []loader.Key) ([]loader.Result, error) {
		t.Fatal("unauthorized lookup")
		return nil, nil
	}, 8)
	e := New(loaderSchema(t, l), nil, WithLoaders([]*loader.Loader{l}), WithFieldAuthorizer(func(_ context.Context, field FieldAuthorizationContext) error {
		if field.FieldName == "name" {
			return errors.New("denied")
		}
		return nil
	}))
	r := e.Execute(context.Background(), core.Request{Query: `{ users { name } }`})
	if len(r.Errors) != 3 {
		t.Fatal(r.Errors)
	}
}

func TestLoadersResolveAbstractTypesOnce(t *testing.T) {
	l, _ := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		results := make([]loader.Result, len(keys))
		for i, key := range keys {
			results[i].Value = string(key)
		}
		return results, nil
	}, 8)
	s := loaderSchema(t, l)
	union := &schema.Union{TypeName: "UserResult", Types: []*schema.Object{s.Types["BatchUser"].(*schema.Object)}}
	s.Types[union.Name()] = union
	s.Query.Fields["users"].Type = &schema.List{OfType: union}
	var calls int
	e := New(s, nil, WithLoaders([]*loader.Loader{l}), WithAbstractTypeResolver(func(any) (string, error) { calls++; return "BatchUser", nil }))
	r := e.Execute(context.Background(), core.Request{Query: `{ users { __typename ... on BatchUser { name } } }`})
	if len(r.Errors) != 0 || calls != 3 {
		t.Fatalf("calls=%d errors=%v", calls, r.Errors)
	}
	if got := roadmapAuditJSON(t, r.Data); got != `{"users":[{"__typename":"BatchUser","name":"2"},{"__typename":"BatchUser","name":"1"},{"__typename":"BatchUser","name":"2"}]}` {
		t.Fatal(got)
	}
}

func TestLoadersSubscriptionEventsHaveFreshCaches(t *testing.T) {
	var calls int
	l, _ := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		calls++
		return []loader.Result{{Value: strconv.Itoa(calls)}}, nil
	}, 8)
	s := loaderSchema(t, l)
	events := make(chan loader.Key, 2)
	events <- "1"
	events <- "1"
	close(events)
	s.Subscription = &schema.Object{TypeName: "Subscription", Fields: map[string]*schema.Field{"user": {Name: "user", Type: s.Types["BatchUser"], Resolver: func(context.Context, schema.ResolveParams) (any, error) { return events, nil }}}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := New(s, nil, WithLoaders([]*loader.Loader{l})).Subscribe(ctx, core.Request{Query: `subscription { user { name } }`})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for response := range out {
		if len(response.Errors) != 0 {
			t.Fatal(response.Errors)
		}
		got = append(got, roadmapAuditJSON(t, response.Data))
	}
	if !reflect.DeepEqual(got, []string{`{"user":{"name":"1"}}`, `{"user":{"name":"2"}}`}) {
		t.Fatal(got)
	}
}

type loaderObserver struct {
	plugins.Base
	mu     sync.Mutex
	starts map[string]int
	ends   map[string]int
}

func (p *loaderObserver) FieldResolveStart(_ context.Context, field plugins.FieldContext) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.starts[fmt.Sprint(field.Path)]++
	return nil
}

func (p *loaderObserver) FieldResolveEnd(_ context.Context, field plugins.FieldContext, _ error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ends[fmt.Sprint(field.Path)]++
}

func TestLoadersBalanceHooksAfterNullPropagation(t *testing.T) {
	l, _ := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		results := make([]loader.Result, len(keys))
		for i := range keys {
			results[i].Err = errors.New("missing")
		}
		return results, nil
	}, 8)
	s := loaderSchema(t, l)
	s.Types["BatchUser"].(*schema.Object).Fields["name"].Type = &schema.NonNull{OfType: s.Types["String"]}
	observer := &loaderObserver{starts: map[string]int{}, ends: map[string]int{}}
	r := New(s, []plugins.Plugin{observer}, WithLoaders([]*loader.Loader{l}), WithApolloTracing()).Execute(context.Background(), core.Request{Query: `{ users { name id } }`})
	if len(r.Errors) != 3 {
		t.Fatal(r.Errors)
	}
	if !reflect.DeepEqual(observer.starts, observer.ends) {
		t.Fatalf("starts=%v ends=%v", observer.starts, observer.ends)
	}
	for key, count := range observer.starts {
		if count != 1 {
			t.Fatalf("duplicate hook %s: %d", key, count)
		}
	}
}

func TestLoadersBatchListFieldsAndIsolateExecutions(t *testing.T) {
	var batches [][]loader.Key
	l, err := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		batches = append(batches, append([]loader.Key(nil), keys...))
		results := make([]loader.Result, len(keys))
		for i, key := range keys {
			results[i].Value = "user" + string(key)
		}
		return results, nil
	}, 32)
	if err != nil {
		t.Fatal(err)
	}
	e := New(loaderSchema(t, l), nil, WithLoaders([]*loader.Loader{l}), WithResolverCache())
	for range 2 {
		r := e.Execute(context.Background(), core.Request{Query: `{ users { name alias:name id } }`})
		want := `{"users":[{"name":"user2","alias":"user2","id":"2"},{"name":"user1","alias":"user1","id":"1"},{"name":"user2","alias":"user2","id":"2"}]}`
		if got := roadmapAuditJSON(t, r.Data); got != want || len(r.Errors) != 0 {
			t.Fatalf("%s %+v", got, r.Errors)
		}
	}
	if !reflect.DeepEqual(batches, [][]loader.Key{{"2", "1"}, {"2", "1"}}) {
		t.Fatalf("batches %v", batches)
	}
}

func TestLoaderErrorsPreservePathsAndNullability(t *testing.T) {
	l, _ := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		results := make([]loader.Result, len(keys))
		for i, key := range keys {
			if key == "1" {
				results[i].Err = errors.New("private missing user")
			} else {
				results[i].Value = "ok"
			}
		}
		return results, nil
	}, 32)
	s := loaderSchema(t, l)
	user := s.Types["BatchUser"].(*schema.Object)
	user.Fields["name"].Type = &schema.NonNull{OfType: s.Types["String"]}
	r := New(s, nil, WithLoaders([]*loader.Loader{l}), WithClientErrorMasking("hidden")).Execute(context.Background(), core.Request{Query: `{ users { name } }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"users":[{"name":"ok"},null,{"name":"ok"}]}` {
		t.Fatal(got, r.Errors)
	}
	if len(r.Errors) != 1 || r.Errors[0].Message != "hidden" || !reflect.DeepEqual(r.Errors[0].Path, []any{"users", 1, "name"}) || len(r.Errors[0].Locations) == 0 {
		t.Fatalf("%+v", r.Errors)
	}
}

func TestLoadersKeepMutationsSerialAndFresh(t *testing.T) {
	var version int
	l, _ := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		results := make([]loader.Result, len(keys))
		for i := range keys {
			results[i].Value = strconv.Itoa(version)
		}
		return results, nil
	}, 32)
	s := loaderSchema(t, l)
	s.Mutation = &schema.Object{TypeName: "Mutation", Fields: map[string]*schema.Field{"change": {Name: "change", Type: s.Types["BatchUser"], Resolver: func(context.Context, schema.ResolveParams) (any, error) { version++; return loader.Key("1"), nil }}}}
	r := New(s, nil, WithLoaders([]*loader.Loader{l}), WithQueryConcurrency(4)).Execute(context.Background(), core.Request{Query: `mutation { a:change { name } b:change { name } }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"a":{"name":"1"},"b":{"name":"2"}}` || len(r.Errors) != 0 {
		t.Fatal(got, r.Errors)
	}
}

func TestLoadersDoNotRunDeferredWorkBeforeInitialPayload(t *testing.T) {
	var keysSeen []loader.Key
	l, _ := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		keysSeen = append(keysSeen, keys...)
		results := make([]loader.Result, len(keys))
		for i := range keys {
			results[i].Value = "ok"
		}
		return results, nil
	}, 32)
	e := New(loaderSchema(t, l), nil, WithLoaders([]*loader.Loader{l}))
	var patches int
	err := e.ExecuteIncrementalStream(context.Background(), core.Request{Query: `{ users { id ... @defer { name } } }`}, func(r core.Response) error {
		if len(keysSeen) != 0 || len(r.Errors) != 0 {
			t.Fatalf("deferred work ran early: %v %+v", keysSeen, r)
		}
		return nil
	}, func(p core.IncrementalPayload, _ bool) error {
		patches++
		if got := roadmapAuditJSON(t, p.Data); got != `{"name":"ok"}` || len(p.Errors) != 0 {
			t.Fatal(got, p.Errors)
		}
		return nil
	})
	if err != nil || patches != 3 || !reflect.DeepEqual(keysSeen, []loader.Key{"2", "1"}) {
		t.Fatalf("patches=%d keys=%v err=%v", patches, keysSeen, err)
	}
}
