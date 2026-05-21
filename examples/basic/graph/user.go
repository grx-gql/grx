package graph

import (
	"context"
)

type User struct {
	ID    string  `gql:"id,nonNull"`
	Name  string  `gql:"name,nonNull"`
	Email *string `gql:"email"`
}

type UserArgs struct {
	ID string `gql:"id,nonNull"`
}

func (Query) User(ctx context.Context, args UserArgs) (*User, error) {
	email := "ada@example.com"
	return &User{ID: args.ID, Name: "Ada Lovelace", Email: &email}, nil
}
