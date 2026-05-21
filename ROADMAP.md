# Roadmap

This file is the source of truth for feature parity tracking and roadmap sync into the docs site.

## Feature Parity Checklist

### Server HTTP Transport

- [x] `POST /graphql` JSON request handling
- [x] Configurable browser UI path
- [x] GraphiQL UI served from existing CDN assets
- [x] `/favicon.ico` handled without 404 noise
- [x] Invalid JSON request errors
- [x] Missing query request errors
- [x] `GET /graphql?query=...` support
- [x] `application/graphql-response+json` response content type negotiation
- [x] Strict request `Content-Type` validation
- [x] GraphQL-over-HTTP status code semantics
- [x] Batched request support
- [ ] Multipart upload support
- [ ] Incremental delivery over HTTP for `@defer` and `@stream`
- [x] Subscriptions over WebSocket or SSE
- [x] Request size limits
- [x] Timeout/deadline handling
- [x] Persisted queries
- [x] Automatic persisted queries
- [ ] Query cost/depth/complexity limits
- [x] CORS configuration
- [x] Response compression (gzip)
- [ ] Response compression (brotli)
- [x] CSRF prevention for state-changing GET requests
- [x] Introspection enable/disable toggle
- [x] Schema registry endpoint (`/schema.graphql`)

### GraphQL Language Lexer

- [x] Names
- [x] Integer and float-like number tokens
- [x] String tokens without escapes
- [x] Punctuation tokens for basic selections and arguments
- [x] Ignored whitespace and comma handling
- [x] Comments
- [x] Unicode source text correctness
- [x] Escaped string values
- [x] Block strings
- [x] Full integer validation
- [x] Full float validation
- [x] Spread token `...`
- [x] Equals token `=`
- [x] At token `@`
- [x] Ampersand token `&`
- [x] Pipe token `|`
- [x] EOF and source-location tracking for diagnostics
- [x] Unicode escape sequences including variable-width `\u{...}`
- [ ] Line terminator normalization per spec
- [x] Block string common indentation stripping
- [x] BOM handling at source start
- [x] Punctuator tokens `$`, `!`, `[`, `]`, `{`, `}`, `(`, `)`, `:`

### GraphQL Parser

- [x] Anonymous query shorthand selection sets
- [x] Named query operations
- [x] Named mutation operations
- [x] Variable references in argument values
- [x] Basic scalar argument values
- [x] Basic nested input object variables
- [x] Multiple definitions per document
- [x] Operation selection by `operationName`
- [x] Subscription operations
- [x] Field aliases
- [x] Fragment definitions
- [x] Fragment spreads
- [x] Inline fragments
- [ ] Directives on operations, fields, fragments, and variable definitions
- [ ] Variable definitions with default values
- [x] List value literals
- [x] Object value literals
- [x] Enum value literals with type context
- [ ] Null value validation in context
- [ ] Full grammar-compliant error reporting
- [ ] Type system definitions via SDL (schema, types, directives)
- [ ] Description strings on definitions
- [ ] Repeatable directive declarations
- [ ] Oneof input object definitions
- [ ] Interface `implements` lists including multi-interface inheritance
- [ ] Union member type lists
- [ ] Scalar type definitions with `@specifiedBy`
- [ ] Schema and type `extend` definitions

### Type System

- [x] Scalar types: `String`, `Int`, `Float`, `Boolean`, `ID`
- [x] Object types from Go structs
- [x] Root query object
- [x] Root mutation object
- [x] List type wrappers
- [x] Non-null type wrappers
- [x] Input object types for resolver arguments
- [x] Enum types
- [x] Interface types
- [x] Union types
- [x] Custom scalar registration
- [ ] Schema directives
- [ ] Field directives
- [ ] Argument directives
- [ ] Input field directives
- [ ] Type descriptions
- [ ] Field descriptions
- [ ] Argument descriptions
- [ ] Deprecation metadata
- [x] Default argument values
- [x] Default input field values
- [ ] Schema extension
- [ ] Type extension
- [ ] SDL parser
- [x] SDL printer
- [ ] Schema validation rules
- [ ] Reserved `__` name validation
- [ ] Oneof input objects
- [ ] User-defined directive definitions
- [ ] Repeatable directives
- [ ] Interfaces implementing interfaces
- [ ] Type coordinate resolution
- [x] Subscription root operation type
- [ ] Built-in `specifiedByURL` on scalars
- [ ] Block-string descriptions attached to definitions

### Validation

- [x] Executable definitions only
- [x] Operation name uniqueness
- [x] Lone anonymous operation rule
- [x] Subscription single root field rule
- [x] Field exists on parent type
- [x] Leaf field selection rule
- [x] Composite field selection rule
- [ ] Field selection merging
- [x] Argument exists on field/directive
- [x] Argument uniqueness
- [x] Required argument presence
- [x] Fragment name uniqueness
- [x] Fragment target type existence
- [x] Fragments on composite types only
- [x] Fragment spreads target defined
- [x] Fragment spreads must not form cycles
- [x] Fragment spread type overlap
- [x] Fragment must be used
- [ ] Value type correctness
- [ ] Input object field existence
- [ ] Input object field uniqueness
- [ ] Required input object field presence
- [x] Directive existence
- [x] Directive location validity
- [x] Directive uniqueness where non-repeatable
- [ ] Variable uniqueness
- [ ] Variables are input types
- [ ] Variable use is defined
- [ ] Variable use is allowed by location/type
- [ ] Variables are used
- [x] Unknown operation name error
- [x] Multiple operations require operation name
- [ ] Oneof input object exactly-one-field rule
- [ ] `@defer`/`@stream` label uniqueness per document
- [ ] `@defer`/`@stream` placement rules
- [ ] Values of correct input type (including coercion compatibility)
- [ ] Input coercion for variable and argument values
- [ ] Executable directive location enforcement
- [ ] Known type names
- [x] Known fragment names

### Execution

- [x] Query root execution
- [x] Mutation root execution
- [x] Nested object field execution
- [x] List result completion
- [x] Resolver argument binding from variables
- [x] Partial data with field errors
- [x] `__typename`
- [x] Field aliases
- [x] Fragment collection
- [x] Inline fragment type-condition matching
- [x] `@skip(if:)`
- [x] `@include(if:)`
- [ ] Serial mutation field execution guarantee
- [ ] Parallel query field execution where safe
- [x] Operation selection by `operationName`
- [x] Variable default value coercion
- [x] Argument default value coercion
- [x] Input object default value coercion
- [ ] Scalar result coercion
- [x] Enum result coercion
- [ ] List input coercion
- [ ] Non-null error bubbling
- [x] Error path for aliased fields
- [x] Error locations
- [x] Interface concrete type resolution
- [x] Union concrete type resolution
- [x] Custom scalar serialization and parsing
- [x] Context cancellation checks
- [x] Subscription source streams
- [x] Subscription response streams
- [ ] Incremental execution for `@defer`
- [ ] Incremental execution for `@stream`
- [x] Resolver panic recovery
- [ ] Concurrent non-mutation field resolution with deterministic ordering
- [ ] Per-request resolver cache / request-scoped memoization
- [ ] Deferred resolver values (thunk/promise-style futures)
- [ ] Abstract type runtime resolution hook
- [ ] Oneof input object runtime validation

### Introspection

- [x] `__schema` fast-path response
- [x] `__type(name:)` fast-path response
- [x] Root query type metadata
- [x] Root mutation type metadata
- [x] Object field metadata
- [x] Field argument metadata
- [x] Input object field metadata
- [x] Basic type reference metadata
- [ ] Full introspection through normal execution
- [ ] Complete `__Schema` fields
- [ ] Complete `__Type` fields
- [ ] Complete `__Field` fields
- [ ] Complete `__InputValue` fields
- [ ] Complete `__EnumValue` fields
- [ ] Complete `__Directive` fields
- [ ] `__TypeKind` enum
- [ ] `__DirectiveLocation` enum
- [ ] Built-in directive introspection
- [ ] Deprecated field filtering through `includeDeprecated`
- [ ] Deprecated enum filtering through `includeDeprecated`
- [ ] Description metadata
- [x] Default value string rendering
- [ ] Custom scalar `specifiedByURL`
- [ ] Correct introspection type registration in schema

### Built-in Directives

- [x] `@skip(if: Boolean!)`
- [x] `@include(if: Boolean!)`
- [ ] `@deprecated(reason: String)`
- [ ] `@specifiedBy(url: String!)`
- [ ] `@oneOf` on input object types
- [ ] `@defer(if: Boolean, label: String)`
- [ ] `@stream(if: Boolean, label: String, initialCount: Int)`

### Response Format

- [x] JSON `data`
- [x] JSON `errors`
- [x] Error message
- [x] Error path for field execution errors
- [x] Error locations
- [x] Error extensions
- [x] Stable response ordering
- [x] Request errors without `data`
- [x] Field errors with partial `data`
- [x] Incremental response payloads
- [x] Top-level `extensions` object
- [x] `hasNext` flag on incremental payloads
- [x] Error classification (request vs field)

### Developer Experience

- [x] GraphiQL UI for local testing
- [x] Logger plugin hooks
- [x] Basic plugin lifecycle hooks
- [x] Public documentation for server configuration
- [x] Public documentation for schema mapping
- [x] Public documentation for resolver signatures
- [x] Public examples for query-only server
- [x] Public examples for query and mutation server
- [ ] Public examples for custom scalars
- [ ] Public examples for directives
- [x] Benchmarks
- [ ] Fuzz tests for parser
- [ ] Spec fixture tests
- [x] Race detector CI
- [ ] Compatibility test suite against GraphiQL introspection query
- [x] Schema SDL export/printing
- [ ] Schema change diff tool
- [x] Public examples for subscriptions
- [ ] Public examples for interfaces and unions
- [ ] Public examples for enums

### Subscriptions

- [x] Subscription root operation type registration
- [x] Subscription source stream creation from resolver
- [x] Subscription response stream dispatch
- [x] Single root field subscription rule
- [x] `graphql-transport-ws` protocol transport
- [x] Server-sent events transport
- [x] Connection initialization payload handling
- [x] Keep-alive/heartbeat handling
- [x] Subscription cancellation and cleanup on client disconnect
- [x] Per-connection context propagation
- [x] Backpressure handling for slow clients (`Config.WriteTimeout`)
- [x] Connection authentication hook (`Config.OnConnect`)
- [x] Pub/Sub primitive for cross-resolver fan-out (`pkg/pubsub.PubSub`)
- [x] In-process pub/sub backend (`pkg/pubsub.Memory`)
- [x] Redis pub/sub backend (`pkg/pubsub/redis`, separate Go module)
- [x] Type-safe generic wrapper with pluggable codec (`pkg/pubsub.Typed`)
- [x] Server-side filters / typed predicates on subscribe

### WebSocket Transport (RFC 6455)

- [x] Server handshake with `Sec-WebSocket-Accept` and version `13`
- [x] Subprotocol negotiation (`graphql-transport-ws`, `graphql-ws`)
- [x] Client-to-server frame masking enforcement
- [x] Fragmented message reassembly
- [x] Ping/Pong/Close control frames
- [x] UTF-8 validation on text frames (close `1007`)
- [x] Reserved bits (RSV1/2/3) must be zero (close `1002`)
- [x] Control frames must have FIN=1 and payload ≤ 125 bytes (close `1002`)
- [x] Reserved opcodes 0x3-0x7 / 0xB-0xF rejected (close `1002`)
- [x] Continuation frame ordering enforced (close `1002`)
- [x] Configurable maximum message size (close `1009`)
- [x] Configurable read idle timeout
- [x] Configurable per-write deadline (slow-consumer protection)
- [x] Origin allowlist hook (`CheckOrigin`)
- [x] Server-initiated application ping interval
- [ ] Server-side graceful shutdown that drains active connections with
      close code `1001`
- [ ] permessage-deflate compression (RFC 7692)
- [ ] Maximum concurrent connection limit
- [ ] Maximum subscriptions-per-connection limit

### graphql-transport-ws Subprotocol

- [x] `connection_init` / `connection_ack`
- [x] `subscribe` / `next` / `error` / `complete` lifecycle
- [x] Application-level `ping` / `pong`
- [x] Close `4400 InvalidMessage`
- [x] Close `4401 Unauthorized` (subscribe before init)
- [x] Close `4403 Forbidden` (auth hook rejection)
- [x] Close `4408 ConnectionInitTimeout`
- [x] Close `4409 SubscriberAlreadyExists`
- [x] Close `4429 TooManyInitialisationRequests`
- [x] `connection_ack` payload from `OnConnect`
- [x] Subscribe payload validation (non-empty `query`)
- [x] Per-connection context derived from `OnConnect`

### Non-goals

- Apollo `subscriptions-transport-ws` (`graphql-ws`) legacy subprotocol.
  Deprecated in 2021; modern Apollo Client (≥ 3.5), urql, Relay, and Hasura
  clients all default to `graphql-transport-ws`. Clients still on the
  legacy wire format should upgrade rather than be carried indefinitely
  here.

### Security

- [x] Introspection disable flag for production
- [x] Client-facing error message masking
- [x] Internal error redaction with server-side preservation
- [x] Field-level authorization hook
- [x] Operation-level authorization hook
- [x] Trusted documents / operation safelist
- [x] Automatic persisted query safety limits
- [x] Rate limiting hook per operation or client
- [x] Variable value size limits
- [x] Rejection of unknown variables
- [x] Safe panic recovery at the HTTP handler boundary

### Observability

- [x] Parse phase hook
- [x] Validate phase hook
- [x] Execute phase hook
- [ ] Field-level resolver tracing hook
- [ ] OpenTelemetry span emission per operation and field
- [ ] Apollo-compatible tracing extension
- [ ] Prometheus-style metrics hook (count, latency, error rate)
- [ ] Structured operation access logs
- [x] Request ID propagation into resolver context

### Data Loading

- [ ] Batch loader primitive (DataLoader-style)
- [ ] Per-request resolver cache
- [ ] Request-scoped deduplication of identical lookups
- [ ] Batch dispatch tied to execution tick boundaries
- [ ] Typed loader registration keyed to resolver context

## Implementation Plan

The implementation should proceed in thin, test-first phases:

1. Lexer and parser parity: full GraphQL document parsing with locations, including SDL.
2. AST model: complete operation, fragment, value, directive, and type-reference nodes.
3. Validation: implement GraphQL spec validation rules before execution.
4. Execution correctness: aliases, fragments, directives, null bubbling, coercion, operation selection.
5. Type system parity: enums, interfaces (including inheritance), unions, custom scalars, oneof input objects, custom and repeatable directives, descriptions, deprecation, defaults.
6. Introspection parity: implement introspection as normal schema fields, not a broad fast path.
7. HTTP parity: GraphQL-over-HTTP semantics, content negotiation, GET, batching, limits, CORS.
8. Subscriptions and incremental delivery: `graphql-transport-ws`, SSE, `@defer`, `@stream`.
9. Data loading primitives: batching, per-request cache, N+1 mitigation.
10. Observability and security: tracing, metrics, introspection toggle, trusted documents, error masking.
11. Production hardening: benchmarks, memory profiles, fuzzing, concurrency, cancellation, complexity limits.

## Performance Requirements

- Keep hot execution paths allocation-aware.
- Prefer precomputed schema metadata over repeated reflection.
- Parse and validate once where possible, then execute cached prepared operations.
- Avoid global mutable state.
- Keep resolver invocation predictable and type-safe.
- Benchmark every broad execution change before declaring it production-ready.
