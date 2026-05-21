package exec

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

type testUser struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name,nonNull"`
}

type testEpisode string

const (
	testEpisodeNewHope testEpisode = "NEWHOPE"
	testEpisodeEmpire  testEpisode = "EMPIRE"
	testEpisodeJedi    testEpisode = "JEDI"
)

type testDate struct {
	Raw string
}

type testNode interface {
	isTestNode()
}

type testSearchResult interface {
	isTestSearchResult()
}

type testNodeUser struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name,nonNull"`
}

func (*testNodeUser) isTestNode() {}

func (*testNodeUser) isTestSearchResult() {}

type testNodePost struct {
	ID    string `gql:"id,nonNull"`
	Title string `gql:"title,nonNull"`
}

func (*testNodePost) isTestNode() {}

func (*testNodePost) isTestSearchResult() {}

type testQuery struct{}

type testAdvancedQuery struct {
	testQuery
}

type testCountingQuery struct {
	count *int
}

type testInvalidOutputQuery struct{}

type testMutation struct{}

type testUserCreateInput struct {
	Name string `gql:"name,nonNull"`
}

type testUserCreateArgs struct {
	Input testUserCreateInput `gql:"input,nonNull"`
}

type testUserCreatePayload struct {
	User *testUser `gql:"user,nonNull"`
}

type testDefaultInput struct {
	Query string `gql:"query,default=all"`
	Limit int    `gql:"limit,default=10"`
}

func (testQuery) User(ctx context.Context, args struct {
	ID string `gql:"id,nonNull"`
}) (*testUser, error) {
	return &testUser{ID: args.ID, Name: "Ada"}, nil
}

func (q testCountingQuery) User(ctx context.Context, args struct {
	ID string `gql:"id,nonNull"`
}) (*testUser, error) {
	*q.count = *q.count + 1
	return &testUser{ID: args.ID, Name: "Ada"}, nil
}

func (testInvalidOutputQuery) Count(ctx context.Context) (int, error) {
	return 65, nil
}

func (testAdvancedQuery) User(ctx context.Context, args struct {
	ID string `gql:"id,nonNull"`
}) (*testUser, error) {
	return testQuery{}.User(ctx, args)
}

func (testAdvancedQuery) FavoriteEpisode(ctx context.Context, args struct {
	Episode testEpisode `gql:"episode,default=JEDI"`
}) (testEpisode, error) {
	return args.Episode, nil
}

func (testAdvancedQuery) EchoDate(ctx context.Context, args struct {
	At testDate `gql:"at,default=2026-05-01"`
}) (testDate, error) {
	return args.At, nil
}

func (testAdvancedQuery) Node(ctx context.Context, args struct {
	Kind string `gql:"kind,default=user"`
}) (testNode, error) {
	if args.Kind == "post" {
		return &testNodePost{ID: "post_1", Title: "GraphQL"}, nil
	}
	return &testNodeUser{ID: "user_1", Name: "Ada"}, nil
}

func (testAdvancedQuery) Search(ctx context.Context, args struct {
	Kind string `gql:"kind,default=user"`
}) (testSearchResult, error) {
	if args.Kind == "post" {
		return &testNodePost{ID: "post_1", Title: "GraphQL"}, nil
	}
	return &testNodeUser{ID: "user_1", Name: "Ada"}, nil
}

func (testAdvancedQuery) Defaulted(ctx context.Context, args struct {
	Input testDefaultInput `gql:"input,nonNull"`
}) (string, error) {
	return fmt.Sprintf("%s:%d", args.Input.Query, args.Input.Limit), nil
}

func (testMutation) CreateUser(ctx context.Context, args testUserCreateArgs) (*testUserCreatePayload, error) {
	return &testUserCreatePayload{User: &testUser{ID: "user_1", Name: args.Input.Name}}, nil
}

func TestExecutorResolvesNestedSelection(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query:     `query GetUser($id: String!) { user(id: $id) { id name } }`,
		Variables: map[string]any{"id": "1"},
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)

	user := responseObject(t, data["user"])
	if user["id"] != "1" {
		t.Fatalf("expected id 1, got %#v", user["id"])
	}
	if user["name"] != "Ada" {
		t.Fatalf("expected name Ada, got %#v", user["name"])
	}
}

func TestExecutorBindsNestedInputObject(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}, Mutation: testMutation{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `mutation CreateUser($input: UserCreateInput!) {
			createUser(input: $input) {
				user {
					id
					name
				}
			}
		}`,
		Variables: map[string]any{
			"input": map[string]any{"name": "Grace"},
		},
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)

	payload := responseObject(t, data["createUser"])

	user := responseObject(t, payload["user"])
	if user["name"] != "Grace" {
		t.Fatalf("expected name Grace, got %#v", user["name"])
	}
}

func TestExecutorBindsInlineInputObjectLiteral(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}, Mutation: testMutation{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `mutation MyMutation {
			__typename
			createUser(input: {name: "test"}) {
				user {
					id
					name
				}
			}
		}`,
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	payload := responseObject(t, data["createUser"])
	user := responseObject(t, payload["user"])
	if user["name"] != "test" {
		t.Fatalf("expected name test, got %#v", user["name"])
	}
}

func TestExecutorMergesDuplicateResponseFields(t *testing.T) {
	count := 0
	schemaValue, err := schema.Build(schema.Config{Query: testCountingQuery{count: &count}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ user(id: "1") { id } user(id: "1") { name } }`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}
	if count != 1 {
		t.Fatalf("expected one resolver call, got %d", count)
	}

	data := responseObject(t, response.Data)
	user := responseObject(t, data["user"])
	if user["id"] != "1" || user["name"] != "Ada" {
		t.Fatalf("expected merged user fields, got %#v", user)
	}
}

func TestExecutorSupportsInlineFragmentWithoutTypeCondition(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ user(id: "1") { ... { id name } } }`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	user := responseObject(t, data["user"])
	if user["id"] != "1" || user["name"] != "Ada" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestExecutorRejectsInvalidScalarInputBeforeResolver(t *testing.T) {
	called := false
	query := testCountingQuery{count: new(int)}
	schemaValue, err := schema.Build(schema.Config{Query: query})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	schemaValue.Query.Fields["user"].Resolver = func(ctx context.Context, params schema.ResolveParams) (any, error) {
		called = true
		return &testUser{ID: "bad", Name: "bad"}, nil
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query:     `query($id: String!) { user(id: $id) { id } }`,
		Variables: map[string]any{"id": 65},
	})
	if len(response.Errors) == 0 {
		t.Fatal("expected scalar input error")
	}
	if called {
		t.Fatal("resolver should not be called for invalid input")
	}
}

func TestExecutorRejectsInvalidScalarOutput(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testInvalidOutputQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	schemaValue.Query.Fields["count"].Type = &schema.Scalar{TypeName: "String"}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{Query: `{ count }`})
	if len(response.Errors) == 0 {
		t.Fatal("expected scalar output error")
	}
}

func TestExecutorRejectsSelectionLimit(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil, WithMaxSelectionCount(1))
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ user(id: "1") { id } }`,
	})
	if len(response.Errors) == 0 {
		t.Fatal("expected selection limit error")
	}
}

func TestExecutorCachesNoVariableDocuments(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil, WithDocumentCache(2))
	for index := 0; index < 2; index++ {
		response := executor.Execute(context.Background(), core.Request{
			Query: `{ user(id: "1") { id } }`,
		})
		if len(response.Errors) != 0 {
			t.Fatalf("unexpected errors: %#v", response.Errors)
		}
	}
	if len(executor.documentCache) != 1 {
		t.Fatalf("expected one cached document, got %d", len(executor.documentCache))
	}
}

func TestExecutorHandlesSchemaIntrospectionQueryWithFragments(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}, Mutation: testMutation{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `query IntrospectionQuery {
			__schema {
				queryType { name }
				mutationType { name }
				types { ...FullType }
				directives { name }
			}
		}

		fragment FullType on __Type {
			kind
			name
			fields(includeDeprecated: true) {
				name
				args { name }
				type { kind name ofType { kind name } }
			}
		}`,
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)

	schemaData := responseObject(t, data["__schema"])

	queryType := responseObject(t, schemaData["queryType"])
	if queryType["name"] != "Query" {
		t.Fatalf("expected Query root, got %#v", queryType["name"])
	}

	types, ok := schemaData["types"].([]any)
	if !ok {
		t.Fatalf("expected types list, got %T", schemaData["types"])
	}

	userField := introspectionField(t, types, "Query", "user")
	args, ok := userField["args"].([]any)
	if !ok {
		t.Fatalf("expected user args list, got %T", userField["args"])
	}
	if len(args) != 1 {
		t.Fatalf("expected one user arg, got %#v", args)
	}

	idArg, ok := args[0].(map[string]any)
	if !ok {
		idArg, ok = responseObjectValue(args[0])
	}
	if !ok {
		t.Fatalf("expected id arg map, got %T", args[0])
	}
	if idArg["name"] != "id" {
		t.Fatalf("expected id arg, got %#v", idArg["name"])
	}

	createUserField := introspectionField(t, types, "Mutation", "createUser")
	createUserArgs, ok := createUserField["args"].([]any)
	if !ok {
		t.Fatalf("expected createUser args list, got %T", createUserField["args"])
	}
	if len(createUserArgs) != 1 {
		t.Fatalf("expected one createUser arg, got %#v", createUserArgs)
	}
}

func TestExecutorRejectsIntrospectionWhenDisabled(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}, Mutation: testMutation{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil, WithDisableIntrospection())
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ __schema { queryType { name } } }`,
	})

	if len(response.Errors) != 1 {
		t.Fatalf("expected one error, got %#v", response.Errors)
	}
	if !strings.Contains(response.Errors[0].Message, "introspection is disabled") {
		t.Fatalf("unexpected error: %#v", response.Errors[0].Message)
	}
}

func introspectionField(t *testing.T, types []any, typeName string, fieldName string) map[string]any {
	t.Helper()

	for _, rawType := range types {
		typeValue, ok := responseObjectValue(rawType)
		if !ok || typeValue["name"] != typeName {
			continue
		}

		fields, ok := typeValue["fields"].([]any)
		if !ok {
			t.Fatalf("expected fields list for %s, got %T", typeName, typeValue["fields"])
		}
		for _, rawField := range fields {
			field, ok := responseObjectValue(rawField)
			if ok && field["name"] == fieldName {
				return field
			}
		}
	}

	t.Fatalf("expected field %s.%s", typeName, fieldName)
	return nil
}

func testAdvancedSchema(t *testing.T) *schema.Schema {
	t.Helper()

	schemaValue, err := schema.Build(schema.Config{
		Query: testAdvancedQuery{},
		Scalars: []schema.ScalarConfig{
			{
				Type: testDate{},
				Name: "Date",
				Parse: func(input any) (any, error) {
					value, ok := input.(string)
					if !ok {
						return nil, fmt.Errorf("expected string date input, got %T", input)
					}
					return testDate{Raw: value}, nil
				},
				Serialize: func(value any) (any, error) {
					date, ok := value.(testDate)
					if !ok {
						return nil, fmt.Errorf("expected testDate value, got %T", value)
					}
					return date.Raw, nil
				},
			},
		},
		Enums: []schema.EnumConfig{
			{
				Type: testEpisode(""),
				Name: "Episode",
				Values: []schema.EnumValueConfig{
					{Name: "NEWHOPE", Value: testEpisodeNewHope},
					{Name: "EMPIRE", Value: testEpisodeEmpire},
					{Name: "JEDI", Value: testEpisodeJedi},
				},
			},
		},
		Interfaces: []schema.InterfaceConfig{
			{
				Type:         (*testNode)(nil),
				Implementors: []any{testNodeUser{}, testNodePost{}},
			},
		},
		Unions: []schema.UnionConfig{
			{
				Type:         (*testSearchResult)(nil),
				Name:         "SearchResult",
				Implementors: []any{testNodeUser{}, testNodePost{}},
			},
		},
	})
	if err != nil {
		t.Fatalf("build advanced schema: %v", err)
	}

	return schemaValue
}

func TestExecutorBindsEnumArgumentAndSerializesEnumValue(t *testing.T) {
	executor := New(testAdvancedSchema(t), nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `query FavoriteEpisode($episode: Episode!) {
			favoriteEpisode(episode: $episode)
		}`,
		Variables: map[string]any{"episode": "EMPIRE"},
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	if data["favoriteEpisode"] != "EMPIRE" {
		t.Fatalf("expected EMPIRE, got %#v", data["favoriteEpisode"])
	}
}

func TestExecutorUsesCustomScalarParseAndSerialize(t *testing.T) {
	executor := New(testAdvancedSchema(t), nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `query EchoDate($at: Date!) {
			echoDate(at: $at)
		}`,
		Variables: map[string]any{"at": "2026-05-02"},
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	if data["echoDate"] != "2026-05-02" {
		t.Fatalf("expected serialized date, got %#v", data["echoDate"])
	}
}

func TestExecutorResolvesInterfaceSelectionAndTypename(t *testing.T) {
	executor := New(testAdvancedSchema(t), nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `query Node($kind: String!) {
			node(kind: $kind) {
				__typename
				id
			}
		}`,
		Variables: map[string]any{"kind": "user"},
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	node := responseObject(t, data["node"])
	if node["__typename"] != "testNodeUser" {
		t.Fatalf("expected concrete typename, got %#v", node["__typename"])
	}
	if node["id"] != "user_1" {
		t.Fatalf("expected user_1 id, got %#v", node["id"])
	}
}

func TestExecutorResolvesUnionTypename(t *testing.T) {
	executor := New(testAdvancedSchema(t), nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `query Search($kind: String!) {
			search(kind: $kind) {
				__typename
			}
		}`,
		Variables: map[string]any{"kind": "post"},
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	search := responseObject(t, data["search"])
	if search["__typename"] != "testNodePost" {
		t.Fatalf("expected concrete typename, got %#v", search["__typename"])
	}
}

func TestExecutorAppliesDefaultValues(t *testing.T) {
	executor := New(testAdvancedSchema(t), nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `query Defaults {
			favoriteEpisode
			echoDate
			defaulted(input: {})
		}`,
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	if data["favoriteEpisode"] != "JEDI" {
		t.Fatalf("expected enum default JEDI, got %#v", data["favoriteEpisode"])
	}
	if data["echoDate"] != "2026-05-01" {
		t.Fatalf("expected scalar default 2026-05-01, got %#v", data["echoDate"])
	}
	if data["defaulted"] != "all:10" {
		t.Fatalf("expected nested defaults all:10, got %#v", data["defaulted"])
	}
}

func TestExecutorUsesFieldAliasInResponse(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ u: user(id: "7") { n: name } }`,
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	u := responseObject(t, data["u"])
	if u["n"] != "Ada" {
		t.Fatalf("expected nested alias n=Ada, got %#v", u["n"])
	}
}

func TestExecutorSkipIncludeOmitsFields(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `query Q($skipUser: Boolean!, $includeName: Boolean!) {
			a: user(id: "1") @skip(if: $skipUser) { id }
			b: user(id: "1") @include(if: $includeName) { id }
		}`,
		Variables: map[string]any{"skipUser": true, "includeName": false},
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	if _, ok := data["a"]; ok {
		t.Fatalf("expected skipped field omitted, got %#v", data["a"])
	}
	if _, ok := data["b"]; ok {
		t.Fatalf("expected excluded field omitted, got %#v", data["b"])
	}
}

func TestExecutorNamedFragmentSpread(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `
			fragment UserFrag on Query {
				user(id: "1") { id name }
			}
			query {
				...UserFrag
			}
		`,
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	user := responseObject(t, data["user"])
	if user["name"] != "Ada" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestExecutorUnknownFragmentProducesError(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ ...Missing }`,
	})

	if len(response.Errors) == 0 {
		t.Fatalf("expected errors, got data %#v", response.Data)
	}
	if !strings.Contains(response.Errors[0].Message, `Unknown fragment "Missing".`) {
		t.Fatalf("unexpected error: %#v", response.Errors)
	}
}

func TestExecutorRejectsSelectionBeyondMaxDepth(t *testing.T) {
	type nestUser struct {
		ID string `gql:"id,nonNull"`
	}
	type nestPost struct {
		Author *nestUser `gql:"author,nonNull"`
	}
	type nestQuery struct{}

	schemaValue, err := schema.Build(schema.Config{
		Query: struct {
			Post func(context.Context, struct {
				ID string `gql:"id,nonNull"`
			}) (*nestPost, error)
		}{
			Post: func(ctx context.Context, args struct {
				ID string `gql:"id,nonNull"`
			}) (*nestPost, error) {
				return &nestPost{Author: &nestUser{ID: "1"}}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil, WithMaxSelectionDepth(2))
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ post(id: "1") { author { id } } }`,
	})

	if len(response.Errors) == 0 {
		t.Fatal("expected parse depth error")
	}
	if !strings.Contains(response.Errors[0].Message, "selection depth exceeds limit") {
		t.Fatalf("unexpected error: %#v", response.Errors)
	}
}

func responseObject(t *testing.T, value any) map[string]any {
	t.Helper()

	object, ok := responseObjectValue(value)
	if !ok {
		t.Fatalf("expected response object, got %T", value)
	}
	return object
}

func responseObjectValue(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case *core.OrderedObject:
		return typed.Map(), true
	default:
		return nil, false
	}
}
