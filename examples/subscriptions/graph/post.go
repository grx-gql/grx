package graph

import "context"

type Post struct {
	ID     string `gql:"id,nonNull"`
	Title  string `gql:"title,nonNull"`
	Body   string `gql:"body,nonNull"`
	Author *User  `gql:"author,nonNull"`
}

type PostArgs struct {
	ID string `gql:"id,nonNull"`
}

type PostCreateInput struct {
	Title    string `gql:"title,nonNull"`
	Body     string `gql:"body,nonNull"`
	AuthorID string `gql:"authorId,nonNull"`
}

type PostCreateArgs struct {
	Input PostCreateInput `gql:"input,nonNull"`
}

type PostCreatePayload struct {
	Post *Post `gql:"post,nonNull"`
}

type PostQuery struct{}

func (PostQuery) Post(ctx context.Context, args PostArgs) (*Post, error) {
	email := "ada@example.com"
	return &Post{
		ID:     args.ID,
		Title:  "Hello, grx",
		Body:   "Composing root types from per-entity resolver structs.",
		Author: &User{ID: "user_1", Name: "Ada Lovelace", Email: &email},
	}, nil
}

type PostMutation struct{}

func (PostMutation) CreatePost(ctx context.Context, args PostCreateArgs) (*PostCreatePayload, error) {
	post := &Post{
		ID:     "post_1",
		Title:  args.Input.Title,
		Body:   args.Input.Body,
		Author: &User{ID: args.Input.AuthorID, Name: "Ada Lovelace"},
	}
	return &PostCreatePayload{Post: post}, nil
}
