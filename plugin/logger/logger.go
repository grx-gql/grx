// Package logger provides a [plugin.Plugin] that records GraphQL request
// lifecycle events through a structured slog logger. It is the canonical
// example of a built-in plugin and a useful drop-in for production
// observability.
package logger

import (
	"context"
	"errors"
	"log/slog"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/plugin"
)

// Config configures a [Logger]. Logger is required.
type Config struct {
	// Logger is the slog logger used to emit lifecycle events. The
	// plugin attaches the request context to every log call so that
	// per-request fields propagated via the context are preserved.
	Logger *slog.Logger
}

// Logger is a [plugin.Plugin] that emits a structured log entry at every
// stage of the GraphQL request lifecycle. It embeds [plugin.Base] and
// therefore satisfies the full Plugin interface.
type Logger struct {
	plugin.Base
	logger *slog.Logger
}

// New constructs a [Logger]. It returns an error when config.Logger is
// nil so that misconfigured deployments fail fast at startup.
func New(config Config) (*Logger, error) {
	if config.Logger == nil {
		return nil, errors.New("logger plugin requires a slog logger")
	}

	return &Logger{logger: config.Logger}, nil
}

// RequestStart logs that a request has begun and returns ctx unchanged.
func (l *Logger) RequestStart(ctx context.Context, req core.Request) (context.Context, error) {
	l.logger.InfoContext(ctx, "graphql.request.start",
		slog.String("operation_name", req.OperationName),
		slog.Int("query_length", len(req.Query)),
		slog.Int("variables_count", len(req.Variables)),
	)
	return ctx, nil
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
func (l *Logger) FieldResolveStart(ctx context.Context, field plugin.FieldContext) error {
	l.logger.DebugContext(ctx, "graphql.field.resolve.start",
		slog.String("field_name", field.FieldName),
		slog.Any("path", field.Path),
	)
	return nil
}

// ResponseSend logs that a response is about to be returned to the
// client.
func (l *Logger) ResponseSend(ctx context.Context, res core.Response) error {
	l.logger.InfoContext(ctx, "graphql.response.send",
		slog.Bool("has_data", res.Data != nil),
		slog.Int("errors_count", len(res.Errors)),
	)
	return nil
}

// Error logs an error that aborted the request.
func (l *Logger) Error(ctx context.Context, err error) {
	l.logger.ErrorContext(ctx, "graphql.error",
		slog.String("error", err.Error()),
	)
}
