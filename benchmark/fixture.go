// Package benchmark contains perf tests for the grx runtime using
// production-shaped GraphQL operations and shared in-memory fixtures.
//
// All scenarios use the same canonical Go types and resolvers (see grx_impl.go)
// so timings reflect parse, validate, execution, and JSON encoding only.
//
//	type User { id: ID!, name: String!, email: String }
//	type Post { id: ID!, title: String!, body: String!, author: User! }
//	type Query {
//	  user(id: String!): User
//	  post(id: String!): Post
//	  users(count: Int!): [User!]!
//	  feed(limit: Int!): [Post!]!
//	}
//
// They share the same in-memory fixture data so that the only thing being
// measured is request-time overhead (parsing + validation + execution +
// JSON serialization) and not data-source cost.
package benchmark

import "fmt"

// User is the canonical Go shape for the benchmark User type.
// gql tags are used by grx's code-first schema builder.
type User struct {
	ID    string  `gql:"id,nonNull"`
	Name  string  `gql:"name,nonNull"`
	Email *string `gql:"email"`
}

// Post is the canonical Go shape for the benchmark Post type.
type Post struct {
	ID     string `gql:"id,nonNull"`
	Title  string `gql:"title,nonNull"`
	Body   string `gql:"body,nonNull"`
	Author *User  `gql:"author,nonNull"`
}

// fixtureUsers returns a deterministic slice of users sized to count.
func fixtureUsers(count int) []*User {
	users := make([]*User, count)
	for i := 0; i < count; i++ {
		email := fmt.Sprintf("user_%d@example.com", i)
		users[i] = &User{
			ID:    fmt.Sprintf("user_%d", i),
			Name:  fmt.Sprintf("User %d", i),
			Email: &email,
		}
	}
	return users
}

// fixturePost returns a single deterministic post whose author is fixed so
// every implementation walks the same nested resolver path.
func fixturePost(id string) *Post {
	email := "ada@example.com"
	return &Post{
		ID:     id,
		Title:  "Hello, grx",
		Body:   "Composing root types from per-entity resolver structs.",
		Author: &User{ID: "user_1", Name: "Ada Lovelace", Email: &email},
	}
}

// fixtureUser returns a single deterministic user.
func fixtureUser(id string) *User {
	email := "ada@example.com"
	return &User{ID: id, Name: "Ada Lovelace", Email: &email}
}

// fixtureFeed returns limit posts keyed so each row resolves a nested author
// (timeline-style payloads).
func fixtureFeed(limit int) []*Post {
	if limit <= 0 {
		return nil
	}
	out := make([]*Post, limit)
	for i := 0; i < limit; i++ {
		out[i] = fixturePost(fmt.Sprintf("post_%d", i))
	}
	return out
}
