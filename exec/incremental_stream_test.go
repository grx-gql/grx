package exec

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/plugins"
	"github.com/grx-gql/grx/schema"
)

type incrementalResponseHook struct {
	plugins.Base
	responses int
	failure   error
	observed  error
}

func (h *incrementalResponseHook) ResponseSend(context.Context, core.Response) error {
	h.responses++
	return h.failure
}
func (h *incrementalResponseHook) Error(_ context.Context, err error) { h.observed = err }

func TestIncrementalStreamNestedWork(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: incrementalQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	hook := &incrementalResponseHook{}
	e := New(s, []plugins.Plugin{hook})
	initialCount := 0
	var labels []string
	var more []bool
	err = e.ExecuteIncrementalStream(context.Background(), core.Request{Query: `{ ... on Query @defer(label:"outer") { friends @stream(initialCount:0, label:"items") { ... on incrementalFriend @defer(label:"inner") { id } } } }`}, func(initial core.Response) error {
		initialCount++
		if len(initial.Errors) > 0 || initial.HasNext == nil || !*initial.HasNext {
			t.Fatalf("initial = %#v", initial)
		}
		return nil
	}, func(patch core.IncrementalPayload, hasNext bool) error {
		if initialCount != 1 {
			t.Fatal("patch emitted before initial response")
		}
		if len(patch.Errors) > 0 {
			t.Fatalf("patch errors = %v", patch.Errors)
		}
		labels = append(labels, patch.Label)
		more = append(more, hasNext)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"outer", "items", "items", "items", "inner", "inner", "inner"}; !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %v; want %v", labels, want)
	}
	if want := []bool{true, true, true, true, true, true, false}; !reflect.DeepEqual(more, want) {
		t.Fatalf("hasNext = %v", more)
	}
	if hook.responses != 1 {
		t.Fatalf("response hooks = %d", hook.responses)
	}
}

func TestIncrementalStreamShortAndNoPending(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: incrementalQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	e := New(s, nil)
	for _, query := range []string{`{ missing }`, `{ friends { id } }`} {
		t.Run(query, func(t *testing.T) {
			initialCount := 0
			err := e.ExecuteIncrementalStream(context.Background(), core.Request{Query: query}, func(response core.Response) error {
				initialCount++
				if response.HasNext != nil && *response.HasNext {
					t.Fatal("unexpected pending work")
				}
				if (len(response.Errors) > 0) != (query == `{ missing }`) {
					t.Fatalf("errors = %v", response.Errors)
				}
				return nil
			}, func(core.IncrementalPayload, bool) error { t.Fatal("unexpected patch"); return nil })
			if err != nil || initialCount != 1 {
				t.Fatalf("err=%v initialCount=%d", err, initialCount)
			}
		})
	}
	failure := errors.New("initial rejected")
	if err := e.ExecuteIncrementalStream(context.Background(), core.Request{Query: `{ missing }`}, func(core.Response) error { return failure }, func(core.IncrementalPayload, bool) error { return nil }); err != failure {
		t.Fatalf("short callback error=%v", err)
	}
}

func TestIncrementalStreamStopsPendingWork(t *testing.T) {
	for _, stage := range []string{"before", "initial error", "patch error", "initial cancel", "resolver cancel", "patch cancel"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			scalar := &schema.Scalar{TypeName: "String"}
			root := &schema.Object{TypeName: "Query", Fields: map[string]*schema.Field{}}
			for _, name := range []string{"one", "two"} {
				root.Fields[name] = &schema.Field{Name: name, Type: scalar, Resolver: func(context.Context, schema.ResolveParams) (any, error) {
					calls++
					if stage == "resolver cancel" {
						cancel()
					}
					return "ok", nil
				}}
			}
			e := New(&schema.Schema{Query: root, Types: map[string]schema.Type{"Query": root, "String": scalar}}, nil)
			failure := errors.New("emitter stopped")
			patches := 0
			if stage == "before" {
				cancel()
			}
			err := e.ExecuteIncrementalStream(ctx, core.Request{Query: `{ ... on Query @defer { one } ... on Query @defer { two } }`}, func(core.Response) error {
				if stage == "initial error" {
					return failure
				}
				if stage == "initial cancel" {
					cancel()
				}
				return nil
			}, func(core.IncrementalPayload, bool) error {
				patches++
				if stage == "patch error" {
					return failure
				}
				if stage == "patch cancel" {
					cancel()
				}
				return nil
			})
			want := context.Canceled
			if stage == "initial error" || stage == "patch error" {
				want = failure
			}
			if err != want {
				t.Fatalf("error = %v; want %v", err, want)
			}
			wantCalls := 0
			if stage == "patch error" || stage == "patch cancel" || stage == "resolver cancel" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("resolver calls = %d; want %d", calls, wantCalls)
			}
			if stage == "resolver cancel" && patches != 0 {
				t.Fatal("emitted patch after resolver canceled")
			}
		})
	}
}

func TestIncrementalStreamResponseHookFailureStopsWork(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: incrementalQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("response hook failed")
	hook := &incrementalResponseHook{failure: failure}
	e := New(s, []plugins.Plugin{hook})
	err = e.ExecuteIncrementalStream(context.Background(), core.Request{Query: `{friends @stream(initialCount:0){id}}`}, func(initial core.Response) error {
		if len(initial.Errors) != 1 {
			t.Fatalf("initial errors=%v", initial.Errors)
		}
		return nil
	}, func(core.IncrementalPayload, bool) error { t.Fatal("patch after hook failure"); return nil })
	if err != nil || hook.responses != 1 || hook.observed != failure {
		t.Fatalf("err=%v hook=%#v", err, hook)
	}
}

func TestBufferedIncrementalReportsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	initial, patches := (&Executor{}).ExecuteIncremental(ctx, core.Request{})
	if len(initial.Errors) != 1 || len(patches) != 0 || initial.HasNext == nil || *initial.HasNext {
		t.Fatalf("initial=%#v patches=%#v", initial, patches)
	}
}
