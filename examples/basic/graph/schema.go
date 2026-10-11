package graph

import "github.com/grx-gql/grx/schema"

type Query struct{}

func Schema() schema.Config {
	return schema.Config{
		Query: Query{},
	}
}
