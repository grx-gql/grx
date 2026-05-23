// Package session stores an authenticated subject on [context.Context] for this example.
//
// Production code normally uses JWT claims keys or an opaque session id—not raw strings—
// but the pattern mirrors how HTTP middleware attaches identity before GraphQL runs.
package session

import (
	"context"
)

type subjectKey struct{}

// ContextWithSubject returns a derived context carrying the authenticated subject id.
func ContextWithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subject)
}

// Subject returns ("", false) when the request context has no subject.
func Subject(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(subjectKey{}).(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}
