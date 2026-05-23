package logger

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/plugin"
)

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
	if err := loggerPlugin.FieldResolveStart(ctx, plugin.FieldContext{FieldName: "user", Path: []string{"user"}}); err != nil {
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
