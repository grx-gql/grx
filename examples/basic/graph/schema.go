package graph

import "github.com/patrickkabwe/grx/schema"

type Query struct{}

func NewSchema() schema.Config {
	return schema.Config{
		Query: Query{},
	}
}
