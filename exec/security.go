package exec

import (
	"context"

	"github.com/grx-gql/grx/core"
)

// OperationContext describes the selected operation for security hooks.
type OperationContext struct {
	Request core.Request
	Kind    core.OperationKind
	Name    string
}

// FieldAuthorizationContext describes a field about to be resolved.
type FieldAuthorizationContext struct {
	ParentType string
	FieldName  string
	Path       []string
}

// OperationAuthorizer authorizes a parsed operation before execution.
type OperationAuthorizer func(context.Context, OperationContext) error

// FieldAuthorizer authorizes a field before its resolver runs.
type FieldAuthorizer func(context.Context, FieldAuthorizationContext) error

// RateLimiter allows callers to reject an operation based on context, client,
// or operation metadata before execution starts.
type RateLimiter func(context.Context, OperationContext) error
