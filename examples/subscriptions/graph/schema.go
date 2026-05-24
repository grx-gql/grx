package graph

import (
	"github.com/grx-gql/grx/memory-pubsub"
	"github.com/grx-gql/grx/schema"
)

// SchemaOption configures [New].
type SchemaOption func(*schemaOptions)

type schemaOptions struct {
	bus pubsub.PubSub
}

// WithPubSub wires the typed pub/sub bridge used by mutation and subscription
// resolvers. Required for this example schema, which publishes across
// resolvers via [pubsub](https://pkg.go.dev/github.com/grx-gql/grx/memory-pubsub).
func WithPubSub(bus pubsub.PubSub) SchemaOption {
	return func(o *schemaOptions) {
		o.bus = bus
	}
}

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

// New composes schema.Config using functional options such as [WithPubSub].
// Which URL path listens for subscriptions (WebSocket/SSE vs POST JSON) is
// configured on the HTTP server ([server.Config.SubscriptionPath]), not here.
func New(opts ...SchemaOption) schema.Config {
	var o schemaOptions
	for _, apply := range opts {
		apply(&o)
	}
	if o.bus == nil {
		panic("graph: WithPubSub is required for this schema (mutations/subscriptions publish through pubsub)")
	}
	return wiredSchema(o.bus)
}

func wiredSchema(bus pubsub.PubSub) schema.Config {
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

// NewSchema is equivalent to New(WithPubSub(bus)).
func NewSchema(bus pubsub.PubSub) schema.Config {
	return New(WithPubSub(bus))
}
