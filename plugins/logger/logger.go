// Package logger provides a [plugins.Plugin] that records GraphQL request
// lifecycle events through a structured slog logger. It is the canonical
// example of a built-in plugin and a useful drop-in for production
// observability.
package logger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/plugins"
)

type requestStartedAtCtxKey struct{}

func requestElapsed(ctx context.Context) (time.Duration, bool) {
	v := ctx.Value(requestStartedAtCtxKey{})
	if v == nil {
		return 0, false
	}
	started, ok := v.(time.Time)
	if !ok {
		return 0, false
	}
	return time.Since(started), true
}

func graphqlResponseTimeAttr(d time.Duration) slog.Attr {
	const key = "graphql.response.time"
	switch {
	case d >= time.Second:
		return slog.String(key, fmt.Sprintf("%.3fs", d.Seconds()))
	case d <= 0:
		return slog.String(key, "0ms")
	default:
		ms := float64(d) / float64(time.Millisecond)
		return slog.String(key, fmt.Sprintf("%.3fms", ms))
	}
}

// Config configures a [Logger]. Logger is required.
type Config struct {
	// Logger is the slog logger used to emit lifecycle events. The
	// plugins attaches the request context to every log call so that
	// per-request fields propagated via the context are preserved.
	Logger *slog.Logger
}

// Logger is a [plugins.Plugin] that emits a structured log entry at every
// stage of the GraphQL request lifecycle. It embeds [plugins.Base] and
// therefore satisfies the full Plugin interface.
type Logger struct {
	plugins.Base
	logger *slog.Logger
}

// New constructs a [Logger]. It returns an error when config.Logger is
// nil so that misconfigured deployments fail fast at startup.
func New(config Config) (*Logger, error) {
	if config.Logger == nil {
		return nil, errors.New("logger plugins requires a slog logger")
	}

	return &Logger{logger: config.Logger}, nil
}

// RequestStart logs that a request has begun and returns a derived context that
// records the wall-clock start time used for graphql.response.time on
// graphql.response.send and graphql.error.
func (l *Logger) RequestStart(ctx context.Context, req core.Request) (context.Context, error) {
	l.logger.InfoContext(ctx, "graphql.request.start",
		slog.String("operation_name", req.OperationName),
		slog.Int("query_length", len(req.Query)),
		slog.Int("variables_count", len(req.Variables)),
	)
	return context.WithValue(ctx, requestStartedAtCtxKey{}, time.Now()), nil
}

// ParsingStart logs the start of the parsing phase.
func (l *Logger) ParsingStart(ctx context.Context, req core.Request) error {
	l.logger.DebugContext(ctx, "graphql.parsing.start",
		slog.String("operation_name", req.OperationName),
	)
	return nil
}

// ValidationStart logs the start of the validation phase.
func (l *Logger) ValidationStart(ctx context.Context, req core.Request) error {
	l.logger.DebugContext(ctx, "graphql.validation.start",
		slog.String("operation_name", req.OperationName),
	)
	return nil
}

// ExecutionStart logs the start of the execution phase.
func (l *Logger) ExecutionStart(ctx context.Context, req core.Request) error {
	l.logger.DebugContext(ctx, "graphql.execution.start",
		slog.String("operation_name", req.OperationName),
	)
	return nil
}

// FieldResolveStart logs that a field resolver is about to be invoked.
// Field-level events are emitted at debug level to keep production logs
// manageable on schemas with large selection sets.
func (l *Logger) FieldResolveStart(ctx context.Context, field plugins.FieldContext) error {
	l.logger.DebugContext(ctx, "graphql.field.resolve.start",
		slog.String("field_name", field.FieldName),
		slog.Any("path", field.Path),
	)
	return nil
}

// ResponseSend logs that a response is about to be returned to the
// client.
func (l *Logger) ResponseSend(ctx context.Context, res core.Response) error {
	attrs := []slog.Attr{
		slog.Bool("has_data", res.Data != nil),
		slog.Int("errors_count", len(res.Errors)),
	}
	if d, ok := requestElapsed(ctx); ok {
		attrs = append(attrs, graphqlResponseTimeAttr(d))
	}
	l.logger.LogAttrs(ctx, slog.LevelInfo, "graphql.response.send", attrs...)
	return nil
}

// Error logs an error that aborted the request.
func (l *Logger) Error(ctx context.Context, err error) {
	attrs := []slog.Attr{slog.String("error", err.Error())}
	if d, ok := requestElapsed(ctx); ok {
		attrs = append(attrs, graphqlResponseTimeAttr(d))
	}
	l.logger.LogAttrs(ctx, slog.LevelError, "graphql.error", attrs...)
}
