package core

import "context"

type requestIDCtxKey struct{}

var requestIDKey = &requestIDCtxKey{}

// WithRequestID returns ctx carrying id for downstream resolvers and plugins.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the request ID from ctx, or empty when absent.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}
