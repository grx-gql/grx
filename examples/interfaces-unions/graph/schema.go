// Package graph defines a schema that registers a GraphQL interface and a
// union, both backed by Go interface types.
package graph

import (
	"context"

	"github.com/grx-gql/grx/schema"
)

type Query struct{}

// Node is a Go interface backing the GraphQL Node interface. Concrete types
// satisfy it by implementing its unexported marker method.
type Node interface {
	isNode()
}

// SearchResult is a Go interface backing the GraphQL SearchResult union.
type SearchResult interface {
	isSearchResult()
}

// User implements both Node and SearchResult.
type User struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name,nonNull"`
}

func (*User) isNode()         {}
func (*User) isSearchResult() {}

// Post implements both Node and SearchResult.
type Post struct {
	ID    string `gql:"id,nonNull"`
	Title string `gql:"title,nonNull"`
}

func (*Post) isNode()         {}
func (*Post) isSearchResult() {}

// NewSchema registers the Node interface and SearchResult union, listing the
// concrete implementors so the executor can resolve the runtime type.
func NewSchema() schema.Config {
	return schema.Config{
		Query: Query{},
		Interfaces: []schema.InterfaceConfig{
			{
				Type:         (*Node)(nil),
				Implementors: []any{User{}, Post{}},
			},
		},
		Unions: []schema.UnionConfig{
			{
				Type:         (*SearchResult)(nil),
				Name:         "SearchResult",
				Implementors: []any{User{}, Post{}},
			},
		},
	}
}

// Node returns a value typed as the Node interface. Query it with inline
// fragments: { node(kind: "post") { ... on Post { id title } ... on User { id name } } }
func (Query) Node(ctx context.Context, args struct {
	Kind string `gql:"kind,default=user"`
}) (Node, error) {
	if args.Kind == "post" {
		return &Post{ID: "post_1", Title: "GraphQL interfaces"}, nil
	}
	return &User{ID: "user_1", Name: "Ada"}, nil
}

// Search returns a value typed as the SearchResult union.
// { search(kind: "user") { ... on User { id name } ... on Post { id title } } }
func (Query) Search(ctx context.Context, args struct {
	Kind string `gql:"kind,default=user"`
}) (SearchResult, error) {
	if args.Kind == "post" {
		return &Post{ID: "post_1", Title: "GraphQL unions"}, nil
	}
	return &User{ID: "user_1", Name: "Ada"}, nil
}
