package main

import (
	"log"
	"net/http"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/examples/file-upload/graph"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("File upload example: http://localhost:4005/")
	log.Println("GraphQL endpoint: POST http://localhost:4005/graphql")
	log.Fatal(http.ListenAndServe(":4005", srv))
}
