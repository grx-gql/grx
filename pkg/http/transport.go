// Package http provides the default GraphQL-over-HTTP+JSON transport.
//
// It implements [core.Transport] for the canonical "POST a JSON body to
// /graphql" wire format described by the GraphQL over HTTP spec. Pair it
// with [pkg/sse] and [pkg/websocket] when subscriptions are needed.
//
// The package name collides with the standard library's net/http; consumers
// that need both will alias one of the imports, e.g.
//
//	import (
//	    "net/http"
//
//	    grxhttp "github.com/patrickkabwe/grx/pkg/http"
//	)
package http

import (
	nethttp "net/http"

	"github.com/patrickkabwe/grx/core"
)

// Config tunes a [Transport]. All fields are optional; the zero value is a
// valid production configuration.
type Config struct{}

// Transport implements [core.Transport] for GraphQL over HTTP+JSON. The
// zero value is ready to use; [New] exists for symmetry with the other
// built-in transports.
type Transport struct {
	config Config
}

// New returns a Transport ready to be registered with the server. An
// optional Config may be supplied to override defaults.
func New(cfg ...Config) *Transport {
	t := &Transport{}
	if len(cfg) > 0 {
		t.config = cfg[0]
	}
	return t
}

// Match reports whether r is a POST request that this transport will
// handle. The server is responsible for filtering by URL path before
// dispatch, so the transport only validates the HTTP method here.
func (Transport) Match(r *nethttp.Request) bool {
	return r.Method == nethttp.MethodPost
}

// Serve decodes the JSON-encoded GraphQL request body, runs it through the
// executor, and writes the response as JSON. Request-level failures
// (invalid JSON, missing query) are surfaced as HTTP 400 with a GraphQL
// error envelope; field-level failures are surfaced as HTTP 200 with the
// errors array populated.
func (Transport) Serve(w nethttp.ResponseWriter, r *nethttp.Request, executor core.Executor) {
	body, err := core.DecodeGraphQLBody(r)
	if err != nil {
		core.WriteJSON(w, nethttp.StatusBadRequest, core.Response{
			Errors: []core.Error{{
				Message: err.Error(),
				Extensions: map[string]any{
					"classification": "request",
				},
			}},
		})
		return
	}

	response := executor.Execute(r.Context(), core.Request{
		Query:         body.Query,
		OperationName: body.OperationName,
		Variables:     body.Variables,
	})
	core.WriteJSON(w, nethttp.StatusOK, response)
}
