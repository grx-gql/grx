package core

import (
	"context"
	"testing"
)

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-123")
	if got := RequestIDFromContext(ctx); got != "req-123" {
		t.Fatalf("RequestIDFromContext = %q", got)
	}
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty id, got %q", got)
	}
}
