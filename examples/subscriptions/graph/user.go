package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/grx-gql/grx/pubsub"
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

func (UserQuery) ErrorExample(ctx context.Context) (string, error) {
	return "", fmt.Errorf("example error from basic server")
}

// UserMutation groups every write-side resolver for the User entity.
// The Bus dependency is wired by NewSchema so the mutation can publish
// domain events that subscriptions consume.
type UserMutation struct {
	Bus    pubsub.PubSub
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
		payload, err := json.Marshal(user)
		if err != nil {
			return nil, err
		}
		if err := m.Bus.Publish(ctx, userCreatedTopic, payload); err != nil {
			return nil, err
		}
	}
	return &UserCreatePayload{User: user}, nil
}

// UserSubscription groups every stream resolver for the User entity. The
// Bus dependency is wired by NewSchema; UserCreated relays User events
// published by CreateUser to every active GraphQL subscription.
type UserSubscription struct {
	Bus pubsub.PubSub
}

func (s UserSubscription) UserCreated(ctx context.Context) (<-chan *User, error) {
	msgs, err := s.Bus.Subscribe(ctx, userCreatedTopic)
	if err != nil {
		return nil, err
	}
	out := make(chan *User)
	go func() {
		defer close(out)
		for msg := range msgs {
			var user User
			if err := json.Unmarshal(msg.Payload, &user); err != nil {
				continue
			}
			u := user
			select {
			case <-ctx.Done():
				return
			case out <- &u:
			}
		}
	}()
	return out, nil
}
