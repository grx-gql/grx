package graph

import (
	"github.com/grx-gql/grx/pubsub"
	"github.com/grx-gql/grx/schema"
)

// SchemaOption configures [New].
type SchemaOption func(*schemaOptions)

type schemaOptions struct {
	events Events
}

// Events holds the pub/sub bus used by mutation and subscription resolvers.
type Events struct {
	Bus pubsub.PubSub
}

// WithEvents wires the pub/sub bus used by mutation and subscription
// resolvers. Required for this example schema.
func WithEvents(events Events) SchemaOption {
	return func(o *schemaOptions) {
		o.events = events
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
// that publish domain events take a pubsub dependency so subscriptions
// can fan the events out to connected clients.
type Mutation struct {
	*UserMutation
	PostMutation
	*MessageMutation
}

// Subscription is the schema's root subscription type. Each entity
// that publishes streams contributes its own <Entity>Subscription
// struct, receiving the same bus instance as the matching mutation
// so events flow end-to-end.
type Subscription struct {
	UserSubscription
	MessageSubscription
}

// New composes schema.Config using functional options such as [WithEvents].
func New(opts ...SchemaOption) schema.Config {
	var o schemaOptions
	for _, apply := range opts {
		apply(&o)
	}
	if o.events.Bus == nil {
		panic("graph: WithEvents is required for this schema (mutations/subscriptions publish through pubsub)")
	}
	return wiredSchema(o.events)
}

func wiredSchema(events Events) schema.Config {
	return schema.Config{
		Query: Query{},
		Mutation: Mutation{
			UserMutation:    &UserMutation{Bus: events.Bus},
			MessageMutation: &MessageMutation{Bus: events.Bus},
		},
		Subscription: Subscription{
			UserSubscription:    UserSubscription{Bus: events.Bus},
			MessageSubscription: MessageSubscription{Bus: events.Bus},
		},
	}
}

// NewSchema is equivalent to New(WithEvents(events)).
func NewSchema(events Events) schema.Config {
	return New(WithEvents(events))
}
