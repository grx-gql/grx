// Package graph demonstrates GraphQL directives in grx.
//
// Two kinds of directives appear here:
//
//   - Query-time built-ins @skip and @include, which clients apply in their
//     operations to conditionally omit fields. No schema wiring is required.
//   - The schema directive @deprecated, declared on fields via the `gql` struct
//     tag's `deprecated` option, which surfaces in introspection and GraphiQL.
package graph

import (
	"context"

	"github.com/patrickkabwe/grx/schema"
)

type Query struct{}

// Profile shows a field marked deprecated through the struct tag. The reason is
// reported in introspection so tooling can warn clients.
type Profile struct {
	ID       string `gql:"id,nonNull"`
	Username string `gql:"username,nonNull"`
	// legacyName is retained for older clients but discouraged.
	LegacyName string `gql:"legacyName,deprecated=Use username instead"`
}

func NewSchema() schema.Config {
	return schema.Config{
		Query: Query{},
	}
}

// Me returns a profile. Exercise the built-in directives from a client:
//
//	{
//	  me {
//	    id
//	    username @include(if: true)
//	    legacyName @skip(if: true)
//	  }
//	}
func (Query) Me(ctx context.Context) (*Profile, error) {
	return &Profile{ID: "1", Username: "ada", LegacyName: "Ada Lovelace"}, nil
}
