package logger

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/plugins"
)

func TestGraphqlResponseTimeAttrBranches(t *testing.T) {
	sec := graphqlResponseTimeAttr(2 * time.Second).Value.String()
	if sec != `2.000s` {
		t.Fatalf("seconds branch got %q", sec)
	}
	ms := graphqlResponseTimeAttr(3 * time.Millisecond).Value.String()
	if ms != `3.000ms` {
		t.Fatalf("ms branch got %q", ms)
	}
	z := graphqlResponseTimeAttr(0).Value.String()
	if z != `0ms` {
		t.Fatalf("zero branch got %q", z)
	}
	neg := graphqlResponseTimeAttr(-time.Millisecond).Value.String()
	if neg != `0ms` {
		t.Fatalf("negative branch got %q", neg)
	}
}

func TestRequestElapsedEdgeCases(t *testing.T) {
	if _, ok := requestElapsed(context.Background()); ok {
		t.Fatal("missing start")
	}

	ctxWrong := context.WithValue(context.Background(), requestStartedAtCtxKey{}, "not-time")
	if _, ok := requestElapsed(ctxWrong); ok {
		t.Fatal("wrong Value type must be invisible")
	}

	startAt := time.Now().Add(-2 * time.Minute)
	ctxOK := context.WithValue(context.Background(), requestStartedAtCtxKey{}, startAt)
	d, ok := requestElapsed(ctxOK)
	if !ok || d <= 0 {
		t.Fatalf("expected elapsed, got dur=%v ok=%v", d, ok)
	}
}

func TestNewRequiresLogger(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for nil slog logger")
	}
}

func TestLoggerWritesLifecycleEvents(t *testing.T) {
	var output bytes.Buffer
	handler := slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})
	loggerPlugin, err := New(Config{Logger: slog.New(handler)})
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := context.Background()
	req := core.Request{
		Query:         `{ user { id } }`,
		OperationName: "GetUser",
		Variables:     map[string]any{"id": "1"},
	}

	ctx, err = loggerPlugin.RequestStart(ctx, req)
	if err != nil {
		t.Fatalf("request start: %v", err)
	}
	if err := loggerPlugin.ParsingStart(ctx, req); err != nil {
		t.Fatalf("parsing start: %v", err)
	}
	if err := loggerPlugin.ValidationStart(ctx, req); err != nil {
		t.Fatalf("validation start: %v", err)
	}
	if err := loggerPlugin.ExecutionStart(ctx, req); err != nil {
		t.Fatalf("execution start: %v", err)
	}
	if err := loggerPlugin.FieldResolveStart(ctx, plugins.FieldContext{FieldName: "user", Path: []string{"user"}}); err != nil {
		t.Fatalf("field resolve start: %v", err)
	}
	if err := loggerPlugin.ResponseSend(ctx, core.Response{Data: map[string]any{"user": "1"}}); err != nil {
		t.Fatalf("response send: %v", err)
	}
	loggerPlugin.Error(ctx, errors.New("resolver failed"))

	logs := output.String()
	expectedValues := []string{
		"graphql.request.start",
		"graphql.parsing.start",
		"graphql.validation.start",
		"graphql.execution.start",
		"graphql.field.resolve.start",
		"graphql.response.send",
		`"graphql.response.time":`,
		"graphql.error",
		"GetUser",
		"resolver failed",
	}
	for _, expected := range expectedValues {
		if !strings.Contains(logs, expected) {
			t.Fatalf("expected logs to contain %q, got %s", expected, logs)
		}
	}
}
