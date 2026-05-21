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

// WithClientErrorMasking replaces internal execution errors with message in
// client-facing responses while preserving raw errors through plugin.Error.
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
