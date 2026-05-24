package main

import (
	"log"
	"net/http"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/examples/basic/graph"
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
