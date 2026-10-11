package exec

import "github.com/grx-gql/grx/core"

// completionErrorLocations attaches the selecting field to errors raised while
// completing its value, preserving the locations of nested field errors.
func completionErrorLocations(errs []core.Error, location core.Location) []core.Error {
	for index, err := range errs {
		if len(err.Locations) != 0 {
			continue
		}
		out := append([]core.Error(nil), errs...)
		for next := index; next < len(out); next++ {
			if len(out[next].Locations) == 0 {
				out[next].Locations = []core.Location{location}
			}
		}
		return out
	}
	return errs
}
