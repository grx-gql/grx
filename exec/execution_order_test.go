package exec

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

type orderMutation struct{}

func (orderMutation) First(ctx context.Context) (string, error) {
	orderMutationLog = append(orderMutationLog, "first")
	return "first", nil
}

func (orderMutation) Second(ctx context.Context) (string, error) {
	orderMutationLog = append(orderMutationLog, "second")
	return "second", nil
}

var orderMutationLog []string

func TestMutationRootFieldsExecuteSerially(t *testing.T) {
	orderMutationLog = nil

	schemaValue, err := schema.Build(schema.Config{
		Query:    introQuery{},
		Mutation: orderMutation{},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `mutation {
			second
			first
		}`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}
	if len(orderMutationLog) != 2 || orderMutationLog[0] != "second" || orderMutationLog[1] != "first" {
		t.Fatalf("expected serial mutation order [second first], got %#v", orderMutationLog)
	}
}

type parallelQuery struct{}

var (
	parallelPeak    int32
	parallelCurrent int32
)

func (parallelQuery) A(ctx context.Context) (string, error) {
	n := atomic.AddInt32(&parallelCurrent, 1)
	for {
		old := atomic.LoadInt32(&parallelPeak)
		if n <= old || atomic.CompareAndSwapInt32(&parallelPeak, old, n) {
			break
		}
	}
	time.Sleep(40 * time.Millisecond)
	atomic.AddInt32(&parallelCurrent, -1)
	return "a", nil
}

func (parallelQuery) B(ctx context.Context) (string, error) {
	n := atomic.AddInt32(&parallelCurrent, 1)
	for {
		old := atomic.LoadInt32(&parallelPeak)
		if n <= old || atomic.CompareAndSwapInt32(&parallelPeak, old, n) {
			break
		}
	}
	time.Sleep(40 * time.Millisecond)
	atomic.AddInt32(&parallelCurrent, -1)
	return "b", nil
}

func TestQuerySiblingFieldsMayExecuteInParallel(t *testing.T) {
	atomic.StoreInt32(&parallelPeak, 0)
	atomic.StoreInt32(&parallelCurrent, 0)

	schemaValue, err := schema.Build(schema.Config{Query: parallelQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	start := time.Now()
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ a b }`,
	})
	elapsed := time.Since(start)

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}
	if atomic.LoadInt32(&parallelPeak) < 2 {
		t.Fatalf("expected parallel sibling execution (peak concurrency %d)", parallelPeak)
	}
	if elapsed >= 70*time.Millisecond {
		t.Fatalf("expected parallel query to finish faster than serial sleep, took %v", elapsed)
	}
}

type BubbleItem struct{}

type bubbleQuery struct{}

func (bubbleQuery) RequiredItem(ctx context.Context) (*BubbleItem, error) {
	return &BubbleItem{}, nil
}

func TestNonNullFieldErrorBubblesToParent(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: bubbleQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	itemType := schemaValue.Types["BubbleItem"]
	object, ok := itemType.(*schema.Object)
	if !ok {
		t.Fatalf("expected BubbleItem object, got %T", itemType)
	}
	object.Fields["required"] = &schema.Field{
		Name: "required",
		Type: &schema.NonNull{OfType: schemaValue.Types["String"]},
		Resolver: func(ctx context.Context, params schema.ResolveParams) (any, error) {
			return nil, errors.New("resolver failed")
		},
	}

	// Wrap the item object as a non-null return type on the root field.
	schemaValue.Query.Fields["requiredItem"].Type = &schema.NonNull{OfType: itemType}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ requiredItem { required } }`,
	})

	if len(response.Errors) == 0 {
		t.Fatal("expected field error")
	}
	data := responseObject(t, response.Data)
	if data["requiredItem"] != nil {
		t.Fatalf("expected non-null parent bubbled to null, got %#v", data["requiredItem"])
	}
}
