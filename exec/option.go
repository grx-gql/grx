package exec

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
