// Package server is the HTTP entry point for the grx GraphQL runtime. It
// wires user-supplied schema values, plugins, and transports into a value
// that satisfies http.Handler and serves both the GraphiQL playground and
// the GraphQL endpoint.
package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/exec"
	grxhttp "github.com/patrickkabwe/grx/pkg/http"
	"github.com/patrickkabwe/grx/pkg/sse"
	"github.com/patrickkabwe/grx/pkg/websocket"
	"github.com/patrickkabwe/grx/plugin"
	"github.com/patrickkabwe/grx/schema"
)

// Middleware wraps the HTTP handler exposed by Server.
type Middleware func(http.Handler) http.Handler

// Config tunes a [Server]. Schema is required; the remaining fields have
// safe zero-value defaults.
type Config struct {
	// Schema selects the root resolvers the executor reflects over.
	// Query is required; Mutation and Subscription are optional. See
	// [schema.Config] for the field-by-field documentation.
	Schema schema.Config

	// Plugins are invoked at every lifecycle stage of a GraphQL
	// request, in registration order. They may short-circuit a request
	// by returning an error.
	Plugins []plugin.Plugin

	// PlaygroundPath is the URL path at which the bundled GraphiQL
	// playground is served on GET. The empty string disables the
	// playground.
	PlaygroundPath string

	// GraphQLPath is the URL path for query and mutation traffic (always
	// HTTP: JSON POST plus any user transports retained on this path).
	// The empty string defaults to "/graphql".
	GraphQLPath string

	// SubscriptionPath, when empty, uses GraphQLPath for subscription
	// transports (WebSocket + SSE)—the conventional single-endpoint
	// deployment for graphql-transport-ws alongside POST /graphql.
	//
	// When non-empty (and normalized not equal to GraphQLPath), WebSocket and
	// SSE transports registered in Transports are moved to SubscriptionPath so
	// queries/mutations stay on GraphQLPath.
	SubscriptionPath string

	// Transports registers protocol handlers (WebSocket, SSE, ...) that
	// the server consults. Each request on the routed path is offered to the
	// relevant chain in order; the first one whose Match returns true takes
	// ownership of the response. For GraphQLPath, a default HTTP+JSON
	// transport ([pkg/http.Transport]) is appended automatically, so plain
	// POST to GraphQLPath always works unless you customise that transport.
	//
	// When SubscriptionPath is split from GraphQLPath, bundled *websocket and
	// *sse transports are routed only to SubscriptionPath; other transports
	// stay on GraphQLPath.
	Transports []core.Transport

	// Middleware wraps the final HTTP handler. Middleware is applied in the
	// order supplied, so the first middleware sees each request first.
	Middleware []Middleware

	// RequestTimeout, when non-zero, applies a deadline to the context of each
	// GraphQL request handled by the built-in transports.
	RequestTimeout time.Duration

	// DisableIntrospection rejects __schema / __type introspection documents.
	DisableIntrospection bool

	// MaxHTTPRequestBytes limits the default GraphQL HTTP transport (POST body
	// and GET query parameters). Zero leaves the transport unlimited.
	MaxHTTPRequestBytes int64

	// EnableResponseGzip enables gzip compression for JSON GraphQL responses
	// when the client advertises Accept-Encoding: gzip.
	EnableResponseGzip bool

	// PersistedQueries maps SHA-256 hex digests (case-insensitive) to GraphQL
	// query strings for automatic persisted query (APQ) support on the default
	// HTTP transport.
	PersistedQueries map[string]string

	// RequirePersistedQuery rejects HTTP requests that do not include an APQ
	// sha256Hash under extensions.persistedQuery.
	RequirePersistedQuery bool

	// StrictPersistedQueries validates APQ metadata and verifies query/hash
	// matches when a request includes both.
	StrictPersistedQueries bool

	// MaxVariableBytes limits the JSON-encoded variables payload. Zero leaves
	// variables unlimited.
	MaxVariableBytes int64

	// MaskInternalErrors replaces internal resolver, hook, and panic errors in
	// client-facing GraphQL responses. Raw errors still reach plugin.Error.
	MaskInternalErrors bool

	// ClientErrorMessage is used when MaskInternalErrors is enabled. Empty uses
	// "internal server error".
	ClientErrorMessage string

	// OperationAuthorizer authorizes a parsed operation before execution.
	OperationAuthorizer exec.OperationAuthorizer

	// FieldAuthorizer authorizes a field before its resolver runs.
	FieldAuthorizer exec.FieldAuthorizer

	// RateLimiter can reject an operation based on context and operation
	// metadata before execution starts.
	RateLimiter exec.RateLimiter

	// TrustedDocuments maps SHA-256 query hashes to exact trusted query strings.
	// Empty disables the safelist.
	TrustedDocuments map[string]string

	// RejectUnknownVariables rejects variables not declared by the selected
	// operation.
	RejectUnknownVariables bool

	// MaxSelectionCount limits total selections in an operation. Zero disables
	// the limit.
	MaxSelectionCount int

	// MaxAliasCount limits aliased fields in an operation. Zero disables the
	// limit.
	MaxAliasCount int

	// MaxRootFieldCount limits top-level fields in an operation. Zero disables
	// the limit.
	MaxRootFieldCount int

	// DocumentCacheSize caches parsed documents for requests without variables.
	// Zero disables the cache.
	DocumentCacheSize int

	// LexerCacheSize bounds an LRU map of shared token streams per normalized
	// query. Zero means: when DocumentCacheSize is positive, use that same
	// capacity for the lexer cache; otherwise lexer reuse is disabled.
	LexerCacheSize int

	// SchemaSDLPath enables GET export of a minimal SDL document at this path
	// (for example "/schema.graphql"). The empty string disables the endpoint.
	SchemaSDLPath string
}

// Server is an http.Handler that exposes a GraphQL endpoint and an
// optional GraphiQL playground. Construct one with [New].
type Server struct {
	executor         core.Executor
	schemaValue      *schema.Schema
	PlaygroundPath   string
	GraphqlPath      string // normalized; persisted from New
	SubscriptionPath string // pathname passed to graphql-ws in playground (canonical subscription URL)
	schemaSDLPath    string

	separateSubs   bool             // graphqlPath != subscriptionPath routing
	mainChain      []core.Transport // graphqlPath
	subChain       []core.Transport // subscriptionPath; empty when combined endpoint
	handler        http.Handler
	requestTimeout time.Duration
}

const faviconPath = "/favicon.ico"

// pathRestricted restricts core.Transport.Match to requests whose URL path
// equals path (after server-level routing passes them through—this is extra
// safety for partitioned endpoints).
type pathRestricted struct {
	path string
	core.Transport
}

func (p pathRestricted) Match(r *http.Request) bool {
	if r.URL.Path != p.path {
		return false
	}
	return p.Transport.Match(r)
}

func (p pathRestricted) Serve(w http.ResponseWriter, r *http.Request, executor core.Executor) {
	p.Transport.Serve(w, r, executor)
}

// New builds a [Server] from cfg. It returns an error when the supplied
// schema cannot be reflected into a valid GraphQL schema.
func New(config Config) (*Server, error) {
	schemaValue, err := schema.Build(config.Schema)
	if err != nil {
		return nil, err
	}

	var execOpts []exec.ExecutorOption
	if config.DisableIntrospection {
		execOpts = append(execOpts, exec.WithDisableIntrospection())
	}
	if config.MaskInternalErrors {
		execOpts = append(execOpts, exec.WithClientErrorMasking(config.ClientErrorMessage))
	}
	if config.OperationAuthorizer != nil {
		execOpts = append(execOpts, exec.WithOperationAuthorizer(config.OperationAuthorizer))
	}
	if config.FieldAuthorizer != nil {
		execOpts = append(execOpts, exec.WithFieldAuthorizer(config.FieldAuthorizer))
	}
	if config.RateLimiter != nil {
		execOpts = append(execOpts, exec.WithRateLimiter(config.RateLimiter))
	}
	if len(config.TrustedDocuments) > 0 {
		execOpts = append(execOpts, exec.WithTrustedDocuments(config.TrustedDocuments))
	}
	if config.RejectUnknownVariables {
		execOpts = append(execOpts, exec.WithRejectUnknownVariables())
	}
	if config.MaxSelectionCount > 0 {
		execOpts = append(execOpts, exec.WithMaxSelectionCount(config.MaxSelectionCount))
	}
	if config.MaxAliasCount > 0 {
		execOpts = append(execOpts, exec.WithMaxAliasCount(config.MaxAliasCount))
	}
	if config.MaxRootFieldCount > 0 {
		execOpts = append(execOpts, exec.WithMaxRootFieldCount(config.MaxRootFieldCount))
	}
	if config.DocumentCacheSize > 0 {
		execOpts = append(execOpts, exec.WithDocumentCache(config.DocumentCacheSize))
	}
	lexSize := config.LexerCacheSize
	if lexSize <= 0 && config.DocumentCacheSize > 0 {
		lexSize = config.DocumentCacheSize
	}
	if lexSize > 0 {
		execOpts = append(execOpts, exec.WithLexerCache(lexSize))
	}
	executor := exec.New(schemaValue, config.Plugins, execOpts...)

	graphqlPath := normalizePath(config.GraphQLPath, "/graphql")
	subPath := graphqlPath
	if strings.TrimSpace(config.SubscriptionPath) != "" {
		subPath = normalizePath(config.SubscriptionPath, "/ws")
	}
	if subPath == "" {
		return nil, errors.New("server: SubscriptionPath is invalid")
	}
	separate := subPath != graphqlPath

	var main []core.Transport
	var sub []core.Transport

	if separate {
		for _, transport := range config.Transports {
			switch transport.(type) {
			case *websocket.Transport, *sse.Transport:
				sub = append(sub, pathRestricted{path: subPath, Transport: transport})
			default:
				main = append(main, transport)
			}
		}
		if len(sub) == 0 {
			return nil, errors.New(`server: SubscriptionPath differs from GraphQLPath but no *websocket.Transport or *sse.Transport was registered`)
		}
	} else {
		main = append(main, config.Transports...)
	}

	httpTransportCfg := grxhttp.Config{Path: graphqlPath}
	if config.MaxHTTPRequestBytes != 0 {
		httpTransportCfg.MaxRequestBytes = config.MaxHTTPRequestBytes
	}
	if config.EnableResponseGzip {
		httpTransportCfg.EnableGzip = true
	}
	if len(config.PersistedQueries) > 0 {
		httpTransportCfg.PersistedQueries = config.PersistedQueries
	}
	if config.RequirePersistedQuery {
		httpTransportCfg.RequirePersistedQuery = true
	}
	if config.StrictPersistedQueries {
		httpTransportCfg.StrictPersistedQueries = true
	}
	if config.MaxVariableBytes != 0 {
		httpTransportCfg.MaxVariableBytes = config.MaxVariableBytes
	}
	if config.ClientErrorMessage != "" {
		httpTransportCfg.PanicErrorMessage = config.ClientErrorMessage
	}
	main = append(main, grxhttp.New(httpTransportCfg))

	sdlPath := strings.TrimSpace(config.SchemaSDLPath)
	if sdlPath != "" && !strings.HasPrefix(sdlPath, "/") {
		sdlPath = "/" + sdlPath
	}

	srv := &Server{
		executor:         executor,
		schemaValue:      schemaValue,
		PlaygroundPath:   config.PlaygroundPath,
		GraphqlPath:      graphqlPath,
		SubscriptionPath: subPath,
		schemaSDLPath:    sdlPath,
		separateSubs:     separate,
		mainChain:        main,
		subChain:         sub,
		requestTimeout:   config.RequestTimeout,
	}
	srv.handler = applyMiddleware(http.HandlerFunc(srv.serveHTTP), config.Middleware)
	return srv, nil
}

func applyMiddleware(handler http.Handler, middleware []Middleware) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

func normalizePath(path string, defaultPath string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return defaultPath
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func (s *Server) canonicalGraphQLPath() string {
	if s.GraphqlPath != "" {
		return s.GraphqlPath
	}
	return "/graphql"
}

func (s *Server) canonicalWSPath() string {
	if s.SubscriptionPath != "" {
		return s.SubscriptionPath
	}
	return s.canonicalGraphQLPath()
}

// ServeHTTP routes incoming HTTP traffic. It serves GraphiQL on GET to the
// configured playground path, returns 204 for /favicon.ico, and dispatches
// GraphQL traffic through the registered transports. Requests that no
// transport claims receive a 404.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.handler != nil {
		s.handler.ServeHTTP(w, r)
		return
	}
	s.serveHTTP(w, r)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == faviconPath && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if s.schemaSDLPath != "" && r.Method == http.MethodGet && r.URL.Path == s.schemaSDLPath {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(schema.PrintSDL(s.schemaValue)))
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == s.PlaygroundPath {
		servePlayground(w, s.canonicalGraphQLPath(), s.canonicalWSPath())
		return
	}

	gqlPath := s.canonicalGraphQLPath()
	wsPath := s.canonicalWSPath()

	if r.Method == http.MethodOptions && r.URL.Path == gqlPath {
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if s.separateSubs && len(s.subChain) > 0 && r.URL.Path == wsPath {
		for _, transport := range s.subChain {
			if transport.Match(r) {
				req, cancel := s.withRequestDeadline(r)
				transport.Serve(w, req, s.executor)
				cancel()
				return
			}
		}
		http.NotFound(w, r)
		return
	}

	if r.URL.Path == gqlPath {
		for _, transport := range s.mainChain {
			if transport.Match(r) {
				req, cancel := s.withRequestDeadline(r)
				transport.Serve(w, req, s.executor)
				cancel()
				return
			}
		}
		http.NotFound(w, r)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) withRequestDeadline(r *http.Request) (*http.Request, context.CancelFunc) {
	if s.requestTimeout <= 0 {
		return r, func() {}
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	return r.WithContext(ctx), cancel
}

func servePlayground(w http.ResponseWriter, httpPath string, wsPath string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write([]byte(playgroundHTML(httpPath, wsPath))); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func playgroundHTML(httpPath string, wsPath string) string {
	replacer := strings.NewReplacer(
		"{{HTTP_ENDPOINT}}", strconv.Quote(httpPath),
		"{{WS_ENDPOINT}}", strconv.Quote(wsPath),
	)
	return replacer.Replace(`<!doctype html>
<html lang="en">
	<head>
		<meta charset="utf-8">
		<meta name="viewport" content="width=device-width, initial-scale=1">
		<title>GraphiQL</title>
		<link rel="stylesheet" href="https://esm.sh/graphiql@5.2.2/dist/style.css">
		<link rel="stylesheet" href="https://esm.sh/@graphiql/plugin-explorer@5.1.1/dist/style.css">
		<style>
			html,
			body,
			#root {
				height: 100vh;
				margin: 0;
				width: 100vw;
			}
		</style>
	</head>
	<body>
		<div id="root"></div>
		<script type="importmap">
			{
				"imports": {
					"react": "https://esm.sh/react@19.2.5",
					"react/": "https://esm.sh/react@19.2.5/",
					"react-dom": "https://esm.sh/react-dom@19.2.5",
					"react-dom/client": "https://esm.sh/react-dom@19.2.5/client",
					"graphiql": "https://esm.sh/graphiql@5.2.2?standalone&external=react,react-dom,@graphiql/react,graphql",
					"graphiql/": "https://esm.sh/graphiql@5.2.2/",
					"@graphiql/plugin-explorer": "https://esm.sh/@graphiql/plugin-explorer@5.1.1?standalone&external=react,@graphiql/react,graphql",
					"@graphiql/toolkit": "https://esm.sh/@graphiql/toolkit@0.11.3?standalone&external=graphql",
					"@graphiql/react": "https://esm.sh/@graphiql/react@0.37.3?standalone&external=react,react-dom,graphql,@graphiql/toolkit,@emotion/is-prop-valid",
					"graphql": "https://esm.sh/graphql@16.13.2",
					"graphql-ws": "https://esm.sh/graphql-ws@6.0.6?external=graphql",
					"@emotion/is-prop-valid": "data:text/javascript,"
				}
			}
		</script>
		<script type="module">
			import React from "react";
			import ReactDOM from "react-dom/client";
			import { GraphiQL, HISTORY_PLUGIN } from "graphiql";
			import { explorerPlugin } from "@graphiql/plugin-explorer";
			import { createClient as createWSClient } from "graphql-ws";
			import { parse } from "graphql";
			import "graphiql/setup-workers/esm.sh";

			const httpEndpoint = {{HTTP_ENDPOINT}};
			const wsEndpoint = {{WS_ENDPOINT}};
			const url = new URL(httpEndpoint, window.location.href);
			const subscriptionUrl = new URL(wsEndpoint, window.location.href);
			subscriptionUrl.protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
			const wsClient = createWSClient({ url: subscriptionUrl.toString() });

			const isSubscription = (request) => {
				try {
					const document = parse(request.query);
					const operation = document.definitions.find(
						(definition) =>
							definition.kind === "OperationDefinition" &&
							(!request.operationName || definition.name?.value === request.operationName)
					);
					return operation?.operation === "subscription";
				} catch {
					return false;
				}
			};

			const httpFetch = async (request) => {
				const response = await fetch(url.toString(), {
					method: "POST",
					headers: { "content-type": "application/json", accept: "application/graphql-response+json, application/json;q=0.9" },
					body: JSON.stringify(request),
				});
				return response.json();
			};

			const wsSubscribe = (request) => ({
				[Symbol.asyncIterator]() {
					const pending = [];
					const waiters = [];
					let done = false;
					let error;
					const push = (value) => {
						if (waiters.length) waiters.shift().resolve({ value, done: false });
						else pending.push(value);
					};
					const finish = (err) => {
						done = true;
						error = err;
						while (waiters.length) {
							const waiter = waiters.shift();
							err ? waiter.reject(err) : waiter.resolve({ value: undefined, done: true });
						}
					};
					const dispose = wsClient.subscribe(request, {
						next: (value) => push(value),
						error: (err) => finish(err),
						complete: () => finish(),
					});
					return {
						next() {
							if (pending.length) return Promise.resolve({ value: pending.shift(), done: false });
							if (done) return error ? Promise.reject(error) : Promise.resolve({ value: undefined, done: true });
							return new Promise((resolve, reject) => waiters.push({ resolve, reject }));
						},
						return() {
							dispose();
							finish();
							return Promise.resolve({ value: undefined, done: true });
						},
					};
				},
			});

			const fetcher = (request) => (isSubscription(request) ? wsSubscribe(request) : httpFetch(request));

			const plugins = [HISTORY_PLUGIN, explorerPlugin()];
			const root = ReactDOM.createRoot(document.getElementById("root"));

			root.render(
				React.createElement(GraphiQL, {
					defaultEditorToolsVisibility: true,
					fetcher: fetcher,
					plugins: plugins
				})
			);
		</script>
	</body>
</html>`)
}
