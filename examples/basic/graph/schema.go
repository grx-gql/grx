package graph

import "github.com/grx-gql/grx/schema"

type Query struct{}

func NewSchema() schema.Config {
	return schema.Config{
		Query: Query{},
	}
}
