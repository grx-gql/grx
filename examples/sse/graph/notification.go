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

// notificationTopicPrefix is the bus topic prefix for notification events.
// Each channel gets its own topic so subscribers only receive its traffic.
const notificationTopicPrefix = "notification.sent."

func notificationTopic(channel string) string {
	return notificationTopicPrefix + channel
}

type Notification struct {
	ID      string `gql:"id,nonNull"`
	Channel string `gql:"channel,nonNull"`
	Text    string `gql:"text,nonNull"`
	SentAt  string `gql:"sentAt,nonNull"`
}

type SendNotificationInput struct {
	Channel string `gql:"channel,nonNull"`
	Text    string `gql:"text,nonNull"`
}

type SendNotificationArgs struct {
	Input SendNotificationInput `gql:"input,nonNull"`
}

type NotificationsArgs struct {
	Channel string `gql:"channel,nonNull"`
}

// NotificationMutation publishes each notification on its channel's topic.
type NotificationMutation struct {
	Bus    pubsub.PubSub
	nextID atomic.Uint64
}

func (m *NotificationMutation) SendNotification(ctx context.Context, args SendNotificationArgs) (*Notification, error) {
	channel := strings.TrimSpace(args.Input.Channel)
	if channel == "" {
		return nil, fmt.Errorf("channel is required")
	}
	if strings.TrimSpace(args.Input.Text) == "" {
		return nil, fmt.Errorf("text is required")
	}

	notification := &Notification{
		ID:      fmt.Sprintf("notification_%d", m.nextID.Add(1)),
		Channel: channel,
		Text:    args.Input.Text,
		SentAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(notification)
	if err != nil {
		return nil, err
	}
	if err := m.Bus.Publish(ctx, notificationTopic(channel), payload); err != nil {
		return nil, err
	}
	return notification, nil
}

// NotificationSubscription relays the notifications published on a channel
// to every client subscribed to it.
type NotificationSubscription struct {
	Bus pubsub.PubSub
}

func (s NotificationSubscription) Notifications(ctx context.Context, args NotificationsArgs) (<-chan *Notification, error) {
	channel := strings.TrimSpace(args.Channel)
	if channel == "" {
		return nil, fmt.Errorf("channel is required")
	}
	msgs, err := s.Bus.Subscribe(ctx, notificationTopic(channel))
	if err != nil {
		return nil, err
	}
	out := make(chan *Notification)
	go func() {
		defer close(out)
		for msg := range msgs {
			var notification Notification
			if err := json.Unmarshal(msg.Payload, &notification); err != nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- &notification:
			}
		}
	}()
	return out, nil
}
