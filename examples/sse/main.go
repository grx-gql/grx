package main

import (
	"log"
	"net/http"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/examples/sse/graph"
	"github.com/grx-gql/grx/pubsub/memory"
	"github.com/grx-gql/grx/sse"
)

const port = "4000"

func main() {
	bus := memory.New()
	defer bus.Close()

	// sse.New() is the only subscription transport, so subscriptions are
	// plain HTTP requests answered with a text/event-stream response. The
	// bundled playground detects this and subscribes over SSE as well.
	srv, err := grx.NewServer(
		grx.WithSchema(graph.Schema(bus)),
		grx.WithPlaygroundPath("/"),
		grx.WithTransports(sse.New()),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("GraphQL playground: http://localhost:" + port + srv.PlaygroundPath)
	log.Println("GraphQL endpoint: POST http://localhost:" + port + srv.GraphqlPath)
	log.Println("Subscriptions (SSE): POST http://localhost:" + port + srv.SubscriptionPath + " with Accept: text/event-stream")
	log.Fatal(http.ListenAndServe(":"+port, srv))
}
