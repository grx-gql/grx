// Package graph defines a schema that registers a custom DateTime scalar.
package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/patrickkabwe/grx/schema"
)

type Query struct{}

// DateTime is the Go representation backing the custom GraphQL DateTime scalar.
// Values cross the wire as RFC 3339 strings.
type DateTime struct {
	Time time.Time
}

// Event uses the custom scalar for one of its fields.
type Event struct {
	Name     string   `gql:"name,nonNull"`
	StartsAt DateTime `gql:"startsAt,nonNull"`
}

// NewSchema wires the DateTime scalar into the schema. Parse converts an
// incoming GraphQL value into the Go type; Serialize does the reverse for
// responses.
func NewSchema() schema.Config {
	return schema.Config{
		Query: Query{},
		Scalars: []schema.ScalarConfig{
			{
				Type:           DateTime{},
				Name:           "DateTime",
				SpecifiedByURL: "https://www.rfc-editor.org/rfc/rfc3339",
				Parse: func(input any) (any, error) {
					raw, ok := input.(string)
					if !ok {
						return nil, fmt.Errorf("DateTime must be a string, got %T", input)
					}
					t, err := time.Parse(time.RFC3339, raw)
					if err != nil {
						return nil, fmt.Errorf("invalid DateTime %q: %w", raw, err)
					}
					return DateTime{Time: t}, nil
				},
				Serialize: func(value any) (any, error) {
					dt, ok := value.(DateTime)
					if !ok {
						return nil, fmt.Errorf("expected DateTime, got %T", value)
					}
					return dt.Time.Format(time.RFC3339), nil
				},
			},
		},
	}
}

// NextEvent returns an event whose startsAt field is serialized via the custom
// DateTime scalar. Try: { nextEvent { name startsAt } }
func (Query) NextEvent(ctx context.Context) (*Event, error) {
	return &Event{
		Name:     "GraphQL Meetup",
		StartsAt: DateTime{Time: time.Date(2026, time.June, 1, 18, 30, 0, 0, time.UTC)},
	}, nil
}
