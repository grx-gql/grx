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

// Schema composes schema.Config. A nil bus returns the query and non-event
// mutation schema; a bus adds event mutations and subscriptions.
func Schema(bus pubsub.PubSub) schema.Config {
	config := schema.Config{
		Query:    Query{},
		Mutation: PostMutation{},
	}
	if bus == nil {
		return config
	}
	config.Mutation = Mutation{
		PostMutation:    PostMutation{},
		UserMutation:    &UserMutation{Bus: bus},
		MessageMutation: &MessageMutation{Bus: bus},
	}
	config.Subscription = Subscription{
		UserSubscription:    UserSubscription{Bus: bus},
		MessageSubscription: MessageSubscription{Bus: bus},
	}
	return config
}
