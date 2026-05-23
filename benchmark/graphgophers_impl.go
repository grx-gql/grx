package benchmark

import graphql "github.com/graph-gophers/graphql-go"

const gophersSDL = `
schema {
  query: Query
}
type Query {
  user(id: String!): User
  post(id: String!): Post
  users(count: Int!): [User!]!
  feed(limit: Int!): [Post!]!
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

func (*gophersRoot) User(args struct{ ID string }) *gophersUserResolver {
	return &gophersUserResolver{u: fixtureUser(args.ID)}
}

func (*gophersRoot) Post(args struct{ ID string }) *gophersPostResolver {
	return &gophersPostResolver{p: fixturePost(args.ID)}
}

func (*gophersRoot) Users(args struct{ Count int32 }) []*gophersUserResolver {
	src := fixtureUsers(int(args.Count))
	out := make([]*gophersUserResolver, len(src))
	for i, u := range src {
		out[i] = &gophersUserResolver{u: u}
	}
	return out
}

func (*gophersRoot) Feed(args struct{ Limit int32 }) []*gophersPostResolver {
	src := fixtureFeed(int(args.Limit))
	out := make([]*gophersPostResolver, len(src))
	for i, p := range src {
		out[i] = &gophersPostResolver{p: p}
	}
	return out
}

func newGophersSchema() *graphql.Schema {
	// Single-threaded resolver fan-out for apples-to-apples vs grx.
	s, err := graphql.ParseSchema(gophersSDL, &gophersRoot{}, graphql.MaxParallelism(1))
	if err != nil {
		panic(err)
	}
	return s
}
