package benchmark

import (
	graphql "github.com/graph-gophers/graphql-go"
)

// gophers SDL mirrors the benchmark schema.
const gophersSDL = `
schema {
  query: Query
}
type Query {
  user(id: ID!): User
  post(id: ID!): Post
  users(count: Int!): [User!]!
}
type User {
  id: ID!
  name: String!
  email: String
}
type Post {
  id: ID!
  title: String!
  body: String!
  author: User!
}
`

type gophersRoot struct{}

type gophersUserResolver struct{ u *User }

func (r *gophersUserResolver) ID() graphql.ID { return graphql.ID(r.u.ID) }
func (r *gophersUserResolver) Name() string   { return r.u.Name }
func (r *gophersUserResolver) Email() *string { return r.u.Email }

type gophersPostResolver struct{ p *Post }

func (r *gophersPostResolver) ID() graphql.ID { return graphql.ID(r.p.ID) }
func (r *gophersPostResolver) Title() string  { return r.p.Title }
func (r *gophersPostResolver) Body() string   { return r.p.Body }
func (r *gophersPostResolver) Author() *gophersUserResolver {
	return &gophersUserResolver{u: r.p.Author}
}

func (*gophersRoot) User(args struct{ ID graphql.ID }) *gophersUserResolver {
	return &gophersUserResolver{u: fixtureUser(string(args.ID))}
}

func (*gophersRoot) Post(args struct{ ID graphql.ID }) *gophersPostResolver {
	return &gophersPostResolver{p: fixturePost(string(args.ID))}
}

func (*gophersRoot) Users(args struct{ Count int32 }) []*gophersUserResolver {
	src := fixtureUsers(int(args.Count))
	out := make([]*gophersUserResolver, len(src))
	for i, u := range src {
		out[i] = &gophersUserResolver{u: u}
	}
	return out
}

// newGophersSchema parses the SDL and binds the resolver root once.
func newGophersSchema() *graphql.Schema {
	// MaxParallelism(1) keeps execution single-threaded so the benchmark
	// number reflects the executor itself rather than goroutine scheduling
	// noise. graph-gophers spawns a goroutine per field by default.
	s, err := graphql.ParseSchema(gophersSDL, &gophersRoot{}, graphql.MaxParallelism(1))
	if err != nil {
		panic(err)
	}
	return s
}
