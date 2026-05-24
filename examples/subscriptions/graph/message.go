package graph

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/grx-gql/grx/memory-pubsub"
)

// messagePostedTopic is the bus topic for chat message events. A single
// topic is used; subscribers filter by RoomID via a typed predicate so
// consumers never see messages from other rooms.
const messagePostedTopic = "message.posted"

type Message struct {
	ID       string `gql:"id,nonNull"`
	RoomID   string `gql:"roomId,nonNull"`
	Author   string `gql:"author,nonNull"`
	Body     string `gql:"body,nonNull"`
	PostedAt string `gql:"postedAt,nonNull"`
}

type PostMessageInput struct {
	RoomID string `gql:"roomId,nonNull"`
	Author string `gql:"author,nonNull"`
	Body   string `gql:"body,nonNull"`
}

type PostMessageArgs struct {
	Input PostMessageInput `gql:"input,nonNull"`
}

type PostMessagePayload struct {
	Message *Message `gql:"message,nonNull"`
}

type MessagePostedArgs struct {
	RoomID string `gql:"roomId,nonNull"`
}

// MessageMutation owns the write side of the Message entity. It assigns
// a monotonic ID, stamps the server time, and publishes the new message
// on the shared typed bus so room subscribers can fan it out.
type MessageMutation struct {
	Bus    *pubsub.Typed[*Message]
	nextID atomic.Uint64
}

func (m *MessageMutation) PostMessage(ctx context.Context, args PostMessageArgs) (*PostMessagePayload, error) {
	if strings.TrimSpace(args.Input.RoomID) == "" {
		return nil, fmt.Errorf("roomId is required")
	}
	if strings.TrimSpace(args.Input.Body) == "" {
		return nil, fmt.Errorf("body is required")
	}

	id := m.nextID.Add(1)
	message := &Message{
		ID:       fmt.Sprintf("msg_%d", id),
		RoomID:   args.Input.RoomID,
		Author:   args.Input.Author,
		Body:     args.Input.Body,
		PostedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if m.Bus != nil {
		if err := m.Bus.Publish(ctx, messagePostedTopic, message); err != nil {
			return nil, err
		}
	}
	return &PostMessagePayload{Message: message}, nil
}

// MessageSubscription owns the streaming side of the Message entity.
// MessagePosted demonstrates argument-driven filtering: a single topic
// carries every room's traffic, but each subscriber only forwards
// events that match its requested RoomID via a typed predicate. This
// is the standard pattern for chat rooms, ticker symbols, channel
// feeds, etc.
type MessageSubscription struct {
	Bus *pubsub.Typed[*Message]
}

func (s MessageSubscription) MessagePosted(ctx context.Context, args MessagePostedArgs) (<-chan *Message, error) {
	if strings.TrimSpace(args.RoomID) == "" {
		return nil, fmt.Errorf("roomId is required")
	}
	return s.Bus.Subscribe(ctx, messagePostedTopic, func(m *Message) bool {
		return m != nil && m.RoomID == args.RoomID
	})
}
