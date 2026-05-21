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
	// Extensions carries transport-specific metadata (for example automatic
	// persisted query hashes). Most callers leave it unset.
	Extensions map[string]any `json:"extensions"`
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

// DecodeGraphQLRequest parses a GraphQL request from an HTTP request. POST
// requests use a JSON body; GET requests use query, variables, and
// operationName URL parameters per GraphQL-over-HTTP.
func DecodeGraphQLRequest(r *http.Request) (GraphQLBody, error) {
	switch r.Method {
	case http.MethodPost:
		return DecodeGraphQLBody(r)
	case http.MethodGet:
		q := r.URL.Query()
		body := GraphQLBody{
			Query:         q.Get("query"),
			OperationName: q.Get("operationName"),
		}
		if body.Query == "" {
			return body, fmt.Errorf("missing GraphQL query")
		}
		if raw := q.Get("variables"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &body.Variables); err != nil {
				return body, fmt.Errorf("invalid GraphQL variables: %s", err.Error())
			}
		}
		return body, nil
	default:
		return GraphQLBody{}, fmt.Errorf("unsupported HTTP method %s", r.Method)
	}
}

// WriteGraphQLResponse writes value using the GraphQL-over-HTTP response
// media type. charset=utf-8 is appended when not already present.
func WriteGraphQLResponse(w http.ResponseWriter, status int, mediaType string, value any) {
	if !strings.Contains(mediaType, "charset=") {
		mediaType += "; charset=utf-8"
	}
	w.Header().Set("Content-Type", mediaType)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// WriteJSON writes value as a legacy application/json GraphQL response.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	WriteGraphQLResponse(w, status, MediaTypeJSON, value)
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
