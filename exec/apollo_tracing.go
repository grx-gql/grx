package exec

import (
	"sync"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/plugin"
)

// apolloTrace accumulates per-request timing for the Apollo Tracing extension
// (https://github.com/apollographql/apollo-tracing). All times are recorded
// relative to start so the response can report nanosecond offsets and durations.
type apolloTrace struct {
	start time.Time

	mu        sync.Mutex
	resolvers []apolloResolverTrace
}

type apolloResolverTrace struct {
	path        []string
	parentType  string
	fieldName   string
	returnType  string
	startOffset time.Duration
	duration    time.Duration
}

func newApolloTrace() *apolloTrace {
	return &apolloTrace{start: time.Now()}
}

// recordApolloFieldTrace appends one resolver timing entry. It is a no-op when
// tracing is not enabled for the request.
func recordApolloFieldTrace(arena *pathArena, field plugin.FieldContext, start, end time.Time) {
	if arena == nil || arena.apollo == nil {
		return
	}
	trace := arena.apollo
	pathCopy := append([]string(nil), field.Path...)
	trace.mu.Lock()
	trace.resolvers = append(trace.resolvers, apolloResolverTrace{
		path:        pathCopy,
		parentType:  field.ParentType,
		fieldName:   field.FieldName,
		returnType:  field.ReturnType,
		startOffset: start.Sub(trace.start),
		duration:    end.Sub(start),
	})
	trace.mu.Unlock()
}

// extension builds the Apollo-tracing "tracing" extension value for a request
// that finished at end.
func (t *apolloTrace) extension(end time.Time) map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()

	resolvers := make([]any, 0, len(t.resolvers))
	for _, r := range t.resolvers {
		path := make([]any, len(r.path))
		for i, p := range r.path {
			path[i] = p
		}
		resolvers = append(resolvers, map[string]any{
			"path":        path,
			"parentType":  r.parentType,
			"fieldName":   r.fieldName,
			"returnType":  r.returnType,
			"startOffset": r.startOffset.Nanoseconds(),
			"duration":    r.duration.Nanoseconds(),
		})
	}

	return map[string]any{
		"version":   1,
		"startTime": t.start.UTC().Format(time.RFC3339Nano),
		"endTime":   end.UTC().Format(time.RFC3339Nano),
		"duration":  end.Sub(t.start).Nanoseconds(),
		"execution": map[string]any{
			"resolvers": resolvers,
		},
	}
}

// attachApolloTracing adds the tracing extension to res when tracing is active.
func attachApolloTracing(arena *pathArena, res core.Response) core.Response {
	if arena == nil || arena.apollo == nil {
		return res
	}
	if res.Extensions == nil {
		res.Extensions = make(map[string]any, 1)
	}
	res.Extensions["tracing"] = arena.apollo.extension(time.Now())
	return res
}
