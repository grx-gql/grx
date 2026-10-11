package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/grx-gql/grx/pubsub"
)

// messagePostedTopicPrefix is the bus topic prefix for chat message events.
// Each room gets its own topic so subscribers receive only that room's traffic.
const messagePostedTopicPrefix = "message.posted."

func messagePostedTopic(roomID string) string {
	return messagePostedTopicPrefix + roomID
}

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
// on the room's topic so subscribers for that room receive it.
type MessageMutation struct {
	Bus    pubsub.PubSub
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
		payload, err := json.Marshal(message)
		if err != nil {
			return nil, err
		}
		if err := m.Bus.Publish(ctx, messagePostedTopic(message.RoomID), payload); err != nil {
			return nil, err
		}
	}
	return &PostMessagePayload{Message: message}, nil
}

// MessageSubscription owns the streaming side of the Message entity.
// MessagePosted subscribes to the room-specific topic so each GraphQL
// subscription receives only messages for its requested roomId.
type MessageSubscription struct {
	Bus pubsub.PubSub
}

func (s MessageSubscription) MessagePosted(ctx context.Context, args MessagePostedArgs) (<-chan *Message, error) {
	if strings.TrimSpace(args.RoomID) == "" {
		return nil, fmt.Errorf("roomId is required")
	}
	msgs, err := s.Bus.Subscribe(ctx, messagePostedTopic(args.RoomID))
	if err != nil {
		return nil, err
	}
	out := make(chan *Message)
	go func() {
		defer close(out)
		for msg := range msgs {
			var message Message
			if err := json.Unmarshal(msg.Payload, &message); err != nil {
				continue
			}
			m := message
			select {
			case <-ctx.Done():
				return
			case out <- &m:
			}
		}
	}()
	return out, nil
}
