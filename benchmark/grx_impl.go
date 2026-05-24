package benchmark

import (
	"context"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/schema"
)

type grxIDArgs struct {
	ID string `gql:"id,nonNull"`
}

type grxUsersArgs struct {
	Count int `gql:"count,nonNull"`
}

type grxQuery struct{}

func (grxQuery) User(ctx context.Context, args grxIDArgs) (*User, error) {
	return fixtureUser(args.ID), nil
}

func (grxQuery) Post(ctx context.Context, args grxIDArgs) (*Post, error) {
	return fixturePost(args.ID), nil
}

func (grxQuery) Users(ctx context.Context, args grxUsersArgs) ([]*User, error) {
	// Same shape as graph-gophers: return shared fixture pointers, no copying.
	return fixtureUsers(args.Count), nil
}

type grxFeedArgs struct {
	Limit int `gql:"limit,nonNull"`
}

func (grxQuery) Feed(ctx context.Context, args grxFeedArgs) ([]*Post, error) {
	return fixtureFeed(args.Limit), nil
}

// newGRXExecutor builds an executor for benchmark loops.
func newGRXExecutor() core.Executor {
	s, err := schema.Build(schema.Config{Query: grxQuery{}})
	if err != nil {
		panic(err)
	}
	return exec.New(s, nil)
}
