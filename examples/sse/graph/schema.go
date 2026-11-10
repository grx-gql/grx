package graph

import (
	"context"
	"time"

	"github.com/grx-gql/grx/pubsub"
	"github.com/grx-gql/grx/schema"
)

// Query is the schema's root query type.
type Query struct{}

// ServerTime returns the server's current UTC time.
func (Query) ServerTime(ctx context.Context) (string, error) {
	return time.Now().UTC().Format(time.RFC3339), nil
}

// Subscription is the schema's root subscription type. Countdown streams on
// its own; Notifications relays events published by SendNotification.
type Subscription struct {
	CountdownSubscription
	NotificationSubscription
}

// Schema composes schema.Config. The bus carries notification events from
// the mutation to every active subscription.
func Schema(bus pubsub.PubSub) schema.Config {
	return schema.Config{
		Query:    Query{},
		Mutation: &NotificationMutation{Bus: bus},
		Subscription: Subscription{
			NotificationSubscription: NotificationSubscription{Bus: bus},
		},
	}
}
