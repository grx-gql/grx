package main

import (
	"log"
	"net/http"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/examples/directives/graph"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Directives example: http://localhost:4004/")
	log.Fatal(http.ListenAndServe(":4004", srv))
}
