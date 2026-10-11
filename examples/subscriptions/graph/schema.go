package graph

import (
	"github.com/grx-gql/grx/pubsub"
	"github.com/grx-gql/grx/schema"
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

// New composes schema.Config. With no bus, it returns the query and
// non-event mutation schema. Passing one bus adds event mutations and
// subscriptions.
func New(buses ...pubsub.PubSub) schema.Config {
	if len(buses) > 1 {
		panic("graph: New accepts at most one pubsub.PubSub")
	}
	if len(buses) == 0 || buses[0] == nil {
		return wiredSchema(nil)
	}
	return wiredSchema(buses[0])
}

func wiredSchema(bus pubsub.PubSub) schema.Config {
	mutation := Mutation{PostMutation: PostMutation{}}
	config := schema.Config{
		Query:    Query{},
		Mutation: mutation,
	}
	if bus == nil {
		return config
	}
	mutation.UserMutation = &UserMutation{Bus: bus}
	mutation.MessageMutation = &MessageMutation{Bus: bus}
	config.Mutation = mutation
	config.Subscription = Subscription{
		UserSubscription:    UserSubscription{Bus: bus},
		MessageSubscription: MessageSubscription{Bus: bus},
	}
	return config
}
