package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Transport plugs a GraphQL protocol (WebSocket subscriptions, SSE,
// HTTP+JSON, batching, ...) into the server. The server iterates registered
// transports in order; the first one whose Match returns true takes ownership
// of the response.
type Transport interface {
	// Match reports whether this transport wants to handle the given request.
	// It must be cheap and side-effect free.
	Match(r *http.Request) bool

	// Serve writes the protocol-specific response. It is only called after
	// Match returned true.
	Serve(w http.ResponseWriter, r *http.Request, executor Executor)
}

// GraphQLBody is the canonical wire shape for an HTTP-borne GraphQL request.
// Transports decode payloads into this struct before invoking the Executor.
type GraphQLBody struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

// DecodeGraphQLBody parses a JSON body into a GraphQLBody and returns a
// human-readable error suitable for surfacing as a request-level GraphQL
// error.
func DecodeGraphQLBody(r *http.Request) (GraphQLBody, error) {
	var body GraphQLBody
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		return body, fmt.Errorf("invalid GraphQL JSON body: %s", err.Error())
	}
	if body.Query == "" {
		return body, fmt.Errorf("missing GraphQL query")
	}
	return body, nil
}

// WriteJSON writes value as a JSON response with the given status. It is
// shared by transports that need to surface request-level errors before any
// streaming has started.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// HeaderContains reports whether any of values contains needle as a
// case-insensitive, comma-separated token. Useful for parsing request
// headers like Connection or Accept.
func HeaderContains(values []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			if strings.EqualFold(strings.TrimSpace(part), needle) {
				return true
			}
		}
	}
	return false
}
