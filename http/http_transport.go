// Package http provides the default GraphQL-over-HTTP+JSON transport.
//
// It implements [core.Transport] for GraphQL-over-HTTP: POST with a JSON
// body and GET with query, variables, and operationName URL parameters.
// Pair it with the module's [`sse`](https://pkg.go.dev/github.com/grx-gql/grx/sse) and [`websocket`](https://pkg.go.dev/github.com/grx-gql/grx/websocket) transports when subscriptions are needed.
//
// The package name collides with the standard library's net/http; consumers
// that need both will alias one of the imports, e.g.
//
//	import (
//	    "net/http"
//
//	    grxhttp "github.com/grx-gql/grx/http"
//	)
package http

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	nethttp "net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/grx-gql/grx/core"
)

// Pool JSON encode scratch buffers sized for typical GraphQL JSON responses so
// we avoid reallocating intermediate []byte slabs on each request hot path.
var jsonEncodeBufferPool = sync.Pool{
	New: func() any {
		buf := bytes.NewBuffer(nil)
		buf.Grow(2048)
		return buf
	},
}

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
	// operationName and extensions parameter bytes combined. Zero disables the limit; a
	// negative value also disables it.
	MaxRequestBytes int64
	// EnableGzip compresses JSON GraphQL responses when the client sends
	// Accept-Encoding: gzip. Small payloads stay uncompressed.
	EnableGzip bool
	// EnableBrotli compresses JSON GraphQL responses with Brotli when the
	// client sends Accept-Encoding: br. When both EnableBrotli and EnableGzip
	// are set and the client accepts both, Brotli is preferred. Small payloads
	// stay uncompressed.
	EnableBrotli bool
	// PersistedQueries maps lowercase SHA-256 hex digests (64 characters) to
	// GraphQL query strings for automatic persisted query (APQ) requests that
	// send an empty "query" and a hash under extensions.persistedQuery.
	PersistedQueries map[string]string
	// APQCacheSize enables automatic registration and bounds cached query count.
	// Zero disables registration. Entries are evicted in insertion order; each
	// query is limited to 64 KiB. Static PersistedQueries remain independent.
	APQCacheSize int
	apq          *persistedQueryCache
	// RequirePersistedQuery rejects requests that do not include
	// extensions.persistedQuery.sha256Hash.
	RequirePersistedQuery bool
	// StrictPersistedQueries verifies the APQ version and, when a query is sent
	// with a hash, checks that the SHA-256 hash matches the query bytes.
	StrictPersistedQueries bool
	// MaxVariableBytes limits the JSON-encoded variables payload. Zero disables
	// the limit.
	MaxVariableBytes int64
	// PanicErrorMessage is returned for transport-boundary panics. Empty uses a
	// generic message.
	PanicErrorMessage string
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
		if t.config.APQCacheSize > 0 {
			t.config.apq = newPersistedQueryCache(t.config.APQCacheSize)
		}
		if t.config.PersistedQueries != nil {
			norm := make(map[string]string, len(t.config.PersistedQueries))
			for k, v := range t.config.PersistedQueries {
				norm[strings.ToLower(strings.TrimSpace(k))] = v
			}
			t.config.PersistedQueries = norm
		}
	}
	return t
}

// Match reports whether r is a GraphQL-over-HTTP request this transport will
// handle: POST with a JSON body, or GET with query or extensions parameters.
// When [Config.Path] is non-empty, the request path must match it.
func (t *Transport) Match(r *nethttp.Request) bool {
	switch r.Method {
	case nethttp.MethodPost:
	case nethttp.MethodGet:
		parameters := r.URL.Query()
		if strings.TrimSpace(parameters.Get("query")) == "" && strings.TrimSpace(parameters.Get("extensions")) == "" {
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
			msg := strings.TrimSpace(t.config.PanicErrorMessage)
			if msg == "" {
				msg = "internal server error"
			}
			core.WriteGraphQLResponse(w, nethttp.StatusInternalServerError, responseType, core.Response{
				Errors: []core.Error{core.NewRequestError(fmt.Errorf("%s", msg))},
			})
		}
	}()

	responseType, jsonAccepted := core.NegotiateResponseContentType(r.Header.Values("Accept"))
	multipartAccepted := acceptsMultipartMixed(r)
	if !jsonAccepted && !multipartAccepted {
		writeRequestError(r.Context(), w, core.MediaTypeJSON, nethttp.StatusNotAcceptable, fmt.Errorf("no supported response media type in Accept header"))
		return
	}

	if !jsonAccepted {
		responseType = core.MediaTypeJSON
	}

	if r.Method == nethttp.MethodPost && !core.IsMultipartRequest(r) {
		if err := core.ValidatePostContentType(r); err != nil {
			writeRequestError(r.Context(), w, responseType, nethttp.StatusUnsupportedMediaType, err)
			return
		}
	}

	if max := t.config.maxRequestBytes(); max > 0 {
		if err := limitRequestSize(w, r, max); err != nil {
			writeRequestError(r.Context(), w, responseType, nethttp.StatusRequestEntityTooLarge, err)
			return
		}
	}

	decoded, err := decodeGraphQLHTTP(r, t.config)
	if err != nil {
		status := nethttp.StatusBadRequest
		if t.config.maxRequestBytes() > 0 && requestBodyTooLarge(err) {
			status = nethttp.StatusRequestEntityTooLarge
		}
		writeRequestError(r.Context(), w, responseType, status, err)
		return
	}

	bodies := decoded.bodies
	if !decoded.batch {
		gqlReq := core.Request{
			Query:         bodies[0].Query,
			OperationName: bodies[0].OperationName,
			Variables:     bodies[0].Variables,
		}

		if kind, kindErr := executor.OperationKind(gqlReq); kindErr == nil {
			switch r.Method {
			case nethttp.MethodGet:
				if kind == core.OperationMutation || kind == core.OperationSubscription {
					w.Header().Set("Allow", nethttp.MethodPost)
					writeRequestError(r.Context(), w, responseType, nethttp.StatusMethodNotAllowed, fmt.Errorf("HTTP GET cannot execute GraphQL %s operations", kind))
					return
				}
			case nethttp.MethodPost:
				if kind == core.OperationSubscription {
					writeRequestError(r.Context(), w, responseType, nethttp.StatusMethodNotAllowed, fmt.Errorf("GraphQL subscription operations are not supported over HTTP POST; use WebSocket or SSE"))
					return
				}
			}
		}

		if ie, ok := executor.(core.IncrementalExecutor); ok && multipartAccepted && ie.HasIncrementalDirectives(gqlReq) {
			t.writeIncremental(w, r, ie, gqlReq)
			return
		}

		if !jsonAccepted {
			writeRequestError(r.Context(), w, responseType, nethttp.StatusNotAcceptable, fmt.Errorf("operation does not support the requested multipart response"))
			return
		}

		response := executor.Execute(r.Context(), gqlReq)
		t.writeGraphQLPayload(w, r, responseStatus(responseType, response), responseType, response)
		return
	}

	if !jsonAccepted {
		writeRequestError(r.Context(), w, responseType, nethttp.StatusNotAcceptable, fmt.Errorf("batched operations require a supported JSON response media type"))
		return
	}
	responses := make([]core.Response, len(bodies))
	for i := range bodies {
		gqlReq := core.Request{
			Query:         bodies[i].Query,
			OperationName: bodies[i].OperationName,
			Variables:     bodies[i].Variables,
		}
		kind, kindErr := executor.OperationKind(gqlReq)
		if kindErr != nil {
			responses[i] = core.Response{Errors: []core.Error{core.NewRequestError(kindErr)}}
			continue
		}
		if kind == core.OperationMutation || kind == core.OperationSubscription {
			responses[i] = core.Response{Errors: []core.Error{core.NewRequestError(
				fmt.Errorf("GraphQL %s operations are not supported in batched HTTP requests", kind),
			)}}
			continue
		}
		responses[i] = executor.Execute(r.Context(), gqlReq)
	}
	t.writeGraphQLBatch(w, r, nethttp.StatusOK, responseType, responses)
}

func responseStatus(mediaType string, response core.Response) int {
	if mediaType == core.MediaTypeGraphQLResponse && response.Data == nil && !response.DataNull && len(response.Errors) > 0 {
		return nethttp.StatusBadRequest
	}
	return nethttp.StatusOK
}

// negotiateEncoding selects the highest accepted quality, preferring Brotli on
// ties. An explicit coding overrides the wildcard even when its quality is zero.
// The second result reports whether an uncompressed response is acceptable.
func (t *Transport) negotiateEncoding(r *nethttp.Request) (string, bool) {
	if len(r.Header.Values("Accept-Encoding")) == 0 {
		return "", true
	}
	gzipQuality, brotliQuality, wildcard, identity := -1.0, -1.0, -1.0, -1.0
	for _, value := range r.Header.Values("Accept-Encoding") {
		for _, part := range strings.Split(value, ",") {
			coding, params, err := mime.ParseMediaType(strings.TrimSpace(part))
			if err != nil {
				continue
			}
			quality := 1.0
			if raw, exists := params["q"]; exists {
				parsed, err := strconv.ParseFloat(raw, 64)
				if err != nil || !(parsed >= 0 && parsed <= 1) {
					quality = 0
				} else {
					quality = parsed
				}
			}
			switch coding {
			case "gzip":
				gzipQuality = quality
			case "br":
				brotliQuality = quality
			case "*":
				wildcard = quality
			case "identity":
				identity = quality
			}
		}
	}
	if gzipQuality < 0 {
		gzipQuality = wildcard
	}
	if brotliQuality < 0 {
		brotliQuality = wildcard
	}
	identityAllowed := identity > 0 || identity < 0 && wildcard != 0
	encoding, quality := "", 0.0
	if t.config.EnableBrotli && brotliQuality > 0 {
		encoding, quality = "br", brotliQuality
	}
	if t.config.EnableGzip && gzipQuality > quality {
		encoding, quality = "gzip", gzipQuality
	}
	if identity > quality {
		return "", true
	}
	return encoding, identityAllowed
}

func (t *Transport) writeGraphQLPayload(w nethttp.ResponseWriter, r *nethttp.Request, status int, mediaType string, response core.Response) {
	encoded, err := response.MarshalJSON()
	if err != nil {
		wrapped := &json.MarshalerError{Type: reflect.TypeFor[core.Response](), Err: err}
		nethttp.Error(w, wrapped.Error(), nethttp.StatusInternalServerError)
		return
	}
	t.writeEncodedGraphQLPayload(w, r, status, mediaType, encoded)
}

func (t *Transport) writeGraphQLBatch(w nethttp.ResponseWriter, r *nethttp.Request, status int, mediaType string, responses []core.Response) {
	buf := jsonEncodeBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer jsonEncodeBufferPool.Put(buf)

	enc := json.NewEncoder(buf)
	if err := enc.Encode(responses); err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	encoded := buf.Bytes()
	if n := len(encoded); n > 0 && encoded[n-1] == '\n' {
		encoded = encoded[:n-1]
	}
	t.writeEncodedGraphQLPayload(w, r, status, mediaType, encoded)
}

func (t *Transport) writeEncodedGraphQLPayload(w nethttp.ResponseWriter, r *nethttp.Request, status int, mediaType string, encoded []byte) {
	contentType := mediaType
	if !strings.Contains(contentType, "charset=") {
		contentType += "; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	if t.config.EnableGzip || t.config.EnableBrotli {
		w.Header().Add("Vary", "Accept-Encoding")
	}
	encoding, identityAllowed := t.negotiateEncoding(r)
	if encoding == "" && !identityAllowed {
		writeRequestError(r.Context(), w, mediaType, nethttp.StatusNotAcceptable, fmt.Errorf("no acceptable response content encoding"))
		return
	}
	if encoding != "" && (len(encoded) >= 64 || !identityAllowed) {
		w.Header().Set("Content-Encoding", encoding)
		w.WriteHeader(status)
		cw := newCompressWriter(w, encoding)
		if _, err := cw.Write(encoded); err != nil {
			_ = cw.Close()
			return
		}
		_ = cw.Close()
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}

// newCompressWriter returns a compressing writer for the negotiated encoding.
// The caller must Close it to flush the trailer.
func newCompressWriter(w io.Writer, encoding string) io.WriteCloser {
	if encoding == "br" {
		return brotli.NewWriter(w)
	}
	return gzip.NewWriter(w)
}

// incrementalBoundary is the multipart boundary used for incremental delivery.
// A single hyphen matches the value used by the GraphQL incremental delivery
// reference implementations.
const incrementalBoundary = "-"

func acceptsMultipartMixed(r *nethttp.Request) bool {
	for _, raw := range r.Header.Values("Accept") {
		for _, part := range strings.Split(raw, ",") {
			mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(part))
			if err != nil || mediaType != "multipart/mixed" {
				continue
			}
			if raw, exists := params["q"]; exists {
				quality, err := strconv.ParseFloat(raw, 64)
				if err != nil || !(quality > 0 && quality <= 1) {
					continue
				}
			}
			return true
		}
	}
	return false
}

// writeIncremental streams a GraphQL incremental-delivery response as
// multipart/mixed per https://github.com/graphql/graphql-over-http. The initial
// payload is sent first, followed by one part per deferred/streamed payload, and
// finally the closing boundary.
func (t *Transport) writeIncremental(w nethttp.ResponseWriter, r *nethttp.Request, ie core.IncrementalExecutor, req core.Request) {
	started := false
	emitInitial := func(initial core.Response) error {
		initialJSON, err := json.Marshal(initial)
		if err != nil {
			nethttp.Error(w, "failed to encode incremental response", nethttp.StatusInternalServerError)
			return err
		}
		status := responseStatus(core.MediaTypeGraphQLResponse, initial)
		if status != nethttp.StatusOK {
			if responseType, accepted := core.NegotiateResponseContentType(r.Header.Values("Accept")); accepted {
				t.writeGraphQLPayload(w, r, responseStatus(responseType, initial), responseType, initial)
				return nil
			}
		}
		w.Header().Set("Content-Type", `multipart/mixed; boundary="`+incrementalBoundary+`"; deferSpec=20220824`)
		w.WriteHeader(status)
		started = true
		return writeMultipartPart(w, initialJSON)
	}
	emitPatch := func(payload core.IncrementalPayload, hasNext bool) error {
		body, err := (core.Response{Incremental: []core.IncrementalPayload{payload}, HasNext: &hasNext}).MarshalJSON()
		if err != nil {
			return err
		}
		return writeMultipartPart(w, body)
	}
	if streaming, ok := ie.(core.IncrementalStreamExecutor); ok {
		if err := streaming.ExecuteIncrementalStream(r.Context(), req, emitInitial, emitPatch); err != nil {
			return
		}
	} else {
		initial, payloads := ie.ExecuteIncremental(r.Context(), req)
		if err := r.Context().Err(); err != nil {
			return
		}
		if err := emitInitial(initial); err != nil {
			return
		}
		for i, payload := range payloads {
			if err := r.Context().Err(); err != nil {
				return
			}
			if err := emitPatch(payload, i < len(payloads)-1); err != nil {
				return
			}
		}
	}
	if !started || r.Context().Err() != nil {
		return
	}
	if _, err := io.WriteString(w, "\r\n--"+incrementalBoundary+"--\r\n"); err != nil {
		return
	}
	_ = flushMultipart(w)
}

func writeMultipartPart(w nethttp.ResponseWriter, body []byte) error {
	var b bytes.Buffer
	b.WriteString("\r\n--")
	b.WriteString(incrementalBoundary)
	b.WriteString("\r\nContent-Type: application/json; charset=utf-8\r\n\r\n")
	b.Write(body)
	n, err := w.Write(b.Bytes())
	if err != nil {
		return err
	}
	if n != b.Len() {
		return io.ErrShortWrite
	}
	return flushMultipart(w)
}

func flushMultipart(w nethttp.ResponseWriter) error {
	if flusher, ok := w.(interface{ FlushError() error }); ok {
		return flusher.FlushError()
	}
	if flusher, ok := w.(nethttp.Flusher); ok {
		flusher.Flush()
	}
	return nil
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
		size := int64(len(q.Get("query")) + len(q.Get("variables")) + len(q.Get("operationName")) + len(q.Get("extensions")))
		if size > max {
			return fmt.Errorf("request exceeds %d byte limit", max)
		}
		return nil
	default:
		return nil
	}
}

func requestBodyTooLarge(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "request body too large") ||
		(strings.Contains(msg, "request exceeds") && strings.Contains(msg, "byte limit"))
}

func writeRequestError(ctx context.Context, w nethttp.ResponseWriter, mediaType string, status int, err error) {
	requestError := core.NewRequestError(err)
	var persistedError *persistedQueryError
	if errors.As(err, &persistedError) {
		requestError.Extensions["code"] = persistedError.code
	}
	res := core.Response{Errors: []core.Error{requestError}}
	res = core.AttachRequestIDExtension(res, core.RequestIDFromContext(ctx))
	core.WriteGraphQLResponse(w, status, mediaType, res)
}
