package benchmark

import "github.com/graphql-go/graphql"

// newGraphQLGoSchema builds the equivalent schema using the code-first
// graphql-go/graphql library. Idiomatic usage: explicit Object/Field
// definitions and per-field Resolve callbacks. Resolvers return Go values
// from the shared fixture so the comparison measures library overhead.
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
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					id, _ := p.Args["id"].(string)
					return fixtureUser(id), nil
				},
			},
			"post": &graphql.Field{
				Type: postType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
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
		},
	})

	// graphql-go's default field resolver reads exported struct fields by
	// name, so User fields don't need explicit resolvers. The wrapping
	// types, however, expose the data as *User pointers from the shared
	// fixture, so we add an Email accessor only if needed via reflection
	// fall-through provided by graphql-go.

	s, err := graphql.NewSchema(graphql.SchemaConfig{Query: queryType})
	if err != nil {
		panic(err)
	}
	return s
}
