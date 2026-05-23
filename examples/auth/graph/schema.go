package graph

import (
	"context"
	"errors"

	"github.com/patrickkabwe/grx/examples/auth/session"
	"github.com/patrickkabwe/grx/schema"
)

type Query struct{}

// User mirrors a row the viewer is allowed to see after authentication.
type User struct {
	ID          string `gql:"id,nonNull"`
	DisplayName string `gql:"displayName,nonNull"`
}

func NewSchema() schema.Config {
	return schema.Config{
		Query: Query{},
	}
}

func (Query) Ping() string {
	return "ok"
}

// Viewer resolves only after [session.Subject] is present; the executor's field
// authorizer rejects anonymous callers before this resolver runs.
func (Query) Viewer(ctx context.Context) (*User, error) {
	sub, ok := session.Subject(ctx)
	if !ok {
		return nil, errors.New("unauthenticated")
	}
	return &User{ID: sub, DisplayName: "Signed-in subject " + sub}, nil
}
