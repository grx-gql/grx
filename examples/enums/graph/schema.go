// Package graph defines a schema that registers a GraphQL enum type.
package graph

import (
	"context"

	"github.com/grx-gql/grx/schema"
)

type Query struct{}

// Episode is a Go string type backing the GraphQL Episode enum. Using a named
// type keeps resolver signatures type-safe.
type Episode string

const (
	EpisodeNewHope Episode = "NEWHOPE"
	EpisodeEmpire  Episode = "EMPIRE"
	EpisodeJedi    Episode = "JEDI"
)

// NewSchema registers the Episode enum, mapping each GraphQL member name to its
// Go value.
func NewSchema() schema.Config {
	return schema.Config{
		Query: Query{},
		Enums: []schema.EnumConfig{
			{
				Type: Episode(""),
				Name: "Episode",
				Values: []schema.EnumValueConfig{
					{Name: "NEWHOPE", Value: EpisodeNewHope},
					{Name: "EMPIRE", Value: EpisodeEmpire},
					{Name: "JEDI", Value: EpisodeJedi},
				},
			},
		},
	}
}

// Favorite echoes the chosen episode, defaulting to JEDI. The enum is accepted
// as an argument and returned as a result. Try: { favorite(episode: EMPIRE) }
func (Query) Favorite(ctx context.Context, args struct {
	Episode Episode `gql:"episode,default=JEDI"`
}) (Episode, error) {
	return args.Episode, nil
}
