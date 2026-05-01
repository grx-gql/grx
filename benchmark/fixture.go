// Package benchmark contains comparative micro-benchmarks for the grx
// runtime against other production Go GraphQL libraries.
//
// All three implementations expose an equivalent schema:
//
//	type User { id: ID!, name: String!, email: String }
//	type Post { id: ID!, title: String!, body: String!, author: User! }
//	type Query {
//	  user(id: ID!): User
//	  post(id: ID!): Post
//	  users(count: Int!): [User!]!
//	}
//
// They share the same in-memory fixture data so that the only thing being
// measured is request-time overhead (parsing + validation + execution +
// JSON serialization) and not data-source cost.
package benchmark

import "fmt"

// User is the canonical Go shape for the benchmark User type. It is reused
// by all three implementations so the per-library comparison is honest.
type User struct {
	ID    string
	Name  string
	Email *string
}

// Post is the canonical Go shape for the benchmark Post type.
type Post struct {
	ID     string
	Title  string
	Body   string
	Author *User
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
