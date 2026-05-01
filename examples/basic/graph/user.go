package graph

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/patrickkabwe/grx/pkg/pubsub"
)

// userCreatedTopic is the bus topic on which User events are published.
// Centralising it here avoids drift between publisher and subscriber.
const userCreatedTopic = "user.created"

type User struct {
	ID    string  `gql:"id,nonNull"`
	Name  string  `gql:"name,nonNull"`
	Email *string `gql:"email"`
}

type UserArgs struct {
	ID string `gql:"id,nonNull"`
}

type UserCreateInput struct {
	Name  string  `gql:"name,nonNull"`
	Email *string `gql:"email"`
}

type UserCreateArgs struct {
	Input UserCreateInput `gql:"input,nonNull"`
}

type UserCreatePayload struct {
	User *User `gql:"user,nonNull"`
}

// UserQuery groups every read-side resolver for the User entity.
type UserQuery struct{}

func (UserQuery) User(ctx context.Context, args UserArgs) (*User, error) {
	email := "ada@example.com"
	return &User{ID: args.ID, Name: "Ada Lovelace", Email: &email}, nil
}

// UserMutation groups every write-side resolver for the User entity.
// The Bus dependency is wired by NewSchema so the mutation can publish
// domain events that subscriptions consume.
type UserMutation struct {
	Bus    *pubsub.Typed[*User]
	nextID atomic.Uint64
}

func (m *UserMutation) CreateUser(ctx context.Context, args UserCreateArgs) (*UserCreatePayload, error) {
	id := m.nextID.Add(1)
	user := &User{
		ID:    fmt.Sprintf("user_%d", id),
		Name:  args.Input.Name,
		Email: args.Input.Email,
	}
	if m.Bus != nil {
		if err := m.Bus.Publish(ctx, userCreatedTopic, user); err != nil {
			return nil, err
		}
	}
	return &UserCreatePayload{User: user}, nil
}

// UserSubscription groups every stream resolver for the User entity. The
// Bus dependency is wired by NewSchema; UserCreated relays User events
// published by CreateUser to every active GraphQL subscription.
type UserSubscription struct {
	Bus *pubsub.Typed[*User]
}

func (s UserSubscription) UserCreated(ctx context.Context) (<-chan *User, error) {
	return s.Bus.Subscribe(ctx, userCreatedTopic)
}
