package exec

import "context"

type fieldExecutionModeKey struct{}

func withFieldExecutionMode(ctx context.Context, serial bool) context.Context {
	return context.WithValue(ctx, fieldExecutionModeKey{}, serial)
}

func fieldExecutionSerial(ctx context.Context) bool {
	serial, ok := ctx.Value(fieldExecutionModeKey{}).(bool)
	if !ok {
		return true
	}
	return serial
}
