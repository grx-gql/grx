package graph

import (
	"github.com/patrickkabwe/grx/pkg/pubsub"
	"github.com/patrickkabwe/grx/schema"
)

// Query is the schema's root query type. Each entity contributes its own
// resolver struct (e.g. UserQuery, PostQuery) which is embedded here so its
// methods become fields on the root Query type. To add a new entity, define
// its <Entity>Query struct in its own file and embed it below.
type Query struct {
	UserQuery
	PostQuery
}

// Mutation is the schema's root mutation type. Embed each entity's
// <Entity>Mutation struct to expose its mutation fields. Mutations
// that publish domain events take a typed pubsub dependency so
// subscriptions can fan the events out to connected clients.
type Mutation struct {
	*UserMutation
	PostMutation
	*MessageMutation
}

// Subscription is the schema's root subscription type. Each entity
// that publishes streams contributes its own <Entity>Subscription
// struct, receiving the same typed bus instance as the matching
// mutation so events flow end-to-end.
type Subscription struct {
	UserSubscription
	MessageSubscription
}

// NewSchema constructs a fully wired Schema value. The supplied bus is
// the underlying transport used for cross-resolver event delivery.
// Pass [pubsub.NewMemory] for the in-process implementation, or any
// [pubsub.PubSub] (such as the redis sub-module) when running across
// multiple replicas.
func NewSchema(bus pubsub.PubSub) schema.Config {
	users := pubsub.NewTyped[*User](bus)
	messages := pubsub.NewTyped[*Message](bus)

	return schema.Config{
		Query: Query{},
		Mutation: Mutation{
			UserMutation:    &UserMutation{Bus: users},
			MessageMutation: &MessageMutation{Bus: messages},
		},
		Subscription: Subscription{
			UserSubscription:    UserSubscription{Bus: users},
			MessageSubscription: MessageSubscription{Bus: messages},
		},
	}
}
