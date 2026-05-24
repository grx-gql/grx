package main

import (
	"log"
	"net/http"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/examples/enums/graph"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Enum example: http://localhost:4002/")
	log.Fatal(http.ListenAndServe(":4002", srv))
}
