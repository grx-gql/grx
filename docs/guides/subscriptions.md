---
title: Realtime subscriptions
description: WebSocket and SSE transports, securing long-lived subscription connections, and pub/sub fan-out from mutations.
outline: deep
---

# Realtime subscriptions

This guide takes the
[Queries and mutations](/guides/query-mutation-server) walkthrough and adds a
real-time subscription that emits a `User` value once a second over both
WebSocket and SSE.

## 1. Add the subscription resolver

```go
package graph

import (
	"context"
	"fmt"
	"time"
)

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
package graph

import "github.com/grx-gql/grx/schema"

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

## 3. Register the transports {#register-subscription-transports}

Subscriptions are opt-in. With no transports configured, subscription
endpoints return `404`. Enable WebSocket and SSE on the server:

```go
package main

import (
	"log"
	"net/http"
	"time"

	"example.com/hello-grx/graph"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/sse"
	"github.com/grx-gql/grx/websocket"
)

func main() {
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
				PingInterval: 25 * time.Second,
			}),
			sse.New(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = srv
}
```

To **split** the subscription wire path from `POST /graphql` (for example
`/graphql` for HTTP and `/graphql/ws` for WebSocket and SSE), add
`grx.WithSubscriptionPath("/graphql/ws")`. The server automatically moves
only the bundled `*websocket.WebSocketTransport` and `*sse.Transport` to that path.
That is an organizational choice; it is also common to keep a **single**
URL for both HTTP and `graphql-transport-ws`.

```go
package main

import (
	"log"
	"time"

	"example.com/hello-grx/graph"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/sse"
	"github.com/grx-gql/grx/websocket"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/"),
		grx.WithSubscriptionPath("/graphql/ws"),
		grx.WithTransports(websocket.New(websocket.Config{ReadIdleTimeout: 60 * time.Second}), sse.New()),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = srv
}
```

For browser clients, treat WebSocket as a **long‑lived ingress**: lock down
TLS, origins, connection auth, and sizing - see **[Securing subscriptions](#securing-subscriptions)**
after the walkthrough.

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

## Securing subscriptions

Subscriptions are **another front door** beside `POST /graphql`: clients can
keep a socket or SSE stream open indefinitely, so apply the same severity you
would at the HTTP layer - and read **[Security](/guides/production-security)**,
**[Authentication & authorization](/guides/auth)**, **[Limits](/guides/request-limits)**,
and **[CORS & browsers](/guides/cors-browsers)** for the full picture.

### TLS, paths, and proxies

Use **`wss://`** (TLS) in production. Putting WebSocket and HTTP behind a reverse
proxy is normal; ensure upgrade headers and idle timeouts are tuned for long
lived connections (`WithSubscriptionPath` helps isolate subscription traffic for
routing and WAF rules - see [section 3](#register-subscription-transports)).

### Origin policy (`CheckOrigin`)

Browsers send an `Origin` header on the upgrade request. A **nil** `CheckOrigin`
accepts **every** origin (handy for localhost, unsafe on the public internet).
Implement an allowlist keyed on `r.Header.Get("Origin")`, mirroring your CORS
policy for `POST` GraphQL:

```go
package main

import (
	"net/http"

	"github.com/grx-gql/grx/websocket"
)

func corsOriginExample() websocket.Config {
	return websocket.Config{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "https://app.example.com"
		},
	}
}

```

### Authenticate on WebSocket connect (`OnConnect`)

For **`graphql-transport-ws`**, the natural place to validate identity is
**once per connection**, inside `websocket.Config.OnConnect`, using the
client’s `connection_init` payload - **not** once per subscription message.

```go
package main

import (
	"context"

	"github.com/grx-gql/grx/websocket"
)

type userKey struct{}

func authenticate(ctx context.Context, token string) (any, error) {
	// Resolve token for your deployment.
	return "subject", nil
}

func subscriptionAuthExample() {
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
}

```

The returned **context** reaches every subscription resolver on that
connection; the map becomes the `connection_ack` payload.

Pair with HTTP middleware patterns from the [auth guide](/guides/auth) so the
same **`context` values** keys are used across `Query` / `Mutation` and
`Subscription` resolvers where possible.

### SSE and HTTP-layer auth

SSE uses normal HTTP requests (`Accept: text/event-stream`). **Cookies**,
**`Authorization`**, and **path-level middleware** behave like any other
handler: authenticate in `http.Handler` wrappers before the request reaches
grx. You do **not** get a separate `connection_init` handshake; plan token
refresh or short-lived streams accordingly.

### Authorization for subscription fields

**Operation** and **field** authorizers wired on the executor still apply to
subscription selections. A client that passed `OnConnect` must still be
allowed to subscribe to each field you expose - reuse the same guards you use
for queries. See **[Authentication & authorization](/guides/auth)**.

### Timeouts and abuse

`websocket.Config` exposes **connection init**, **read/write**, **ping**, and
**`MaxMessageSize`** limits - set them so a single client cannot hold unbounded
memory or CPU (**[`websocket.Config` sample](#register-subscription-transports)**). At the edge, add
**rate limits** and **connection quotas** the same way you would for any
public streaming API; **[Limits](/guides/request-limits)** covers HTTP body
and executor caps that complement subscription transports.

### Fan-out isolation

When using **[pub/sub](/concepts/pubsub)**, keep predicates and topic naming
structured so tenants cannot subscribe to each other’s events - defense in depth
beyond transport auth.

## Cross-resolver fan-out with `pubsub`

The ticker pattern above is fine for demos but not for real workloads.
Most subscriptions stream events that **mutation resolvers** publish
elsewhere  -  e.g. a `messagePosted` subscription that fans out values
produced by a `postMessage` mutation. grx ships
[`memory-pubsub`](/concepts/pubsub) (`package pubsub`) for exactly this  -  use **in-memory** for single-process setups and **`redis-pubsub`** once you scale horizontally; **[Choosing a backend](/concepts/pubsub#choosing-a-backend)** explains the trade-offs.

### Wire a typed bus

```go
package graph

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/grx-gql/grx/memory-pubsub"
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
package graph

import (
	"github.com/grx-gql/grx/memory-pubsub"
	"github.com/grx-gql/grx/schema"
)

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
```

```go
package main

import (
	"log"
	"net/http"
	"time"

	"example.com/hello-grx/graph"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/memory-pubsub"
	"github.com/grx-gql/grx/sse"
	"github.com/grx-gql/grx/websocket"
)

func allowOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "https://app.example.com" || origin == "http://localhost:4000"
}

func main() {
	bus := pubsub.NewMemory()
	defer bus.Close()

	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema(bus)),
		grx.WithTransports(
			websocket.New(websocket.Config{ReadIdleTimeout: 60 * time.Second, CheckOrigin: allowOrigin}),
			sse.New(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = srv
}

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
Redis-backed bus  -  resolver code and `Typed[T]` wiring stay the same
([**memory vs Redis**](/concepts/pubsub#choosing-a-backend)):

```go
package main

import (
	"log"

	redisps "github.com/grx-gql/grx/redis-pubsub"

	rd "github.com/redis/go-redis/v9"
)

func redisBusSnippet() {
	rdb := rd.NewClient(&rd.Options{Addr: "localhost:6379"})
	bus, err := redisps.New(redisps.Config{Client: rdb, Prefix: "chat:"})
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := bus.Close(); err != nil {
			log.Printf("redis pubsub shutdown: %v", err)
		}
	}()

}

```


See **[Pub/Sub](/concepts/pubsub)** (including [**choosing a backend**](/concepts/pubsub#choosing-a-backend)) for the full surface.
