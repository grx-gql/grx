package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/examples/subscriptions/graph"
	"github.com/grx-gql/grx/plugins/logger"
	"github.com/grx-gql/grx/pubsub/memory"
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

	bus := memory.New()
	defer bus.Close()

	srv, err := grx.NewServer(
		grx.WithSchema(graph.New(graph.WithEvents(graph.Events{
			Bus: bus,
		}))),
		grx.WithPlugins(loggerPlugin),
		grx.WithPlaygroundPath(playgroundPath),
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
