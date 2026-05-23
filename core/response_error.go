package core

// LocationProvider is implemented by errors tied to a position in the
// GraphQL document. Per GraphQL spec Section 7, such errors include a
// locations entry in the response.
type LocationProvider interface {
	GraphQLLocations() []Location
}

const (
	// ErrorCodeValidationFailed matches graphql-js validation error codes.
	ErrorCodeValidationFailed = "GRAPHQL_VALIDATION_FAILED"
)

// NewRequestError builds a request-level GraphQL error per spec Section 7.
// Request error results contain errors but no data entry.
func NewRequestError(err error) Error {
	result := Error{
		Message: err.Error(),
		Extensions: map[string]any{
			"classification": "request",
		},
	}
	if provider, ok := err.(LocationProvider); ok {
		result.Locations = provider.GraphQLLocations()
	}
	return result
}

// NewValidationError builds a request-level validation error using graphql-js
// conventions (GRAPHQL_VALIDATION_FAILED in extensions).
func NewValidationError(err error) Error {
	result := NewRequestError(err)
	if result.Extensions == nil {
		result.Extensions = map[string]any{}
	}
	result.Extensions["code"] = ErrorCodeValidationFailed
	return result
}

// NewFieldError builds a field execution error with path and optional
// source location per spec Section 7.
func NewFieldError(message string, path []any, location Location) Error {
	pathCopy := path
	if len(path) > 0 {
		pathCopy = append([]any(nil), path...)
	}

	result := Error{
		Message: message,
		Path:    pathCopy,
		Extensions: map[string]any{
			"classification": "field",
		},
	}
	if location.Line > 0 && location.Column > 0 {
		result.Locations = []Location{location}
	}
	return result
}

// AttachRequestIDExtension adds requestId to res.Extensions when id is
// non-empty, per the GraphQL spec optional top-level extensions entry.
func AttachRequestIDExtension(res Response, id string) Response {
	if id == "" {
		return res
	}
	if res.Extensions == nil {
		res.Extensions = map[string]any{}
	}
	res.Extensions["requestId"] = id
	return res
}
