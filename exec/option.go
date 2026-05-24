package exec

import "strings"

// ExecutorOption configures an [Executor] created by [New].
type ExecutorOption func(*Executor)

// WithDisableIntrospection rejects [Executor.Execute] requests whose document
// selects __schema or __type when true (GraphiQL and other clients receive a
// request-level GraphQL error).
func WithDisableIntrospection() ExecutorOption {
	return func(e *Executor) {
		e.disableIntrospection = true
	}
}

// WithMaxSelectionDepth sets a maximum nesting depth for selection sets in
// parsed documents. Zero disables the limit.
func WithMaxSelectionDepth(depth int) ExecutorOption {
	return func(e *Executor) {
		e.maxSelectionDepth = depth
	}
}

// WithMaxSelectionCount limits the total selections in an operation. Zero
// disables the limit.
func WithMaxSelectionCount(count int) ExecutorOption {
	return func(e *Executor) {
		e.maxSelectionCount = count
	}
}

// WithMaxAliasCount limits aliased fields in an operation. Zero disables the
// limit.
func WithMaxAliasCount(count int) ExecutorOption {
	return func(e *Executor) {
		e.maxAliasCount = count
	}
}

// WithMaxRootFieldCount limits top-level fields in an operation. Zero disables
// the limit.
func WithMaxRootFieldCount(count int) ExecutorOption {
	return func(e *Executor) {
		e.maxRootFieldCount = count
	}
}

// WithDocumentCache caches parsed documents for requests without variables.
// Variable-bearing requests are intentionally not cached because variable
// defaults and substitutions are currently applied during parsing.
func WithDocumentCache(limit int) ExecutorOption {
	return func(e *Executor) {
		e.documentCacheLimit = limit
	}
}

// WithLexerCache keeps an LRU map of lexical token streams keyed by normalized
// query source. Parsing reuses immutable []token snapshots so HTTP stacks that
// call [Executor.OperationKind] before [Executor.Execute] do not lex the query
// twice.
//
// The token cache applies to every execution path (including GraphQL queries
// with variables). Queries that differ only in variable values share the same
// lexical stream.
func WithLexerCache(limit int) ExecutorOption {
	return func(e *Executor) {
		e.lexCacheLimit = limit
	}
}

// WithClientErrorMasking replaces internal execution errors with message in
// client-facing responses while preserving raw errors through plugins.Error.
func WithClientErrorMasking(message string) ExecutorOption {
	return func(e *Executor) {
		e.maskInternalErrors = true
		e.clientErrorMessage = message
	}
}

// WithOperationAuthorizer installs an operation-level authorization hook.
func WithOperationAuthorizer(authorizer OperationAuthorizer) ExecutorOption {
	return func(e *Executor) {
		e.operationAuthorizer = authorizer
	}
}

// WithFieldAuthorizer installs a field-level authorization hook.
func WithFieldAuthorizer(authorizer FieldAuthorizer) ExecutorOption {
	return func(e *Executor) {
		e.fieldAuthorizer = authorizer
	}
}

// WithRateLimiter installs a per-operation rate limiting hook.
func WithRateLimiter(limiter RateLimiter) ExecutorOption {
	return func(e *Executor) {
		e.rateLimiter = limiter
	}
}

// WithTrustedDocuments allows only documents whose SHA-256 hash appears in
// trusted. Map values may be empty, or the exact query expected for the hash.
func WithTrustedDocuments(trusted map[string]string) ExecutorOption {
	return func(e *Executor) {
		if len(trusted) == 0 {
			return
		}
		e.trustedDocuments = make(map[string]string, len(trusted))
		for hash, query := range trusted {
			normalized := strings.ToLower(strings.TrimSpace(hash))
			if normalized != "" {
				e.trustedDocuments[normalized] = query
			}
		}
	}
}

// WithRejectUnknownVariables rejects JSON variables that are not declared by
// the selected operation.
func WithRejectUnknownVariables() ExecutorOption {
	return func(e *Executor) {
		e.rejectUnknownVars = true
	}
}

// WithResolverCache enables request-scoped resolver memoization. Within a single
// query operation, identical resolver invocations (same field, same source
// value, same arguments) run once and reuse the result. Mutations and
// subscriptions are never cached because their resolvers may have side effects.
func WithResolverCache() ExecutorOption {
	return func(e *Executor) {
		e.resolverCacheEnabled = true
	}
}

// WithApolloTracing attaches an Apollo-tracing-format "tracing" object to the
// response extensions, recording overall request timing and per-resolver
// startOffset/duration. It adds no third-party dependencies; consumers such as
// Apollo Studio or apollo-tracing-aware tools read the extension directly.
func WithApolloTracing() ExecutorOption {
	return func(e *Executor) {
		e.apolloTracing = true
	}
}

// WithExecutableIntrospection runs introspection (__schema/__type) through the
// normal selection-execution path instead of the built-in fast path. Field
// hooks (plugins, field authorizer) then observe introspection fields and the
// response honors the exact selection set. The fast path remains the default to
// preserve its performance characteristics.
func WithExecutableIntrospection() ExecutorOption {
	return func(e *Executor) {
		e.executableIntrospection = true
	}
}

// AbstractTypeResolver returns the concrete GraphQL type name for a value of an
// interface or union type. It lets applications override the default
// reflection-based resolution (for example, when one Go type backs several
// GraphQL types). Returning an empty name falls back to default resolution.
type AbstractTypeResolver func(value any) (string, error)

// WithAbstractTypeResolver installs a runtime hook consulted to resolve the
// concrete object type for interface and union values before the built-in
// reflection-based resolver.
func WithAbstractTypeResolver(resolver AbstractTypeResolver) ExecutorOption {
	return func(e *Executor) {
		e.abstractTypeResolver = resolver
	}
}
