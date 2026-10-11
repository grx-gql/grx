// Package grx is the top-level entry point of the grx GraphQL runtime.
//
// A minimal HTTP GraphQL server only needs a schema:
//
//	srv, err := grx.NewServer(
//	    grx.WithSchema(graph.Schema()),
//	)
//
// Advanced setups can add plugins, custom paths, pub/sub-backed
// subscriptions, WebSocket, and SSE transports:
//
//	srv, err := grx.NewServer(
//	    grx.WithSchema(graph.Schema(bus)),
//	    grx.WithPlugins(loggerPlugin),
//	    grx.WithPlaygroundPath("/"),
//	    grx.WithSubscriptionPath("/graphql/ws"),
//	    grx.WithTransports(
//	        websocket.New(),
//	        sse.New(),
//	    ),
//	)
//
// For GraphQL-over-HTTP requests from Go code, use package github.com/grx-gql/grx/client.
package grx

import (
	"errors"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/plugins"
	"github.com/grx-gql/grx/schema"
	"github.com/grx-gql/grx/server"
)

// ErrMissingSchema is returned by [NewServer] when [WithSchema] was not used.
var ErrMissingSchema = errors.New("grx.NewServer requires grx.WithSchema(...)")

// Server is the HTTP handler returned by [NewServer]. It is an alias of
// [server.Server] so callers can refer to it without importing the
// `server` package directly.
type Server = server.Server

// OperationContext describes the selected operation passed to [OperationAuthorizer].
type OperationContext = exec.OperationContext

// FieldAuthorizationContext describes a field about to resolve for [FieldAuthorizer].
type FieldAuthorizationContext = exec.FieldAuthorizationContext

// OperationAuthorizer runs during parsed-document validation, before field execution.
type OperationAuthorizer = exec.OperationAuthorizer

// FieldAuthorizer runs once per resolved field before the resolver executes.
type FieldAuthorizer = exec.FieldAuthorizer

// Option configures a [Server] built by [NewServer]. Options are applied
// in the order they are supplied; later options override earlier ones for
// scalar fields and append to slice fields.
type Option func(*server.Config)

// NewServer builds a [Server] from opts. Supply [WithSchema]; other behaviour
// (plugins, paths, transports) uses the remaining With* helpers.
//
// It returns an error when the schema configuration is incomplete or invalid.
func NewServer(opts ...Option) (*Server, error) {
	cfg := server.Config{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Schema.Query == nil {
		return nil, ErrMissingSchema
	}
	return server.New(cfg)
}

// WithSchema sets the code-first resolver bundle used to build the GraphQL schema.
func WithSchema(schemaConfig schema.Config) Option {
	return func(c *server.Config) {
		c.Schema = schemaConfig
	}
}

// WithPlugins registers lifecycle plugins that observe or short-circuit
// every GraphQL request. Plugins are invoked in registration order.
// Calling WithPlugins multiple times appends to the existing chain.
func WithPlugins(pluginList ...plugins.Plugin) Option {
	return func(c *server.Config) {
		c.Plugins = append(c.Plugins, pluginList...)
	}
}

// WithPlaygroundPath sets the URL path at which the bundled GraphiQL
// playground is served on GET. The empty string disables the playground.
func WithPlaygroundPath(path string) Option {
	return func(c *server.Config) {
		c.PlaygroundPath = path
	}
}

// WithGraphQLPath sets the path for queries and mutations (POST JSON etc.).
// The empty string defaults to "/graphql".
func WithGraphQLPath(path string) Option {
	return func(c *server.Config) {
		c.GraphQLPath = path
	}
}

// WithSubscriptionPath sets the path exclusively used by bundled WebSocket and
// SSE transports when it differs from the GraphQL path. Leave empty to serve
// subscriptions on the same path as POST /graphql (common with graphql-transport-ws).
func WithSubscriptionPath(path string) Option {
	return func(c *server.Config) {
		c.SubscriptionPath = path
	}
}

// WithTransports registers protocol handlers (WebSocket, SSE, ...) that
// the server consults before falling back to the default HTTP+JSON
// transport. Transports are tried in registration order; the first one
// whose Match returns true takes ownership of the response. Calling
// WithTransports multiple times appends to the existing chain.
func WithTransports(transports ...core.Transport) Option {
	return func(c *server.Config) {
		c.Transports = append(c.Transports, transports...)
	}
}

// WithRequestTimeout sets a deadline on the context passed into each GraphQL
// request (queries, mutations, and subscription setup).
func WithRequestTimeout(d time.Duration) Option {
	return func(c *server.Config) {
		c.RequestTimeout = d
	}
}

// WithDisableIntrospection rejects __schema / __type introspection queries.
func WithDisableIntrospection() Option {
	return func(c *server.Config) {
		c.DisableIntrospection = true
	}
}

// WithMaxHTTPRequestBytes sets a byte limit on the default GraphQL HTTP
// transport (POST JSON body and GET URL parameters).
func WithMaxHTTPRequestBytes(n int64) Option {
	return func(c *server.Config) {
		c.MaxHTTPRequestBytes = n
	}
}

// WithMaxSelectionDepth sets a parse-time limit on nested selection sets.
func WithMaxSelectionDepth(depth int) Option {
	return func(c *server.Config) {
		c.MaxSelectionDepth = depth
	}
}

// WithQueryConcurrency bounds concurrent root fields of query operations.
// Values at most one retain serial execution. Mutations stay serial. Resolvers
// and field hooks must support concurrent calls when this option is enabled.
func WithQueryConcurrency(limit int) Option {
	return func(config *server.Config) { config.QueryConcurrency = limit }
}

// WithResponseGzip enables gzip compression for JSON GraphQL responses when
// the client sends Accept-Encoding: gzip.
func WithResponseGzip() Option {
	return func(c *server.Config) {
		c.EnableResponseGzip = true
	}
}

// WithPersistedQueries registers SHA-256 hex digest → query text mappings for
// automatic persisted query (APQ) support on the default HTTP transport.
func WithPersistedQueries(queries map[string]string) Option {
	return func(c *server.Config) {
		c.PersistedQueries = queries
	}
}

// WithAPQCacheSize enables automatic persisted-query registration with at most
// size entries. Zero disables the cache; each cached query is limited to 64 KiB.
func WithAPQCacheSize(size int) Option {
	return func(c *server.Config) { c.APQCacheSize = size }
}

// WithOperationAuthorizer registers a hook invoked while validating the parsed
// document (before any field resolves). Return a non-nil error to reject the request.
func WithOperationAuthorizer(auth OperationAuthorizer) Option {
	return func(c *server.Config) {
		c.OperationAuthorizer = auth
	}
}

// WithFieldAuthorizer registers a hook that authorizes each field before its resolver runs.
func WithFieldAuthorizer(auth FieldAuthorizer) Option {
	return func(c *server.Config) {
		c.FieldAuthorizer = auth
	}
}

// WithSchemaSDLPath enables GET export of a minimal SDL document at path
// (for example "/schema.graphql"). The empty string disables the endpoint.
func WithSchemaSDLPath(path string) Option {
	return func(c *server.Config) {
		c.SchemaSDLPath = path
	}
}
