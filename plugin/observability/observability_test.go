package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/plugin"
	"github.com/grx-gql/grx/plugin/observability"
	"github.com/grx-gql/grx/schema"
)

type obsUser struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name,nonNull"`
}

type obsQuery struct{}

type obsArgs struct {
	ID string `gql:"id,nonNull"`
}

func (obsQuery) User(ctx context.Context, args obsArgs) (*obsUser, error) {
	return &obsUser{ID: args.ID, Name: "Ada"}, nil
}

func obsExecutor(t *testing.T, plugins []plugin.Plugin) *exec.Executor {
	t.Helper()
	s, err := schema.Build(schema.Config{Query: obsQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return exec.New(s, plugins)
}

// recordingTracer captures span lifecycle events for assertions.
type recordingTracer struct {
	mu         sync.Mutex
	operations []string
	fields     []string
	ended      int
}

type recordingSpan struct {
	tracer *recordingTracer
}

func (s *recordingSpan) End(error) {
	s.tracer.mu.Lock()
	s.tracer.ended++
	s.tracer.mu.Unlock()
}

func (tr *recordingTracer) StartOperation(ctx context.Context, name string) (context.Context, observability.Span) {
	tr.mu.Lock()
	tr.operations = append(tr.operations, name)
	tr.mu.Unlock()
	return ctx, &recordingSpan{tracer: tr}
}

func (tr *recordingTracer) StartField(ctx context.Context, f plugin.FieldContext) observability.Span {
	tr.mu.Lock()
	tr.fields = append(tr.fields, f.ParentType+"."+f.FieldName)
	tr.mu.Unlock()
	return &recordingSpan{tracer: tr}
}

func TestTracingPluginEmitsOperationAndFieldSpans(t *testing.T) {
	tr := &recordingTracer{}
	e := obsExecutor(t, []plugin.Plugin{observability.NewTracingPlugin(tr)})

	resp := e.Execute(context.Background(), core.Request{Query: `query Q { user(id: "1") { id name } }`, OperationName: "Q"})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.operations) != 1 || tr.operations[0] != "Q" {
		t.Fatalf("operations = %v", tr.operations)
	}
	// user, user.id, user.name
	if len(tr.fields) != 3 {
		t.Fatalf("expected 3 field spans, got %v", tr.fields)
	}
	if tr.ended != 1+3 {
		t.Fatalf("expected all 4 spans ended, got %d", tr.ended)
	}
}

func TestMetricsPluginObservesOperation(t *testing.T) {
	var got observability.OperationMetrics
	rec := observability.MetricsRecorderFunc(func(_ context.Context, m observability.OperationMetrics) { got = m })
	e := obsExecutor(t, []plugin.Plugin{observability.NewMetricsPlugin(rec)})

	resp := e.Execute(context.Background(), core.Request{Query: `query Q { user(id: "1") { id } }`, OperationName: "Q"})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	if got.OperationName != "Q" {
		t.Fatalf("operation = %q", got.OperationName)
	}
	if got.HadErrors {
		t.Fatalf("expected no errors recorded")
	}
	if got.Duration <= 0 {
		t.Fatalf("expected positive duration, got %v", got.Duration)
	}
}

func TestAccessLogPluginEmitsRecord(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	e := obsExecutor(t, []plugin.Plugin{observability.NewAccessLogPlugin(logger)})

	resp := e.Execute(context.Background(), core.Request{Query: `query Q { user(id: "1") { id } }`, OperationName: "Q"})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}

	line := buf.String()
	if !strings.Contains(line, "graphql.operation") {
		t.Fatalf("expected access log line, got %q", line)
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &entry); err != nil {
		t.Fatalf("log not valid JSON: %v (%q)", err, line)
	}
	if entry["operation"] != "Q" {
		t.Fatalf("operation = %v", entry["operation"])
	}
}

func TestObservabilityPluginsHandleMissingState(t *testing.T) {
	tr := &recordingTracer{}
	tracing := observability.NewTracingPlugin(tr)
	field := plugin.FieldContext{Path: []string{"missing"}, FieldName: "missing"}
	if err := tracing.FieldResolveStart(context.Background(), field); err != nil {
		t.Fatalf("field start: %v", err)
	}
	tracing.FieldResolveEnd(context.Background(), field, nil)
	tracing.Error(context.Background(), context.Canceled)
	if err := tracing.ResponseSend(context.Background(), core.Response{}); err != nil {
		t.Fatalf("tracing response: %v", err)
	}

	metrics := observability.NewMetricsPlugin(observability.MetricsRecorderFunc(func(context.Context, observability.OperationMetrics) {
		t.Fatal("recorder should not be called without state")
	}))
	if err := metrics.ResponseSend(context.Background(), core.Response{}); err != nil {
		t.Fatalf("metrics response: %v", err)
	}

	var buf bytes.Buffer
	access := observability.NewAccessLogPlugin(slog.New(slog.NewJSONHandler(&buf, nil)))
	if err := access.ResponseSend(context.Background(), core.Response{}); err != nil {
		t.Fatalf("access response: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("unexpected log: %s", buf.String())
	}
}

func TestObservabilityErrorAndDefaultLoggerBranches(t *testing.T) {
	tr := &recordingTracer{}
	tracing := observability.NewTracingPlugin(tr)
	ctx, err := tracing.RequestStart(context.Background(), core.Request{OperationName: "Q"})
	if err != nil {
		t.Fatalf("request start: %v", err)
	}
	tracing.Error(ctx, context.DeadlineExceeded)
	if err := tracing.ResponseSend(ctx, core.Response{}); err != nil {
		t.Fatalf("response send: %v", err)
	}

	var got observability.OperationMetrics
	metrics := observability.NewMetricsPlugin(observability.MetricsRecorderFunc(func(_ context.Context, m observability.OperationMetrics) {
		got = m
	}))
	ctx, err = metrics.RequestStart(context.Background(), core.Request{OperationName: "Bad"})
	if err != nil {
		t.Fatalf("metrics start: %v", err)
	}
	if err := metrics.ResponseSend(ctx, core.Response{Errors: []core.Error{{Message: "boom"}}}); err != nil {
		t.Fatalf("metrics response: %v", err)
	}
	if !got.HadErrors || got.ErrorCount != 1 {
		t.Fatalf("metrics = %#v", got)
	}

	defaultAccess := observability.NewAccessLogPlugin(nil)
	ctx, err = defaultAccess.RequestStart(context.Background(), core.Request{})
	if err != nil {
		t.Fatalf("access start: %v", err)
	}
	if err := defaultAccess.ResponseSend(ctx, core.Response{Errors: []core.Error{{Message: "boom"}}}); err != nil {
		t.Fatalf("access response: %v", err)
	}
}
