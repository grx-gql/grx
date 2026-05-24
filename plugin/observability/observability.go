// Package observability provides dependency-free GraphQL observability plugins:
// vendor-neutral tracing spans (suitable for an OpenTelemetry adapter),
// operation metrics (suitable for a Prometheus adapter), and structured access
// logs. None of these import a third-party telemetry SDK; callers wire their
// backend of choice behind the small interfaces defined here.
package observability

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/plugin"
)

// ---------------------------------------------------------------------------
// Tracing: vendor-neutral operation and field spans.
// ---------------------------------------------------------------------------

// Span represents an in-flight tracing span. End is called once, with the
// resolver/operation error if any.
type Span interface {
	End(err error)
}

// Tracer starts operation- and field-level spans. An OpenTelemetry adapter
// implements this interface by wrapping a trace.Tracer.
type Tracer interface {
	StartOperation(ctx context.Context, operationName string) (context.Context, Span)
	StartField(ctx context.Context, field plugin.FieldContext) Span
}

type tracingState struct {
	op     Span
	opErr  error
	mu     sync.Mutex
	fields map[string]Span
}

type tracingKey struct{}

// TracingPlugin emits one span per operation and one span per resolved field
// using the supplied Tracer. It satisfies both plugin.Plugin and
// plugin.FieldResolveEnder.
type TracingPlugin struct {
	plugin.Base
	tracer Tracer
}

// NewTracingPlugin returns a plugin that drives tracer for operation and field
// spans.
func NewTracingPlugin(tracer Tracer) *TracingPlugin {
	return &TracingPlugin{tracer: tracer}
}

// RequestStart begins the operation span and stashes per-request span state.
func (p *TracingPlugin) RequestStart(ctx context.Context, req core.Request) (context.Context, error) {
	spanCtx, span := p.tracer.StartOperation(ctx, req.OperationName)
	state := &tracingState{op: span, fields: make(map[string]Span)}
	return context.WithValue(spanCtx, tracingKey{}, state), nil
}

// FieldResolveStart opens a field span keyed by response path.
func (p *TracingPlugin) FieldResolveStart(ctx context.Context, field plugin.FieldContext) error {
	state, _ := ctx.Value(tracingKey{}).(*tracingState)
	if state == nil {
		return nil
	}
	span := p.tracer.StartField(ctx, field)
	state.mu.Lock()
	state.fields[strings.Join(field.Path, ".")] = span
	state.mu.Unlock()
	return nil
}

// FieldResolveEnd closes the matching field span.
func (p *TracingPlugin) FieldResolveEnd(ctx context.Context, field plugin.FieldContext, err error) {
	state, _ := ctx.Value(tracingKey{}).(*tracingState)
	if state == nil {
		return
	}
	key := strings.Join(field.Path, ".")
	state.mu.Lock()
	span := state.fields[key]
	delete(state.fields, key)
	state.mu.Unlock()
	if span != nil {
		span.End(err)
	}
}

// Error records the operation-level error used when the operation span ends.
func (p *TracingPlugin) Error(ctx context.Context, err error) {
	if state, _ := ctx.Value(tracingKey{}).(*tracingState); state != nil {
		state.mu.Lock()
		state.opErr = err
		state.mu.Unlock()
	}
}

// ResponseSend ends the operation span.
func (p *TracingPlugin) ResponseSend(ctx context.Context, _ core.Response) error {
	if state, _ := ctx.Value(tracingKey{}).(*tracingState); state != nil && state.op != nil {
		state.op.End(state.opErr)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Metrics: operation count, latency, and error rate.
// ---------------------------------------------------------------------------

// OperationMetrics summarizes one completed operation. A Prometheus adapter
// translates these into counters and histograms.
type OperationMetrics struct {
	OperationName string
	Duration      time.Duration
	ErrorCount    int
	HadErrors     bool
}

// MetricsRecorder receives one OperationMetrics per request.
type MetricsRecorder interface {
	ObserveOperation(ctx context.Context, m OperationMetrics)
}

// MetricsRecorderFunc adapts a function to MetricsRecorder.
type MetricsRecorderFunc func(ctx context.Context, m OperationMetrics)

// ObserveOperation calls f.
func (f MetricsRecorderFunc) ObserveOperation(ctx context.Context, m OperationMetrics) { f(ctx, m) }

type metricsState struct {
	start         time.Time
	operationName string
}

type metricsKey struct{}

// MetricsPlugin records per-operation latency and error counts via a
// MetricsRecorder.
type MetricsPlugin struct {
	plugin.Base
	recorder MetricsRecorder
}

// NewMetricsPlugin returns a plugin that reports operation metrics to recorder.
func NewMetricsPlugin(recorder MetricsRecorder) *MetricsPlugin {
	return &MetricsPlugin{recorder: recorder}
}

// RequestStart stamps the operation start time.
func (p *MetricsPlugin) RequestStart(ctx context.Context, req core.Request) (context.Context, error) {
	return context.WithValue(ctx, metricsKey{}, &metricsState{start: time.Now(), operationName: req.OperationName}), nil
}

// ResponseSend reports the completed operation's metrics.
func (p *MetricsPlugin) ResponseSend(ctx context.Context, res core.Response) error {
	state, _ := ctx.Value(metricsKey{}).(*metricsState)
	if state == nil {
		return nil
	}
	p.recorder.ObserveOperation(ctx, OperationMetrics{
		OperationName: state.operationName,
		Duration:      time.Since(state.start),
		ErrorCount:    len(res.Errors),
		HadErrors:     len(res.Errors) > 0,
	})
	return nil
}

// ---------------------------------------------------------------------------
// Access logs: one structured line per operation.
// ---------------------------------------------------------------------------

type accessLogState struct {
	start         time.Time
	operationName string
}

type accessLogKey struct{}

// AccessLogPlugin emits a structured slog record per completed operation.
type AccessLogPlugin struct {
	plugin.Base
	logger *slog.Logger
}

// NewAccessLogPlugin returns a plugin that logs operation access records to
// logger (defaulting to slog.Default when nil).
func NewAccessLogPlugin(logger *slog.Logger) *AccessLogPlugin {
	if logger == nil {
		logger = slog.Default()
	}
	return &AccessLogPlugin{logger: logger}
}

// RequestStart stamps the operation start time.
func (p *AccessLogPlugin) RequestStart(ctx context.Context, req core.Request) (context.Context, error) {
	return context.WithValue(ctx, accessLogKey{}, &accessLogState{start: time.Now(), operationName: req.OperationName}), nil
}

// ResponseSend logs the access record.
func (p *AccessLogPlugin) ResponseSend(ctx context.Context, res core.Response) error {
	state, _ := ctx.Value(accessLogKey{}).(*accessLogState)
	if state == nil {
		return nil
	}
	op := state.operationName
	if op == "" {
		op = "anonymous"
	}
	p.logger.LogAttrs(ctx, slog.LevelInfo, "graphql.operation",
		slog.String("operation", op),
		slog.Duration("duration", time.Since(state.start)),
		slog.Int("errors", len(res.Errors)),
	)
	return nil
}
