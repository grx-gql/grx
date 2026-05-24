package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/cors"
	"github.com/grx-gql/grx/examples/subscriptions/graph"
	"github.com/grx-gql/grx/memory-pubsub"
	"github.com/grx-gql/grx/plugins/logger"
	"github.com/grx-gql/grx/sse"
	"github.com/grx-gql/grx/websocket"
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
