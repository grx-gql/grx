package main

import (
	"log"
	"net/http"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/examples/scalars/graph"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Custom scalar example: http://localhost:4001/")
	log.Fatal(http.ListenAndServe(":4001", srv))
}
