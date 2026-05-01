package benchmark

import (
	"context"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/exec"
	"github.com/patrickkabwe/grx/schema"
)

// grxUser, grxPost mirror the canonical fixture types but carry the gql tags
// the grx schema builder expects. Keeping them as separate types from the
// neutral User/Post avoids leaking grx-specific tags into the other
// implementations.
type grxUser struct {
	ID    string  `gql:"id,nonNull"`
	Name  string  `gql:"name,nonNull"`
	Email *string `gql:"email"`
}

type grxPost struct {
	ID     string   `gql:"id,nonNull"`
	Title  string   `gql:"title,nonNull"`
	Body   string   `gql:"body,nonNull"`
	Author *grxUser `gql:"author,nonNull"`
}

type grxIDArgs struct {
	ID string `gql:"id,nonNull"`
}

type grxUsersArgs struct {
	Count int `gql:"count,nonNull"`
}

type grxQuery struct{}

func (grxQuery) User(ctx context.Context, args grxIDArgs) (*grxUser, error) {
	u := fixtureUser(args.ID)
	return toGrxUser(u), nil
}

func (grxQuery) Post(ctx context.Context, args grxIDArgs) (*grxPost, error) {
	return toGrxPost(fixturePost(args.ID)), nil
}

func (grxQuery) Users(ctx context.Context, args grxUsersArgs) ([]*grxUser, error) {
	src := fixtureUsers(args.Count)
	out := make([]*grxUser, len(src))
	for i, u := range src {
		out[i] = toGrxUser(u)
	}
	return out, nil
}

func toGrxUser(u *User) *grxUser {
	if u == nil {
		return nil
	}
	return &grxUser{ID: u.ID, Name: u.Name, Email: u.Email}
}

func toGrxPost(p *Post) *grxPost {
	if p == nil {
		return nil
	}
	return &grxPost{ID: p.ID, Title: p.Title, Body: p.Body, Author: toGrxUser(p.Author)}
}

// newGRXExecutor builds the grx executor once so the benchmark loop only
// measures request execution.
func newGRXExecutor() core.Executor {
	s, err := schema.Build(schema.Config{Query: grxQuery{}})
	if err != nil {
		panic(err)
	}
	return exec.New(s, nil)
}
