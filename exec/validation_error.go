package exec

import (
	"fmt"

	"github.com/patrickkabwe/grx/core"
)

// validationError is a graphql-js-style validation failure with source location.
type validationError struct {
	message   string
	locations []core.Location
}

func (e validationError) Error() string { return e.message }

func (e validationError) GraphQLLocations() []core.Location { return e.locations }

func newValidationError(loc core.Location, format string, args ...any) validationError {
	return validationError{
		message:   fmt.Sprintf(format, args...),
		locations: []core.Location{loc},
	}
}

func validationResponse(errs []validationError) core.Response {
	out := make([]core.Error, len(errs))
	for i, err := range errs {
		out[i] = core.NewValidationError(err)
	}
	return core.Response{Errors: out}
}
