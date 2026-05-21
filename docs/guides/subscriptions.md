---
title: Add Subscriptions
description: Wire up a subscription endpoint backed by both WebSocket and SSE.
outline: [2, 3]
---

This guide takes the
[Query &amp; Mutation Server](/guides/query-mutation-server) and adds a
real-time subscription that emits a `User` value once a second over both
WebSocket and SSE.

## 1. Add the subscription resolver

```go
// graph/user.go (additions)

type UserSubscription struct{}

func (UserSubscription) UserCreated(ctx context.Context) (<-chan *User, error) {
    stream := make(chan *User)
    go func() {
        defer close(stream)
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()
        for index := 1; ; index++ {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                email := fmt.Sprintf("user_%d@example.com", index)
                select {
                case <-ctx.Done():
                    return
                case stream <- &User{
                    ID:    fmt.Sprintf("user_%d", index),
                    Name:  fmt.Sprintf("User %d", index),
                    Email: &email,
                }:
                }
            }
        }
    }()
    return stream, nil
}
```

The double `select` on `ctx.Done()` is intentional: it ensures the
goroutine exits both when the ticker fires (but the consumer is gone) and
when the consumer is gone before the next tick.

## 2. Add the subscription root

```go
// graph/schema.go
import "github.com/patrickkabwe/grx/schema"

type Subscription struct {
    UserSubscription
}

func NewSchema() schema.Config {
    return schema.Config{
        Query:        Query{},
        Mutation:     Mutation{},
        Subscription: Subscription{},
    }
}
```

## 3. Register the transports

Subscriptions are opt-in. With no transports configured, subscription
endpoints return `404`. Enable WebSocket and SSE on the server:

```go
import (
    "net/http"
    "time"

    "github.com/patrickkabwe/grx"
    "github.com/patrickkabwe/grx/pkg/sse"
    "github.com/patrickkabwe/grx/pkg/websocket"
)

srv, err := grx.NewServer(
    grx.WithSchema(graph.NewSchema()),
    grx.WithPlaygroundPath("/"),
    grx.WithTransports(
        websocket.New(websocket.Config{
            ConnectionInitTimeout: 3 * time.Second,
            ReadIdleTimeout:       60 * time.Second,
            WriteTimeout:          10 * time.Second,
            MaxMessageSize:        1 << 20,
            CheckOrigin: func(r *http.Request) bool {
                origin := r.Header.Get("Origin")
                return origin == "https://app.example.com"
            },
            PingInterval:          25 * time.Second,
        }),
        sse.New(),
    ),
)
```

To **split** the subscription wire path from `POST /graphql` (for example
`/graphql` for HTTP and `/graphql/ws` for WebSocket and SSE), add
`grx.WithSubscriptionPath("/graphql/ws")`. The server automatically moves
only the bundled `*websocket.Transport` and `*sse.Transport` to that path.
That is an organizational choice; it is also common to keep a **single**
URL for both HTTP and `graphql-transport-ws`.

```go
srv, err := grx.NewServer(
    grx.WithSchema(graph.NewSchema()),
    grx.WithPlaygroundPath("/"),
    grx.WithSubscriptionPath("/graphql/ws"),
    grx.WithTransports(websocket.New(/* ... */), sse.New()),
)
```

In production you almost certainly want a real `CheckOrigin` function on
the WebSocket transport. A nil `CheckOrigin` accepts every origin, which
is convenient for local development but too broad for browser-facing
deployments.

## 4. Try it from the playground

The bundled GraphiQL UI auto-detects subscriptions. Run:

```graphql
subscription {
  userCreated { id name email }
}
```

You should see one `data` payload per second.

## 5. Try it from a script

`graphql-ws` from npm speaks `graphql-transport-ws` directly:

```js
import { createClient } from "graphql-ws";

const client = createClient({ url: "ws://localhost:4000/graphql" }); // or /graphql/ws if you split paths

(async () => {
  const sub = client.iterate({
    query: "subscription { userCreated { id name email } }",
  });
  for await (const event of sub) {
    console.log(event.data);
  }
})();
```

For SSE, just `curl`:

```bash
curl -N -H "Accept: text/event-stream" \
  --data-raw '{"query":"subscription { userCreated { id name } }"}' \
  -H "Content-Type: application/json" \
  http://localhost:4000/graphql  # or your subscription path when split
```

## Authentication on connect

For WebSocket subscriptions, the auth decision happens once when the
client opens the connection, not per message. Wire it up via
`websocket.Config.OnConnect`:

```go
websocket.New(websocket.Config{
    OnConnect: func(ctx context.Context, payload map[string]any) (context.Context, map[string]any, error) {
        token, _ := payload["authToken"].(string)
        user, err := authenticate(ctx, token)
        if err != nil {
            return nil, nil, err
        }
        return context.WithValue(ctx, userKey{}, user), map[string]any{"ok": true}, nil
    },
})
```

The returned context is propagated to every subscription on the
connection; the map becomes the `connection_ack` payload.

## Cross-resolver fan-out with `pubsub`

The ticker pattern above is fine for demos but not for real workloads.
Most subscriptions stream events that **mutation resolvers** publish
elsewhere — e.g. a `messagePosted` subscription that fans out values
produced by a `postMessage` mutation. grx ships
[`pkg/pubsub`](/concepts/pubsub) for exactly this.

### Wire a typed bus

```go
// graph/message.go
import (
    "context"
    "fmt"
    "strings"
    "sync/atomic"
    "time"

    "github.com/patrickkabwe/grx/pkg/pubsub"
)

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

type PostMessageArgs struct{ Input PostMessageInput `gql:"input,nonNull"` }
type PostMessagePayload struct{ Message *Message `gql:"message,nonNull"` }
type MessagePostedArgs struct{ RoomID string `gql:"roomId,nonNull"` }

type MessageMutation struct {
    Bus    *pubsub.Typed[*Message]
    nextID atomic.Uint64
}

func (m *MessageMutation) PostMessage(ctx context.Context, args PostMessageArgs) (*PostMessagePayload, error) {
    if strings.TrimSpace(args.Input.RoomID) == "" {
        return nil, fmt.Errorf("roomId is required")
    }
    msg := &Message{
        ID:       fmt.Sprintf("msg_%d", m.nextID.Add(1)),
        RoomID:   args.Input.RoomID,
        Author:   args.Input.Author,
        Body:     args.Input.Body,
        PostedAt: time.Now().UTC().Format(time.RFC3339Nano),
    }
    if err := m.Bus.Publish(ctx, messagePostedTopic, msg); err != nil {
        return nil, err
    }
    return &PostMessagePayload{Message: msg}, nil
}

type MessageSubscription struct{ Bus *pubsub.Typed[*Message] }

// One topic carries every room's traffic; each subscriber filters
// down to its requested RoomID via a typed predicate. Predicates run
// on the publish path, so other rooms' messages never wake this
// goroutine.
func (s MessageSubscription) MessagePosted(ctx context.Context, args MessagePostedArgs) (<-chan *Message, error) {
    return s.Bus.Subscribe(ctx, messagePostedTopic, func(m *Message) bool {
        return m != nil && m.RoomID == args.RoomID
    })
}
```

### Construct the bus once and share it

```go
// graph/schema.go
func NewSchema(bus pubsub.PubSub) schema.Config {
    messages := pubsub.NewTyped[*Message](bus)

    return schema.Config{
        Query: Query{},
        Mutation: Mutation{
            MessageMutation: &MessageMutation{Bus: messages},
        },
        Subscription: Subscription{
            MessageSubscription: MessageSubscription{Bus: messages},
        },
    }
}

// main.go
bus := pubsub.NewMemory()
defer bus.Close()

srv, _ := grx.NewServer(
    grx.WithSchema(graph.NewSchema(bus)),
    grx.WithTransports(
        websocket.New(websocket.Config{CheckOrigin: allowOrigin}),
        sse.New(),
    ),
)
```

### Try it

Run two GraphiQL tabs. In the first:

```graphql
subscription {
  messagePosted(roomId: "general") { id author body }
}
```

In the second, post a message:

```graphql
mutation {
  postMessage(input: {roomId: "general", author: "alice", body: "hi"}) {
    message { id }
  }
}
```

The first tab receives the value immediately. Posting to a *different*
room does not wake the subscriber because the predicate runs on the
publish path.

### Going multi-replica

When you outgrow a single process, swap `pubsub.NewMemory()` for the
Redis backend — nothing else changes:

```go
import (
    redispubsub "github.com/patrickkabwe/grx/pkg/pubsub/redis"
    "github.com/redis/go-redis/v9"
)

rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
bus, _ := redispubsub.New(redispubsub.Config{Client: rdb, Prefix: "chat:"})
```

See [Pub/Sub](/concepts/pubsub) for the full surface.
