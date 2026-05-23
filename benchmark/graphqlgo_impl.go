package benchmark

import "github.com/graphql-go/graphql"

// newGraphQLGoSchema builds an equivalent schema via graphql-go/graphql.
func newGraphQLGoSchema() graphql.Schema {
	userType := graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id":    &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"name":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"email": &graphql.Field{Type: graphql.String},
		},
	})

	postType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Post",
		Fields: graphql.Fields{
			"id":    &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"title": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"body":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"author": &graphql.Field{
				Type: graphql.NewNonNull(userType),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if post, ok := p.Source.(*Post); ok {
						return post.Author, nil
					}
					return nil, nil
				},
			},
		},
	})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"user": &graphql.Field{
				Type: userType,
				Args: graphql.FieldConfigArgument{
					// String! matches grx's Go string resolver args (benchmark parity).
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					id, _ := p.Args["id"].(string)
					return fixtureUser(id), nil
				},
			},
			"post": &graphql.Field{
				Type: postType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					id, _ := p.Args["id"].(string)
					return fixturePost(id), nil
				},
			},
			"users": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(userType))),
				Args: graphql.FieldConfigArgument{
					"count": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					count, _ := p.Args["count"].(int)
					return fixtureUsers(count), nil
				},
			},
			"feed": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(postType))),
				Args: graphql.FieldConfigArgument{
					"limit": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					limit, _ := p.Args["limit"].(int)
					return fixtureFeed(limit), nil
				},
			},
		},
	})

	s, err := graphql.NewSchema(graphql.SchemaConfig{Query: queryType})
	if err != nil {
		panic(err)
	}
	return s
}
