package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/pkg/pubsub"
	"github.com/patrickkabwe/grx/pkg/sse"
	"github.com/patrickkabwe/grx/pkg/websocket"
	"github.com/patrickkabwe/grx/examples/basic/graph"
	"github.com/patrickkabwe/grx/plugin"
	"github.com/patrickkabwe/grx/plugin/logger"
	"github.com/patrickkabwe/grx/server"
)

var port = "4000"
var playgroundPath = "/"

func main() {
	loggerPlugin, err := logger.New(logger.Config{
		Logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		log.Fatal(err)
	}

	bus := pubsub.NewMemory()
	defer bus.Close()

	srv, err := server.New(server.Config{
		Schema:         graph.NewSchema(bus),
		Plugins:        []plugin.Plugin{loggerPlugin},
		PlaygroundPath: playgroundPath,
		Transports:     []core.Transport{websocket.New(), sse.New()},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Server is running on port ", port)
	log.Println("GraphQL Playground is available at http://localhost:" + port + playgroundPath)
	log.Fatal(http.ListenAndServe(":"+port, srv))
}
