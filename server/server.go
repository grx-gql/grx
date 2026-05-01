// Package server is the HTTP entry point for the grx GraphQL runtime. It
// wires user-supplied schema values, plugins, and transports into a value
// that satisfies http.Handler and serves both the GraphiQL playground and
// the GraphQL endpoint.
package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/exec"
	grxhttp "github.com/patrickkabwe/grx/pkg/http"
	"github.com/patrickkabwe/grx/plugin"
	"github.com/patrickkabwe/grx/schema"
)

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

	// Transports registers protocol handlers (WebSocket, SSE, ...) that
	// the server consults to handle requests on the GraphQL endpoint. Each
	// request is offered to the transports in order; the first one whose
	// Match returns true takes ownership of the response. A default
	// HTTP+JSON transport ([pkg/http.Transport]) is appended to the chain
	// automatically, so plain `POST /graphql` requests always work; to
	// customise its behaviour, register a [pkg/http.Transport] (or a
	// custom transport that matches POST) explicitly before any others.
	Transports []core.Transport
}

// Server is an http.Handler that exposes a GraphQL endpoint and an
// optional GraphiQL playground. Construct one with [New].
type Server struct {
	executor       core.Executor
	playgroundPath string
	transports     []core.Transport
}

const (
	graphQLPath = "/graphql"
	faviconPath = "/favicon.ico"
)

// New builds a [Server] from cfg. It returns an error when the supplied
// schema cannot be reflected into a valid GraphQL schema.
func New(config Config) (*Server, error) {
	schemaValue, err := schema.Build(config.Schema)
	if err != nil {
		return nil, err
	}

	executor := exec.New(schemaValue, config.Plugins)

	// All network handling flows through transports. The user-supplied
	// chain is consulted first; a default HTTP+JSON transport is appended
	// so the canonical `POST /graphql` request continues to work out of
	// the box without any explicit registration.
	transports := make([]core.Transport, 0, len(config.Transports)+1)
	transports = append(transports, config.Transports...)
	transports = append(transports, grxhttp.New())

	return &Server{
		executor:       executor,
		playgroundPath: config.PlaygroundPath,
		transports:     transports,
	}, nil
}

// ServeHTTP routes incoming HTTP traffic. It serves GraphiQL on GET to the
// configured playground path, returns 204 for /favicon.ico, and dispatches
// every /graphql request through the registered transports. Requests that
// no transport claims receive a 404.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == faviconPath && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == s.playgroundPath {
		servePlayground(w)
		return
	}

	if r.URL.Path != graphQLPath {
		http.NotFound(w, r)
		return
	}

	for _, transport := range s.transports {
		if transport.Match(r) {
			transport.Serve(w, r, s.executor)
			return
		}
	}

	http.NotFound(w, r)
}

func servePlayground(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write([]byte(playgroundHTML(graphQLPath))); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func playgroundHTML(endpoint string) string {
	replacer := strings.NewReplacer("{{ENDPOINT}}", strconv.Quote(endpoint))
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

			const endpoint = {{ENDPOINT}};
			const url = new URL(endpoint, window.location.href);
			const subscriptionUrl = new URL(endpoint, window.location.href);
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
					headers: { "content-type": "application/json", accept: "application/json" },
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
