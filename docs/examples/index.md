---
title: Examples
description: Copy-paste starters, subscription wiring, router mounts—with the full Go source on the page.
outline: deep
---

# Examples

In this section you’ll find **the same Go that ships in `examples/`**—ready to skim, paste, or tweak—plus **tiny router mounts** for production-style URLs.

::: tip Runs from repo root  
The commands assume your working directory is a clone of **`github.com/patrickkabwe/grx`**. Prefer your own module path when you copy snippets into another module (everything under **`examples/*/`** is importable paths from this repo).  
:::

---

## Starter servers {#examples-starters}

### Basic GraphQL HTTP server {#examples-basic-http}

GraphiQL at **`GET /`**, **`POST /graphql`** for queries and mutations—the smallest complete loop.

#### `examples/basic/graph/schema.go`

```go
package graph

import "github.com/patrickkabwe/grx/schema"

type Query struct{}

func NewSchema() schema.Config {
	return schema.Config{
		Query: Query{},
	}
}
```

#### `examples/basic/graph/user.go`

```go
package graph

import (
	"context"
)

type User struct {
	ID    string  `gql:"id,nonNull"`
	Name  string  `gql:"name,nonNull"`
	Email *string `gql:"email"`
}

type UserArgs struct {
	ID string `gql:"id,nonNull"`
}

func (Query) User(ctx context.Context, args UserArgs) (*User, error) {
	email := "ada@example.com"
	return &User{ID: args.ID, Name: "Ada Lovelace", Email: &email}, nil
}
```

#### `examples/basic/main.go`

```go
package main

import (
	"log"
	"net/http"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/examples/basic/graph"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("GraphQL playground: http://localhost:4000/")
	log.Println("GraphQL endpoint: POST http://localhost:4000/graphql")
	log.Fatal(http.ListenAndServe(":4000", srv))
}
```

```bash
go run ./examples/basic
```

Try inside GraphiQL:

```graphql
{
  user(id: "user_1") {
    id
    name
    email
  }
}
```

**Next concepts:** [**Get started**](/getting-started/) · [**Queries and mutations**](/guides/query-mutation-server)

---

### Bearer token + field authorizer {#examples-auth}

Shows how **`WithMiddleware`** turns `Authorization: Bearer …` into a request-scoped **subject**, and how **`WithFieldAuthorizer`** blocks `viewer` until that subject exists—the same narrative as [**Authentication & authorization**](/guides/auth). Runs on **`http://localhost:4010/`** with playground at **`/`**.

#### `examples/auth/session/context.go`

```go
package session

import (
	"context"
)

type subjectKey struct{}

func ContextWithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subject)
}

func Subject(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(subjectKey{}).(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}
```

#### `examples/auth/graph/schema.go`

```go
package graph

import (
	"context"
	"errors"

	"github.com/patrickkabwe/grx/examples/auth/session"
	"github.com/patrickkabwe/grx/schema"
)

type Query struct{}

type User struct {
	ID          string `gql:"id,nonNull"`
	DisplayName string `gql:"displayName,nonNull"`
}

func NewSchema() schema.Config {
	return schema.Config{
		Query: Query{},
	}
}

func (Query) Ping() string {
	return "ok"
}

func (Query) Viewer(ctx context.Context) (*User, error) {
	sub, ok := session.Subject(ctx)
	if !ok {
		return nil, errors.New("unauthenticated")
	}
	return &User{ID: sub, DisplayName: "Signed-in subject " + sub}, nil
}
```

#### `examples/auth/main.go`

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/examples/auth/graph"
	"github.com/patrickkabwe/grx/examples/auth/session"
)

const demoBearerToken = "demo-alice-token"

func parseBearer(header string) (token string, ok bool) {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], true
}

func bearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		token, hasBearerScheme := parseBearer(r.Header.Get("Authorization"))
		if hasBearerScheme {
			switch token {
			case demoBearerToken:
				ctx = session.ContextWithSubject(ctx, "alice")
			case "":
				http.Error(w, `invalid Authorization header`, http.StatusBadRequest)
				return
			default:
				http.Error(w, `invalid bearer token`, http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireViewerField() grx.FieldAuthorizer {
	return func(ctx context.Context, fc grx.FieldAuthorizationContext) error {
		if fc.ParentType != "Query" || fc.FieldName != "viewer" {
			return nil
		}
		if _, ok := session.Subject(ctx); !ok {
			return fmt.Errorf("viewer requires Authorization: Bearer %s", demoBearerToken)
		}
		return nil
	}
}

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/"),
		grx.WithMiddleware(bearerAuth),
		grx.WithFieldAuthorizer(requireViewerField()),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(`GraphQL playground: http://localhost:4010/`)
	log.Println(`For { viewer { id } } add header Authorization: Bearer ` + demoBearerToken)
	log.Fatal(http.ListenAndServe(":4010", srv))
}
```

```bash
go run ./examples/auth
```

GraphiQL **Headers** pane:

```json
{ "Authorization": "Bearer demo-alice-token" }
```

Then query:

```graphql
{
  viewer {
    id
    displayName
  }
}
```

**Guide:** [**Authentication & authorization**](/guides/auth)

---

### Subscriptions (WebSocket + SSE) {#examples-subscriptions}

Memory pub/sub feeds [**`graphql-transport-ws`**](https://github.com/graphql/graphql-ws)—plus bundled SSE—with CORS guarded for **`localhost`** below.

#### `examples/subscriptions/main.go`

```go
package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/examples/subscriptions/graph"
	"github.com/patrickkabwe/grx/pkg/cors"
	"github.com/patrickkabwe/grx/pkg/pubsub"
	"github.com/patrickkabwe/grx/pkg/sse"
	"github.com/patrickkabwe/grx/pkg/websocket"
	"github.com/patrickkabwe/grx/plugin/logger"
)

const (
	port           = "4000"
	playgroundPath = "/"
)

func main() {
	loggerPlugin, err := logger.New(logger.Config{
		Logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		log.Fatal(err)
	}

	bus := pubsub.NewMemory()
	defer bus.Close()

	srv, err := grx.NewServer(
		grx.WithSchema(graph.New(graph.WithPubSub(bus))),
		grx.WithPlugins(loggerPlugin),
		grx.WithPlaygroundPath(playgroundPath),
		grx.WithMiddleware(cors.New(cors.Config{
			AllowedOrigins: []string{
				"http://localhost:" + port,
				"http://127.0.0.1:" + port,
			},
			AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			MaxAge:         10 * time.Minute,
		})),
		grx.WithTransports(
			websocket.New(),
			sse.New(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Server is running on port ", port)
	log.Println("GraphQL Playground is available at http://localhost:" + port + srv.PlaygroundPath)
	log.Println("GraphQL HTTP (queries/mutations): POST http://localhost:" + port + srv.GraphqlPath)
	log.Println("Subscriptions (WebSocket/SSE): http://localhost:" + port + srv.SubscriptionPath)
	log.Fatal(http.ListenAndServe(":"+port, srv))
}
```

#### `examples/subscriptions/graph/schema.go`

```go
package graph

import (
	"github.com/patrickkabwe/grx/pkg/pubsub"
	"github.com/patrickkabwe/grx/schema"
)

// SchemaOption configures [New].
type SchemaOption func(*schemaOptions)

type schemaOptions struct {
	bus pubsub.PubSub
}

// WithPubSub wires the typed pub/sub bridge used by mutation and subscription
// resolvers. Required for this example schema, which publishes across
// resolvers via [pkg/pubsub].
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
```

#### `examples/subscriptions/graph/user.go`

```go
package graph

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/patrickkabwe/grx/pkg/pubsub"
)

// userCreatedTopic is the bus topic on which User events are published.
// Centralising it here avoids drift between publisher and subscriber.
const userCreatedTopic = "user.created"

type User struct {
	ID    string  `gql:"id,nonNull"`
	Name  string  `gql:"name,nonNull"`
	Email *string `gql:"email"`
}

type UserArgs struct {
	ID string `gql:"id,nonNull"`
}

type UserCreateInput struct {
	Name  string  `gql:"name,nonNull"`
	Email *string `gql:"email"`
}

type UserCreateArgs struct {
	Input UserCreateInput `gql:"input,nonNull"`
}

type UserCreatePayload struct {
	User *User `gql:"user,nonNull"`
}

// UserQuery groups every read-side resolver for the User entity.
type UserQuery struct{}

func (UserQuery) User(ctx context.Context, args UserArgs) (*User, error) {
	email := "ada@example.com"
	return &User{ID: args.ID, Name: "Ada Lovelace", Email: &email}, nil
}

func (UserQuery) ErrorExample(ctx context.Context) (string, error) {
	return "", fmt.Errorf("example error from basic server")
}

// UserMutation groups every write-side resolver for the User entity.
// The Bus dependency is wired by NewSchema so the mutation can publish
// domain events that subscriptions consume.
type UserMutation struct {
	Bus    *pubsub.Typed[*User]
	nextID atomic.Uint64
}

func (m *UserMutation) CreateUser(ctx context.Context, args UserCreateArgs) (*UserCreatePayload, error) {
	id := m.nextID.Add(1)
	user := &User{
		ID:    fmt.Sprintf("user_%d", id),
		Name:  args.Input.Name,
		Email: args.Input.Email,
	}
	if m.Bus != nil {
		if err := m.Bus.Publish(ctx, userCreatedTopic, user); err != nil {
			return nil, err
		}
	}
	return &UserCreatePayload{User: user}, nil
}

// UserSubscription groups every stream resolver for the User entity. The
// Bus dependency is wired by NewSchema; UserCreated relays User events
// published by CreateUser to every active GraphQL subscription.
type UserSubscription struct {
	Bus *pubsub.Typed[*User]
}

func (s UserSubscription) UserCreated(ctx context.Context) (<-chan *User, error) {
	return s.Bus.Subscribe(ctx, userCreatedTopic)
}
```

Post feeds the root **`Query`**, while **`message.go`** publishes room-scoped payloads with a **`Subscribe`** predicate—both match the **`user.go`** pattern (`embed` structs on each root).

#### `examples/subscriptions/graph/post.go`

```go
package graph

import "context"

type Post struct {
	ID     string `gql:"id,nonNull"`
	Title  string `gql:"title,nonNull"`
	Body   string `gql:"body,nonNull"`
	Author *User  `gql:"author,nonNull"`
}

type PostArgs struct {
	ID string `gql:"id,nonNull"`
}

type PostCreateInput struct {
	Title    string `gql:"title,nonNull"`
	Body     string `gql:"body,nonNull"`
	AuthorID string `gql:"authorId,nonNull"`
}

type PostCreateArgs struct {
	Input PostCreateInput `gql:"input,nonNull"`
}

type PostCreatePayload struct {
	Post *Post `gql:"post,nonNull"`
}

type PostQuery struct{}

func (PostQuery) Post(ctx context.Context, args PostArgs) (*Post, error) {
	email := "ada@example.com"
	return &Post{
		ID:     args.ID,
		Title:  "Hello, grx",
		Body:   "Composing root types from per-entity resolver structs.",
		Author: &User{ID: "user_1", Name: "Ada Lovelace", Email: &email},
	}, nil
}

type PostMutation struct{}

func (PostMutation) CreatePost(ctx context.Context, args PostCreateArgs) (*PostCreatePayload, error) {
	post := &Post{
		ID:     "post_1",
		Title:  args.Input.Title,
		Body:   args.Input.Body,
		Author: &User{ID: args.Input.AuthorID, Name: "Ada Lovelace"},
	}
	return &PostCreatePayload{Post: post}, nil
}
```

#### `examples/subscriptions/graph/message.go`

```go
package graph

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

type PostMessageArgs struct {
	Input PostMessageInput `gql:"input,nonNull"`
}

type PostMessagePayload struct {
	Message *Message `gql:"message,nonNull"`
}

type MessagePostedArgs struct {
	RoomID string `gql:"roomId,nonNull"`
}

type MessageMutation struct {
	Bus    *pubsub.Typed[*Message]
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
		if err := m.Bus.Publish(ctx, messagePostedTopic, message); err != nil {
			return nil, err
		}
	}
	return &PostMessagePayload{Message: message}, nil
}

type MessageSubscription struct {
	Bus *pubsub.Typed[*Message]
}

func (s MessageSubscription) MessagePosted(ctx context.Context, args MessagePostedArgs) (<-chan *Message, error) {
	if strings.TrimSpace(args.RoomID) == "" {
		return nil, fmt.Errorf("roomId is required")
	}
	return s.Bus.Subscribe(ctx, messagePostedTopic, func(m *Message) bool {
		return m != nil && m.RoomID == args.RoomID
	})
}
```

```bash
go run ./examples/subscriptions
```

**Dig deeper:** [**Realtime subscriptions**](/guides/subscriptions)

---

## Router integrations {#examples-router-integrations}

`grx` answers **`ServeHTTP`** with exact path matches—you usually pair **`http.StripPrefix("/api", gql)`** with **`WithPlaygroundPath("/playground")`** and **`WithGraphQLPath("/query")`** so callers see **`/api/playground`** and **`POST /api/query`**.

Build **`graph`** any way you like (see **[Get started**](/getting-started/) or **Basic HTTP** above), then reuse this fragment before each snippet:

```go
	gql, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/playground"),
		grx.WithGraphQLPath("/query"),
	)
	if err != nil {
		log.Fatal(err)
	}
	delegated := http.StripPrefix("/api", gql)
```

**Long-form guides:** [**net/http ServeMux**](/getting-started/servemux) · [**Chi**](/getting-started/chi) · [**Gin**](/getting-started/gin) · [**Echo**](/getting-started/echo) · [**Fiber**](/getting-started/fiber)

The **Gin**, **Echo**, and **Fiber** excerpts below reuse **`delegated`** from the fragment above. **Chi** repeats the full **`grx.NewServer`** block for copy-paste convenience.

---

### net/http `ServeMux` {#examples-router-servemux}

```go
mux := http.NewServeMux()
mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("ok"))
}))
mux.Handle("/api/", delegated)
```

---

### Chi {#examples-router-chi}

```bash
go get github.com/go-chi/chi/v5
```

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"
	"github.com/go-chi/chi/v5"
	"github.com/patrickkabwe/grx"
)

func main() {
	gql, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/playground"),
		grx.WithGraphQLPath("/query"),
	)
	if err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Mount("/api", http.StripPrefix("/api", gql))

	log.Fatal(http.ListenAndServe(":8080", r))
}
```

---

### Gin {#examples-router-gin}

```bash
go get github.com/gin-gonic/gin
```

```go
	engine := gin.Default()
	engine.GET("/health", func(c *gin.Context) { c.String(200, "ok") })
	engine.Any("/api/*proxyPath", gin.WrapH(delegated))
	log.Fatal(engine.Run(":8080"))
```

---

### Echo {#examples-router-echo}

```bash
go get github.com/labstack/echo/v4
```

```go
	e := echo.New()
	e.GET("/health", func(c echo.Context) error { return c.String(200, "ok") })
	e.Any("/api/*", echo.WrapHandler(delegated))
	log.Fatal(e.Start(":8080"))
```

---

### Fiber {#examples-router-fiber}

```bash
go get github.com/gofiber/fiber/v2
```

```go
	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.All("/api/*", adaptor.HTTPHandler(delegated))
	log.Fatal(app.Listen(":8080"))
```

(import **`github.com/gofiber/fiber/v2`** and **`github.com/gofiber/fiber/v2/middleware/adaptor`**)

---

## Measuring throughput {#examples-benchmarks}

Micro-benchmarks help compare resolver hot paths—they are **not** a substitute for profiling your full HTTP/RPS workload.

See **[Benchmarks](/benchmarks)** for scenarios, methodology, and latest tables.

---

## Source {#examples-source}

- [**GitHub repository**](https://github.com/patrickkabwe/grx) — history, CI, **`examples/`** tree mirroring these snippets.
- [**Discussions**](https://github.com/patrickkabwe/grx/discussions) — integrations, roadmap questions.

---

## Doc cross-links {#examples-more-docs}

- [**Define your schema**](/concepts/schema-basics)
- [**Organize larger codebases**](/concepts/schema-mapping)
- [**Authentication & authorization**](/guides/auth)
- [**Custom transports**](/guides/custom-transport) & [**Plugins**](/guides/custom-plugin)
- [**How it fits together**](/concepts/architecture)
