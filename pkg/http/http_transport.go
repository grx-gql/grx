// Package http provides the default GraphQL-over-HTTP+JSON transport.
//
// It implements [core.Transport] for GraphQL-over-HTTP: POST with a JSON
// body and GET with query, variables, and operationName URL parameters.
// Pair it
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
	"compress/gzip"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"strings"

	"github.com/patrickkabwe/grx/core"
)

// DefaultMaxRequestBytes is applied when [Config.MaxRequestBytes] is unset
// and a positive limit is enabled via server wiring. A zero
// [Config.MaxRequestBytes] keeps the transport unlimited for backward
// compatibility.
const DefaultMaxRequestBytes = 1 << 20 // 1 MiB

// Config tunes a [Transport]. All fields are optional; the zero value is a
// valid production configuration.
type Config struct {
	// Path restricts [Transport.Match] to requests whose URL path
	// equals Path. Empty Path keeps the legacy behaviour ("Match only inspects
	// the HTTP method"); callers that mount the GraphQL POST handler on an
	// arbitrary path should set Path accordingly.
	Path string
	// MaxRequestBytes limits POST JSON bodies and GET query, variables, and
	// operationName parameter bytes combined. Zero disables the limit; a
	// negative value also disables it.
	MaxRequestBytes int64
	// EnableGzip compresses JSON GraphQL responses when the client sends
	// Accept-Encoding: gzip. Small payloads stay uncompressed.
	EnableGzip bool
}

func (c Config) maxRequestBytes() int64 {
	if c.MaxRequestBytes < 0 {
		return 0
	}
	return c.MaxRequestBytes
}

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

// Match reports whether r is a GraphQL-over-HTTP request this transport will
// handle: POST with a JSON body, or GET with a non-empty query parameter.
// When [Config.Path] is non-empty, the request path must match it.
func (t *Transport) Match(r *nethttp.Request) bool {
	switch r.Method {
	case nethttp.MethodPost:
	case nethttp.MethodGet:
		if strings.TrimSpace(r.URL.Query().Get("query")) == "" {
			return false
		}
	default:
		return false
	}
	if p := strings.TrimSpace(t.config.Path); p != "" && r.URL.Path != p {
		return false
	}
	return true
}

// Serve decodes the GraphQL request, runs it through the executor, and writes
// the response as JSON. Request-level failures (invalid JSON, missing query,
// oversize payload) are surfaced as HTTP 4xx with a GraphQL error envelope;
// field-level failures are surfaced as HTTP 200 with the errors array
// populated.
func (t *Transport) Serve(w nethttp.ResponseWriter, r *nethttp.Request, executor core.Executor) {
	responseType := core.MediaTypeJSON
	defer func() {
		if rec := recover(); rec != nil {
			msg := fmt.Sprintf("panic: %v", rec)
			core.WriteGraphQLResponse(w, nethttp.StatusInternalServerError, responseType, core.Response{
				Errors: []core.Error{{
					Message: msg,
					Extensions: map[string]any{
						"classification": "request",
					},
				}},
			})
		}
	}()

	responseType, ok := core.NegotiateResponseContentType(r.Header.Values("Accept"))
	if !ok {
		writeRequestError(w, core.MediaTypeJSON, nethttp.StatusNotAcceptable, fmt.Errorf("no supported response media type in Accept header"))
		return
	}

	if r.Method == nethttp.MethodPost {
		if err := core.ValidatePostContentType(r); err != nil {
			writeRequestError(w, responseType, nethttp.StatusUnsupportedMediaType, err)
			return
		}
	}

	if max := t.config.maxRequestBytes(); max > 0 {
		if err := limitRequestSize(w, r, max); err != nil {
			writeRequestError(w, responseType, nethttp.StatusRequestEntityTooLarge, err)
			return
		}
	}

	body, err := core.DecodeGraphQLRequest(r)
	if err != nil {
		status := nethttp.StatusBadRequest
		if t.config.maxRequestBytes() > 0 && requestBodyTooLarge(err) {
			status = nethttp.StatusRequestEntityTooLarge
		}
		writeRequestError(w, responseType, status, err)
		return
	}

	gqlReq := core.Request{
		Query:         body.Query,
		OperationName: body.OperationName,
		Variables:     body.Variables,
	}

	if kind, kindErr := executor.OperationKind(gqlReq); kindErr == nil {
		switch r.Method {
		case nethttp.MethodGet:
			if kind == core.OperationMutation || kind == core.OperationSubscription {
				w.Header().Set("Allow", nethttp.MethodPost)
				writeRequestError(w, responseType, nethttp.StatusMethodNotAllowed, fmt.Errorf("HTTP GET cannot execute GraphQL %s operations", kind))
				return
			}
		case nethttp.MethodPost:
			if kind == core.OperationSubscription {
				writeRequestError(w, responseType, nethttp.StatusMethodNotAllowed, fmt.Errorf("GraphQL subscription operations are not supported over HTTP POST; use WebSocket or SSE"))
				return
			}
		}
	}

	response := executor.Execute(r.Context(), gqlReq)
	t.writeGraphQLPayload(w, r, nethttp.StatusOK, responseType, response)
}

func gzipAccepted(r *nethttp.Request) bool {
	return core.HeaderContains(r.Header.Values("Accept-Encoding"), "gzip")
}

func (t *Transport) writeGraphQLPayload(w nethttp.ResponseWriter, r *nethttp.Request, status int, mediaType string, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	contentType := mediaType
	if !strings.Contains(contentType, "charset=") {
		contentType += "; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	if t.config.EnableGzip && gzipAccepted(r) && len(encoded) >= 64 {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.WriteHeader(status)
		zw := gzip.NewWriter(w)
		if _, err := zw.Write(encoded); err != nil {
			_ = zw.Close()
			return
		}
		_ = zw.Close()
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}

func limitRequestSize(w nethttp.ResponseWriter, r *nethttp.Request, max int64) error {
	switch r.Method {
	case nethttp.MethodPost:
		if r.ContentLength > max {
			return fmt.Errorf("request exceeds %d byte limit", max)
		}
		r.Body = nethttp.MaxBytesReader(w, r.Body, max)
		return nil
	case nethttp.MethodGet:
		q := r.URL.Query()
		size := int64(len(q.Get("query")) + len(q.Get("variables")) + len(q.Get("operationName")))
		if size > max {
			return fmt.Errorf("request exceeds %d byte limit", max)
		}
		return nil
	default:
		return nil
	}
}

func requestBodyTooLarge(err error) bool {
	return strings.Contains(err.Error(), "request body too large")
}

func writeRequestError(w nethttp.ResponseWriter, mediaType string, status int, err error) {
	core.WriteGraphQLResponse(w, status, mediaType, core.Response{
		Errors: []core.Error{{
			Message: err.Error(),
			Extensions: map[string]any{
				"classification": "request",
			},
		}},
	})
}
