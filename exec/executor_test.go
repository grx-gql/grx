package exec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/plugin"
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

type orderedListRow struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name"`
}

type orderedListQuery struct{}

func (orderedListQuery) Items(context.Context) ([]*orderedListRow, error) {
	return []*orderedListRow{
		{ID: "1", Name: "Ada"},
		{ID: "2", Name: "Grace"},
	}, nil
}

type testDate struct {
	Raw string
}

type testNode interface {
	isTestNode()
}

type testSearchResult interface {
	isTestSearchResult()
}

// passthroughSchemaLeaf satisfies schema.Type without being one of the
// executor's typed schema wrappers; completion falls through completeValue's
// default branch.
type passthroughSchemaLeaf struct{}

func (passthroughSchemaLeaf) Name() string { return "PassthroughLeaf" }

func (passthroughSchemaLeaf) Kind() schema.Kind { return schema.ScalarKind }

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

func (testAdvancedQuery) InvalidEpisode(context.Context) (testEpisode, error) {
	// Registered GraphQL Episode values are NEWHOPE/EMPIRE/JEDI — this triggers Enum.Serialize errors.
	return "NOT_A_MEMBER", nil
}

func (testAdvancedQuery) NonFiniteFloat(context.Context) (float64, error) {
	return math.NaN(), nil
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
		Query: `mutation CreateUser($input: testUserCreateInput!) {
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

func TestExecutorEvictsParsedDocumentsWhenCacheIsFull(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil, WithDocumentCache(2))
	for _, query := range []string{
		`{ user(id: "1") { id } }`,
		`{ user(id: "2") { id name } }`,
		`{ user(id: "3") { id name } }`,
	} {
		resp := executor.Execute(context.Background(), core.Request{Query: query})
		if len(resp.Errors) != 0 {
			t.Fatalf("%q unexpected errors: %#v", query, resp.Errors)
		}
	}
	if len(executor.documentCache) != 2 {
		t.Fatalf("expected cache eviction to cap at two entries, got %d keys", len(executor.documentCache))
	}
	if len(executor.documentCacheOrder) != 2 {
		t.Fatalf("expected cache order to track two entries, got %v", executor.documentCacheOrder)
	}
}

func TestLexerCacheSharedAcrossOperationKindAndExecute(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	query := `{ user(id: "1") { id } }`
	executor := New(schemaValue, nil, WithLexerCache(4))

	kind, kindErr := executor.OperationKind(core.Request{Query: query})
	if kindErr != nil {
		t.Fatalf("OperationKind: %v", kindErr)
	}
	if kind != core.OperationQuery {
		t.Fatalf("unexpected kind %v", kind)
	}
	if len(executor.lexTokenCache) != 1 {
		t.Fatalf("expected one lexical cache entry after OperationKind, got %d", len(executor.lexTokenCache))
	}

	res := executor.Execute(context.Background(), core.Request{Query: query})
	if len(res.Errors) != 0 {
		t.Fatalf("Execute: %+v", res.Errors)
	}
	if len(executor.lexTokenCache) != 1 {
		t.Fatalf("expected one lexical cache entry after Execute, got %d", len(executor.lexTokenCache))
	}
}

func TestLexerCacheEvictsWhenOperationKindPassesExhaustLimit(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	executor := New(schemaValue, nil, WithLexerCache(2))
	queries := []string{
		`query Ka { __typename user(id:"1"){ id } }`,
		`query Kb { __typename user(id:"2"){ id } }`,
		`query Kc { __typename user(id:"3"){ id } }`,
	}
	for _, q := range queries {
		if _, err := executor.OperationKind(core.Request{Query: q}); err != nil {
			t.Fatalf("OperationKind %q: %v", q, err)
		}
	}
	if len(executor.lexTokenCache) != 2 || len(executor.lexCacheOrder) != 2 {
		t.Fatalf("expected LRU to cap lexical cache at 2 entries, got %d keys order=%d",
			len(executor.lexTokenCache), len(executor.lexCacheOrder))
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

func TestExecutorCompletesSliceOfPointersViaOrderedObjectLeafFastPath(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	ex := New(schemaValue, nil)
	resp := ex.Execute(context.Background(), core.Request{Query: `{ items { id name } }`})
	if len(resp.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", resp.Errors)
	}

	data := responseObject(t, resp.Data)
	itemsRaw, ok := data["items"]
	if !ok {
		t.Fatalf("missing items: %#v", data)
	}
	items, ok := itemsRaw.([]*core.OrderedObject)
	if !ok || len(items) != 2 {
		t.Fatalf("expected []*OrderedObject items, got %#v (%T)", itemsRaw, itemsRaw)
	}
	first := items[0]
	got := first.Map()
	if got["id"] != "1" || got["name"] != "Ada" {
		t.Fatalf("unexpected first row %#v", got)
	}
}

func TestFieldAuthorizerSeesDistinctPathsForListLeafFields(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	var idPaths [][]string
	auth := func(_ context.Context, fc FieldAuthorizationContext) error {
		if fc.FieldName == "id" {
			idPaths = append(idPaths, slices.Clone(fc.Path))
		}
		return nil
	}
	ex := New(s, nil, WithFieldAuthorizer(auth))
	resp := ex.Execute(context.Background(), core.Request{Query: `{ items { id name } }`})
	if len(resp.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", resp.Errors)
	}
	if len(idPaths) < 2 {
		t.Fatalf("expected authorizer callbacks for multiple list elements, got %d path(s): %#v", len(idPaths), idPaths)
	}
	for index := range idPaths {
		foundItems := slices.Contains(idPaths[index], "items")
		if !foundItems {
			t.Fatalf("expected hook path %#v to include list field segment %q (see FieldAuthorizationContext.Path)", idPaths[index], "items")
		}
	}
	for i := 0; i < len(idPaths); i++ {
		for j := i + 1; j < len(idPaths); j++ {
			if slices.Equal(idPaths[i], idPaths[j]) {
				t.Fatalf("expected distinct FieldAuthorizationContext.Path slices for each list row (got duplicate %#v)", idPaths[i])
			}
		}
	}
	data := responseObject(t, resp.Data)
	itemsRaw, ok := data["items"]
	if !ok {
		t.Fatalf("missing items: %#v", data)
	}
	items, ok := itemsRaw.([]*core.OrderedObject)
	if !ok || len(items) != 2 {
		t.Fatalf("expected list result, got %#v (%T)", itemsRaw, itemsRaw)
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

func TestExecutorInternalHelpers(t *testing.T) {
	if key, ok := sourceIdentity("root"); !ok || key == "" {
		t.Fatalf("string source identity = %q %v", key, ok)
	}
	ch := make(chan int)
	if key, ok := sourceIdentity(ch); !ok || key == "" {
		t.Fatalf("chan source identity = %q %v", key, ok)
	}
	if _, ok := sourceIdentity(struct{}{}); ok {
		t.Fatal("struct value should not have stable identity")
	}
	field := &schema.Field{Name: "field"}
	if key, ok := resolverCacheKey(field, "src", map[string]any{"a": 1}); !ok || key == "" {
		t.Fatalf("cache key = %q %v", key, ok)
	}

	if got := pathSegmentString(2); got != "2" {
		t.Fatalf("path segment = %q", got)
	}
	if got := pathSegmentString(int64(3)); got != "3" {
		t.Fatalf("path segment = %q", got)
	}
	if got := pathSegmentString(uint(4)); got != "4" {
		t.Fatalf("path segment = %q", got)
	}
	if got := pathSegmentString(true); got != "true" {
		t.Fatalf("path segment = %q", got)
	}
	for _, segment := range []any{int8(1), int16(2), int32(3), uint8(4), uint16(5), uint32(6), uint64(7), struct{ A int }{A: 1}} {
		if pathSegmentString(segment) == "" {
			t.Fatalf("empty path segment for %#v", segment)
		}
	}

	stringType := &schema.Scalar{TypeName: "String"}
	item, ok := listItemType(&schema.NonNull{OfType: &schema.List{OfType: stringType}})
	if !ok || item == nil {
		t.Fatalf("list item = %#v %v", item, ok)
	}
	if _, ok := listItemType(stringType); ok {
		t.Fatal("scalar should not be a list")
	}

	if clonePath(nil) != nil {
		t.Fatal("nil path should clone to nil")
	}
	if got := clonePath([]any{"a", 1}); !reflect.DeepEqual(got, []any{"a", 1}) {
		t.Fatalf("clone = %#v", got)
	}

	obj := &schema.Object{TypeName: "Obj"}
	if leaf, ok := schemaObjectLeaf(&schema.NonNull{OfType: obj}); !ok || leaf != obj {
		t.Fatalf("schema leaf = %#v %v", leaf, ok)
	}
	if _, ok := schemaObjectLeaf(stringType); ok {
		t.Fatal("scalar should not be object leaf")
	}
	if masked := (&Executor{maskInternalErrors: true}).maskError(errors.New("secret"), true); masked.Error() != "internal server error" {
		t.Fatalf("masked = %v", masked)
	}
}

type hookPlugin struct {
	plugin.Base
	errAt string
	seen  []string
}

func (p *hookPlugin) RequestStart(ctx context.Context, req core.Request) (context.Context, error) {
	p.seen = append(p.seen, "request")
	if p.errAt == "request" {
		return ctx, errors.New("request failed")
	}
	return ctx, nil
}

func (p *hookPlugin) ParsingStart(context.Context, core.Request) error {
	p.seen = append(p.seen, "parsing")
	if p.errAt == "parsing" {
		return errors.New("parsing failed")
	}
	return nil
}

func (p *hookPlugin) ValidationStart(context.Context, core.Request) error {
	p.seen = append(p.seen, "validation")
	if p.errAt == "validation" {
		return errors.New("validation failed")
	}
	return nil
}

func (p *hookPlugin) ExecutionStart(context.Context, core.Request) error {
	p.seen = append(p.seen, "execution")
	if p.errAt == "execution" {
		return errors.New("execution failed")
	}
	return nil
}

func (p *hookPlugin) ResponseSend(context.Context, core.Response) error {
	p.seen = append(p.seen, "response")
	if p.errAt == "response" {
		return errors.New("response failed")
	}
	return nil
}

func (p *hookPlugin) Error(context.Context, error) {
	p.seen = append(p.seen, "error")
}

type fieldHookPlugin struct {
	plugin.Base
	errField string
}

func (p fieldHookPlugin) FieldResolveStart(ctx context.Context, field plugin.FieldContext) error {
	if field.FieldName == p.errField {
		return errors.New("field hook failed")
	}
	return nil
}

func TestExecutorPluginAndSecurityHelpers(t *testing.T) {
	req := core.Request{Query: `{ ok }`, Variables: map[string]any{"unknown": true}}
	doc := document{Kind: operationQuery, Name: "Q", Variables: []string{"known"}, Selections: []selection{{Name: "a", Alias: "x"}, {Name: "b", Selections: []selection{{Name: "c", Alias: "y"}}}}}
	for _, phase := range []string{"request", "parsing", "validation", "execution"} {
		hook := &hookPlugin{errAt: phase}
		e := &Executor{Plugins: []plugin.Plugin{hook}}
		switch phase {
		case "request":
			_, _ = e.startRequest(context.Background(), req)
		case "parsing":
			_ = e.notifyParsing(context.Background(), req)
		case "validation":
			_ = e.notifyValidation(context.Background(), req)
		case "execution":
			_ = e.notifyExecution(context.Background(), req)
		}
		if len(hook.seen) == 0 {
			t.Fatalf("hook not called for %s", phase)
		}
	}

	e := &Executor{rejectUnknownVars: true}
	if err := e.validateDocumentSecurity(context.Background(), req, doc); err == nil {
		t.Fatal("expected unknown variable error")
	}
	if err := (&Executor{maxSelectionCount: 1}).validateDocumentLimits(doc); err == nil {
		t.Fatal("expected selection limit")
	}
	if err := (&Executor{maxAliasCount: 1}).validateDocumentLimits(doc); err == nil {
		t.Fatal("expected alias limit")
	}
	if err := (&Executor{maxRootFieldCount: 1}).validateDocumentLimits(doc); err == nil {
		t.Fatal("expected root field limit")
	}
	if err := rejectUnknownVariables(core.Request{}, doc); err != nil {
		t.Fatalf("empty variables: %v", err)
	}
	if err := (&Executor{trustedDocuments: map[string]string{"bad": ""}}).validateTrustedDocument(req.Query); err == nil {
		t.Fatal("expected untrusted document")
	}
	sum := sha256.Sum256([]byte(req.Query))
	if err := (&Executor{trustedDocuments: map[string]string{hex.EncodeToString(sum[:]): req.Query}}).validateTrustedDocument(req.Query); err != nil {
		t.Fatalf("trusted document: %v", err)
	}
	responseHook := &hookPlugin{errAt: "response"}
	if res := (&Executor{Plugins: []plugin.Plugin{responseHook}}).sendResponse(context.Background(), core.Response{Data: "ok"}); len(res.Errors) == 0 {
		t.Fatal("expected response hook error")
	}
}

func TestTrustedDocumentsEmptyMapDoesNotRestrictExecution(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	ex := New(s, nil, WithTrustedDocuments(map[string]string{}))
	resp := ex.Execute(context.Background(), core.Request{Query: `{ user(id: "1") { id } }`})
	if len(resp.Errors) != 0 {
		t.Fatalf("empty trusted map should be a no-op: %#v", resp.Errors)
	}
	if resp.Data == nil {
		t.Fatal("expected data")
	}
}

func TestTrustedDocumentsNormalizationSkipsBlankHashes(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	valid := `{ user(id: "1") { id } }`
	sum := sha256.Sum256([]byte(valid))
	digest := strings.ToUpper(hex.EncodeToString(sum[:])) // exercising lower-casing normalization
	ex := New(s, nil, WithTrustedDocuments(map[string]string{" \t ": "junk", digest: valid}))
	resp := ex.Execute(context.Background(), core.Request{Query: valid})
	if len(resp.Errors) != 0 {
		t.Fatalf("trusted document should authorize request: %+v", resp.Errors)
	}
}

func TestTrustedDocumentStoredQueryMismatchFailsExecute(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	canonicalQuery := `{ user(id: "1") { id } }`
	sum := sha256.Sum256([]byte(canonicalQuery))
	digest := hex.EncodeToString(sum[:])
	conflictingStored := `{ user(id: "1") { name } }` // same hash lookup key intentionally wrong body
	ex := New(s, nil, WithTrustedDocuments(map[string]string{digest: conflictingStored}))
	resp := ex.Execute(context.Background(), core.Request{Query: canonicalQuery})
	if len(resp.Errors) == 0 || !strings.Contains(resp.Errors[0].Message, "does not match") {
		t.Fatalf("want hash mismatch trusted-doc error: %#v", resp.Errors)
	}
}

func TestRateLimiterSeesExecuteOperationWithoutBlocking(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var saw core.OperationKind
	ex := New(s, nil, WithRateLimiter(func(_ context.Context, op OperationContext) error {
		saw = op.Kind
		return nil
	}))
	resp := ex.Execute(context.Background(), core.Request{Query: `{ user(id: "1") { id } }`})
	if len(resp.Errors) != 0 {
		t.Fatalf("unexpected execute errors: %#v", resp.Errors)
	}
	if saw != core.OperationQuery {
		t.Fatalf("rate limit hook kind = %q, want query", saw)
	}
	if resp.Data == nil {
		t.Fatal("expected response data")
	}
}

func TestAbstractTypeResolverErrorSurfacesViaInterfaceCompletion(t *testing.T) {
	s := testAdvancedSchema(t)
	ex := New(s, nil, WithAbstractTypeResolver(func(any) (string, error) {
		return "", errors.New("typename hook failed")
	}))
	resp := ex.Execute(context.Background(), core.Request{Query: `{ node { __typename id } }`})
	if len(resp.Errors) == 0 || !strings.Contains(strings.ToLower(resp.Errors[0].Message), "typename") {
		t.Fatalf("expected abstract resolver execution error in response: %#v", resp.Errors)
	}
}

func TestExecutorInvalidEnumSerializationFailsOnOutput(t *testing.T) {
	ex := New(testAdvancedSchema(t), nil)
	resp := ex.Execute(context.Background(), core.Request{
		Query: `{ invalidEpisode }`,
	})
	if len(resp.Errors) == 0 || !strings.Contains(strings.ToLower(resp.Errors[0].Message), "invalid") {
		t.Fatalf("expected enum serialize error in response: %#v", resp.Errors)
	}
}

func TestExecutorNonFiniteFloatSerializesAsError(t *testing.T) {
	ex := New(testAdvancedSchema(t), nil)
	resp := ex.Execute(context.Background(), core.Request{
		Query: `{ nonFiniteFloat }`,
	})
	if len(resp.Errors) == 0 || !strings.Contains(strings.ToLower(resp.Errors[0].Message), "finite") {
		t.Fatalf("expected float non-finite completion error: %#v", resp.Errors)
	}
}

func TestCompletionAndIncrementalWorkBranches(t *testing.T) {
	e := &Executor{}
	stringType := &schema.Scalar{TypeName: "String"}
	intType := &schema.Scalar{TypeName: "Int"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, errs := e.completeValue(ctx, stringType, "x", nil, nil, []any{"field"}); len(errs) == 0 {
		t.Fatal("expected canceled completeValue error")
	}
	if _, errs := e.completeValue(context.Background(), &schema.NonNull{OfType: stringType}, nil, nil, nil, []any{"field"}); len(errs) == 0 {
		t.Fatal("expected non-null nil error")
	}
	if _, errs := e.completeList(context.Background(), intType, "bad", nil, nil, []any{"list"}); len(errs) == 0 {
		t.Fatal("expected non-list error")
	}
	if got, errs := e.completeList(context.Background(), intType, []int{1, 2}, nil, nil, []any{"list"}); len(errs) != 0 || !reflect.DeepEqual(got, []any{1, 2}) {
		t.Fatalf("complete list = %#v %#v", got, errs)
	}
	if _, errs := e.completeValue(context.Background(), stringType, struct{}{}, nil, nil, []any{"bad"}); len(errs) == 0 {
		t.Fatal("expected scalar coercion error")
	}

	collector := &incrementalCollector{}
	collector.addDefer(&schema.Object{TypeName: "Obj", Fields: map[string]*schema.Field{
		"name": {Name: "name", Type: stringType, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return "Ada", nil }},
	}}, nil, []selection{{Name: "name"}}, nil, []any{"user"}, "profile")
	if len(collector.work) != 1 {
		t.Fatalf("collector work = %#v", collector.work)
	}
	payload := e.runIncrementalWork(context.Background(), collector.work[0])
	if payload.Label != "profile" || payload.Data == nil {
		t.Fatalf("defer payload = %#v", payload)
	}

	got, errs := e.completeList(context.Background(), intType, ([]int)(nil), nil, nil, []any{"nums"})
	if len(errs) != 0 {
		t.Fatalf("nil int slice completion errs = %#v", errs)
	}
	listAny, ok := got.([]any)
	if !ok || len(listAny) != 0 {
		t.Fatalf("expected zero-length slice result, got %T %#v", got, got)
	}

	gotNull, errs := e.completeValue(context.Background(), stringType, nil, nil, nil, []any{"nullable"})
	if len(errs) != 0 || gotNull != nil {
		t.Fatalf("nullable nil scalar = %#v errs %#v", gotNull, errs)
	}

	if passthrough, errs := e.completeValue(context.Background(), passthroughSchemaLeaf{}, 42, nil, nil, []any{"leaf"}); len(errs) != 0 || passthrough != 42 {
		t.Fatalf("passthrough scalar completion = %#v errs %#v", passthrough, errs)
	}

	intSliceBacking := []int{9}
	if ptrListG, errs := e.completeList(context.Background(), intType, &intSliceBacking, nil, nil, []any{"ptrList"}); len(errs) != 0 || !reflect.DeepEqual(ptrListG, []any{9}) {
		t.Fatalf("pointer-backed int list = %#v errs %#v", ptrListG, errs)
	}
}

type cacheUser struct {
	ID string `gql:"id,nonNull"`
}

type cacheQuery struct{ calls *int64 }

func (q cacheQuery) User(ctx context.Context, args struct {
	ID string `gql:"id,nonNull"`
}) (*cacheUser, error) {
	atomic.AddInt64(q.calls, 1)
	return &cacheUser{ID: args.ID}, nil
}

func TestResolverCacheMemoizesIdenticalCalls(t *testing.T) {
	var calls int64
	s, err := schema.Build(schema.Config{Query: cacheQuery{calls: &calls}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// Two aliases with identical field+args are not selection-merged, so the
	// resolver would run twice without memoization.
	query := `{ a: user(id: "1") { id } b: user(id: "1") { id } }`

	e := New(s, nil)
	calls = 0
	if resp := e.Execute(context.Background(), core.Request{Query: query}); len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	if calls != 2 {
		t.Fatalf("without cache expected 2 resolver calls, got %d", calls)
	}

	eCached := New(s, nil, WithResolverCache())
	calls = 0
	if resp := eCached.Execute(context.Background(), core.Request{Query: query}); len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	if calls != 1 {
		t.Fatalf("with cache expected 1 resolver call, got %d", calls)
	}
}

type thunkQuery struct{}

func (thunkQuery) Slow(ctx context.Context) (string, error) { return "", nil }

func (thunkQuery) Plain(ctx context.Context) (string, error) { return "", nil }

func TestDeferredResolverThunks(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: thunkQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// Override resolvers to return deferred values. The executor must invoke the
	// thunk and use its result as the field value.
	s.Query.Fields["slow"].Resolver = func(ctx context.Context, p schema.ResolveParams) (any, error) {
		return schema.Thunk(func() (any, error) { return "deferred-value", nil }), nil
	}
	s.Query.Fields["plain"].Resolver = func(ctx context.Context, p schema.ResolveParams) (any, error) {
		return func() (any, error) { return "func-thunk", nil }, nil
	}

	e := New(s, nil)
	resp := e.Execute(context.Background(), core.Request{Query: `{ slow plain }`})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	data := responseObject(t, resp.Data)
	if data["slow"] != "deferred-value" {
		t.Fatalf("slow = %v, want deferred-value", data["slow"])
	}
	if data["plain"] != "func-thunk" {
		t.Fatalf("plain = %v, want func-thunk", data["plain"])
	}
}

type animalQuery struct{}

type Dog struct {
	Name string `gql:"name,nonNull"`
}

type incrementalFriend struct {
	ID string `gql:"id,nonNull"`
}

type incrementalQuery struct{}

func (incrementalQuery) Friends(ctx context.Context) ([]*incrementalFriend, error) {
	return []*incrementalFriend{{ID: "1"}, {ID: "2"}, {ID: "3"}}, nil
}

func (incrementalQuery) User(ctx context.Context) (*incrementalFriend, error) {
	return &incrementalFriend{ID: "root"}, nil
}

func TestIncrementalExecutionDeferAndStream(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: incrementalQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	e := New(s, nil)

	streamReq := core.Request{Query: `{ friends @stream(initialCount: 1, label: "friends") { id } }`}
	if !e.HasIncrementalDirectives(streamReq) {
		t.Fatal("expected stream directive detection")
	}
	initial, payloads := e.ExecuteIncremental(context.Background(), streamReq)
	if len(initial.Errors) != 0 {
		t.Fatalf("initial errors: %#v", initial.Errors)
	}
	if initial.HasNext == nil || !*initial.HasNext {
		t.Fatalf("hasNext = %#v", initial.HasNext)
	}
	if len(payloads) != 2 {
		t.Fatalf("payloads = %#v", payloads)
	}
	if payloads[0].Label != "friends" || len(payloads[0].Items) != 1 {
		t.Fatalf("stream payload = %#v", payloads[0])
	}

	if e.HasIncrementalDirectives(core.Request{Query: `{ friends { id } }`}) {
		t.Fatal("unexpected incremental directive detection")
	}
	if e.HasIncrementalDirectives(core.Request{Query: `{`}) {
		t.Fatal("invalid query should not report incremental directives")
	}

	negReq := core.Request{Query: `{ friends @stream(initialCount: -1, label: "neg") { id } }`}
	initNeg, payNeg := e.ExecuteIncremental(context.Background(), negReq)
	if len(initNeg.Errors) != 0 {
		t.Fatalf("negative initialCount errors: %#v", initNeg.Errors)
	}
	dataNeg := responseObject(t, initNeg.Data)
	arrNeg, ok := dataNeg["friends"].([]any)
	if !ok || len(arrNeg) != 0 {
		t.Fatalf("expected empty initial streamed slice, got %T %#v", dataNeg["friends"], dataNeg["friends"])
	}
	if len(payNeg) != 3 {
		t.Fatalf("expected 3 streamed items after clamp, got %d", len(payNeg))
	}
}

type Cat struct {
	Name string `gql:"name,nonNull"`
}

// Pet is a union (Dog | Cat); abstract-type resolution applies to unions too.

type Pet interface{ isPet() }

func (*Dog) isPet() {}

func (*Cat) isPet() {}

func (animalQuery) Pet(ctx context.Context, args struct {
	Kind string `gql:"kind,nonNull"`
}) (Pet, error) {
	if args.Kind == "cat" {
		return &Cat{Name: "Whiskers"}, nil
	}
	return &Dog{Name: "Rex"}, nil
}

func buildAnimalSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Build(schema.Config{
		Query: animalQuery{},
		Unions: []schema.UnionConfig{{
			Type:         (*Pet)(nil),
			Name:         "Pet",
			Implementors: []any{Dog{}, Cat{}},
		}},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return s
}

func TestAbstractTypeResolverHookIsConsulted(t *testing.T) {
	s := buildAnimalSchema(t)

	var called int
	resolver := func(value any) (string, error) {
		called++
		switch value.(type) {
		case *Cat:
			return "Cat", nil
		default:
			return "Dog", nil
		}
	}
	e := New(s, nil, WithAbstractTypeResolver(resolver))

	resp := e.Execute(context.Background(), core.Request{
		Query: `{ pet(kind: "cat") { __typename ... on Cat { name } } }`,
	})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	if called == 0 {
		t.Fatal("expected abstract type resolver to be consulted")
	}
	pet := responseObject(t, responseObject(t, resp.Data)["pet"])
	if pet["__typename"] != "Cat" {
		t.Fatalf("__typename = %v, want Cat", pet["__typename"])
	}
	if pet["name"] != "Whiskers" {
		t.Fatalf("name = %v, want Whiskers", pet["name"])
	}
}

func TestAbstractTypeResolverEmptyFallsBack(t *testing.T) {
	s := buildAnimalSchema(t)
	// Returning "" must fall back to default reflection-based resolution.
	e := New(s, nil, WithAbstractTypeResolver(func(any) (string, error) { return "", nil }))
	resp := e.Execute(context.Background(), core.Request{
		Query: `{ pet(kind: "dog") { __typename } }`,
	})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	pet := responseObject(t, responseObject(t, resp.Data)["pet"])
	if pet["__typename"] != "Dog" {
		t.Fatalf("__typename = %v, want Dog (fallback)", pet["__typename"])
	}
}

func TestAbstractTypeResolverRejectingNonObjectTypenameFails(t *testing.T) {
	s := buildAnimalSchema(t)
	e := New(s, nil, WithAbstractTypeResolver(func(any) (string, error) {
		return "String", nil
	}))
	resp := e.Execute(context.Background(), core.Request{
		Query: `{ pet(kind:"dog") { __typename ... on Dog { name } } }`,
	})
	if len(resp.Errors) == 0 {
		t.Fatalf("expected resolver error")
	}
	msg := strings.ToLower(resp.Errors[0].Message)
	if !strings.Contains(msg, "non-object") || !strings.Contains(msg, "string") {
		t.Fatalf("expected non-object type error for builtin scalar name, got %q", resp.Errors[0].Message)
	}
}

type orderMutation struct{}

func (orderMutation) First(ctx context.Context) (string, error) {
	orderMutationLog = append(orderMutationLog, "first")
	return "first", nil
}

func (orderMutation) Second(ctx context.Context) (string, error) {
	orderMutationLog = append(orderMutationLog, "second")
	return "second", nil
}

var orderMutationLog []string

func TestMutationRootFieldsExecuteSerially(t *testing.T) {
	orderMutationLog = nil

	schemaValue, err := schema.Build(schema.Config{
		Query:    introQuery{},
		Mutation: orderMutation{},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `mutation {
			second
			first
		}`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}
	if len(orderMutationLog) != 2 || orderMutationLog[0] != "second" || orderMutationLog[1] != "first" {
		t.Fatalf("expected serial mutation order [second first], got %#v", orderMutationLog)
	}
}

type sequentialQuerySleep struct{}

var (
	sleepPeakConcurrent int32
	sleepCurrent        int32
)

func (sequentialQuerySleep) A(ctx context.Context) (string, error) {
	n := atomic.AddInt32(&sleepCurrent, 1)
	for {
		old := atomic.LoadInt32(&sleepPeakConcurrent)
		if n <= old || atomic.CompareAndSwapInt32(&sleepPeakConcurrent, old, n) {
			break
		}
	}
	time.Sleep(40 * time.Millisecond)
	atomic.AddInt32(&sleepCurrent, -1)
	return "a", nil
}

func (sequentialQuerySleep) B(ctx context.Context) (string, error) {
	n := atomic.AddInt32(&sleepCurrent, 1)
	for {
		old := atomic.LoadInt32(&sleepPeakConcurrent)
		if n <= old || atomic.CompareAndSwapInt32(&sleepPeakConcurrent, old, n) {
			break
		}
	}
	time.Sleep(40 * time.Millisecond)
	atomic.AddInt32(&sleepCurrent, -1)
	return "b", nil
}

// Production executor runs sibling fields sequentially (deterministic resolver
// order; no speculative goroutine parallelism at the root selection set).

func TestQuerySiblingFieldsExecuteSerially(t *testing.T) {
	atomic.StoreInt32(&sleepPeakConcurrent, 0)
	atomic.StoreInt32(&sleepCurrent, 0)

	schemaValue, err := schema.Build(schema.Config{Query: sequentialQuerySleep{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	start := time.Now()
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ a b }`,
	})
	elapsed := time.Since(start)

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}
	if atomic.LoadInt32(&sleepPeakConcurrent) != 1 {
		t.Fatalf("expected serial sibling execution (peak concurrency got %d, want 1)", sleepPeakConcurrent)
	}
	if elapsed < 70*time.Millisecond {
		t.Fatalf("expected ~two consecutive sleeps (>70ms serial), took %v", elapsed)
	}
}

type BubbleItem struct{}

type bubbleQuery struct{}

func (bubbleQuery) RequiredItem(ctx context.Context) (*BubbleItem, error) {
	return &BubbleItem{}, nil
}

func TestNonNullFieldErrorBubblesToParent(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: bubbleQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	itemType := schemaValue.Types["BubbleItem"]
	object, ok := itemType.(*schema.Object)
	if !ok {
		t.Fatalf("expected BubbleItem object, got %T", itemType)
	}
	object.Fields["required"] = &schema.Field{
		Name: "required",
		Type: &schema.NonNull{OfType: schemaValue.Types["String"]},
		Resolver: func(ctx context.Context, params schema.ResolveParams) (any, error) {
			return nil, errors.New("resolver failed")
		},
	}

	// Wrap the item object as a non-null return type on the root field.
	schemaValue.Query.Fields["requiredItem"].Type = &schema.NonNull{OfType: itemType}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ requiredItem { required } }`,
	})

	if len(response.Errors) == 0 {
		t.Fatal("expected field error")
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if !strings.Contains(string(raw), `"data":null`) {
		t.Fatalf("expected top-level data:null when non-null field bubbles, got %s", raw)
	}
}

type testSubscription struct {
	source <-chan *testUser
}

func (s testSubscription) UserCreated(ctx context.Context) (<-chan *testUser, error) {
	return s.source, nil
}

func newTestSubscriptionExecutor(t *testing.T, source <-chan *testUser) *Executor {
	t.Helper()

	schemaValue, err := schema.Build(schema.Config{
		Query:        testQuery{},
		Subscription: testSubscription{source: source},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return New(schemaValue, nil)
}

func TestExecutorSubscribeStreamsResponses(t *testing.T) {
	source := make(chan *testUser, 2)
	source <- &testUser{ID: "1", Name: "Ada"}
	source <- &testUser{ID: "2", Name: "Grace"}
	close(source)

	executor := newTestSubscriptionExecutor(t, source)

	stream, err := executor.Subscribe(context.Background(), core.Request{
		Query: `subscription { userCreated { id name } }`,
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	expectedNames := []string{"Ada", "Grace"}
	for index, expected := range expectedNames {
		select {
		case res, ok := <-stream:
			if !ok {
				t.Fatalf("expected %d responses, channel closed early", len(expectedNames))
			}
			if len(res.Errors) != 0 {
				t.Fatalf("unexpected errors at index %d: %#v", index, res.Errors)
			}
			payload := responseObject(t, res.Data)
			user := responseObject(t, payload["userCreated"])
			if user["name"] != expected {
				t.Fatalf("expected %s, got %#v", expected, user["name"])
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for response %d", index)
		}
	}

	select {
	case _, open := <-stream:
		if open {
			t.Fatalf("expected stream to close after source closed")
		}
	case <-time.After(time.Second):
		t.Fatalf("expected stream to close after source closed")
	}
}

func TestExecutorSubscribeRejectsNonSubscriptionOperations(t *testing.T) {
	executor := newTestSubscriptionExecutor(t, nil)

	if _, err := executor.Subscribe(context.Background(), core.Request{Query: `{ user(id: "1") { id } }`}); err == nil {
		t.Fatalf("expected error for query operation")
	}
}

func TestExecutorSubscribeRequiresSingleRootField(t *testing.T) {
	executor := newTestSubscriptionExecutor(t, nil)

	_, err := executor.Subscribe(context.Background(), core.Request{
		Query: `subscription { userCreated { id } anotherField }`,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must select only one top level field") {
		t.Fatalf("expected single-root-field error, got %v", err)
	}
}

func TestExecuteRejectsSubscriptionOperation(t *testing.T) {
	executor := newTestSubscriptionExecutor(t, nil)

	response := executor.Execute(context.Background(), core.Request{
		Query: `subscription { userCreated { id } }`,
	})
	if len(response.Errors) == 0 {
		t.Fatalf("expected error, got none")
	}
	if !strings.Contains(response.Errors[0].Message, "subscription operations") {
		t.Fatalf("expected subscription rejection, got %q", response.Errors[0].Message)
	}
}

func TestExecutorSubscribeWithVariablesParsesBypassingDocumentCache(t *testing.T) {
	source := make(chan *testUser, 2)
	source <- &testUser{ID: "1", Name: "Ada"}
	close(source)

	executor := newTestSubscriptionExecutor(t, source)

	stream, err := executor.Subscribe(context.Background(), core.Request{
		Query: `subscription Sub($skip: Boolean!) {
			userCreated @skip(if: $skip) { id name }
		}`,
		Variables: map[string]any{"skip": false},
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	select {
	case res, ok := <-stream:
		if !ok {
			t.Fatal("expected one response")
		}
		if len(res.Errors) != 0 {
			t.Fatalf("unexpected errors: %#v", res.Errors)
		}
		payload := responseObject(t, res.Data)
		user := responseObject(t, payload["userCreated"])
		if user["name"] != "Ada" {
			t.Fatalf("want Ada, got %#v", user["name"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	select {
	case _, ok := <-stream:
		if ok {
			t.Fatal("expected stream closed after exhausted source")
		}
	case <-time.After(time.Second):
		t.Fatal("expected closed stream")
	}
}

func TestExecutorSubscribeStopsOnContextCancel(t *testing.T) {
	source := make(chan *testUser)
	executor := newTestSubscriptionExecutor(t, source)

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := executor.Subscribe(ctx, core.Request{
		Query: `subscription { userCreated { id name } }`,
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	cancel()

	select {
	case _, open := <-stream:
		if open {
			t.Fatalf("expected stream to close on cancel")
		}
	case <-time.After(time.Second):
		t.Fatalf("expected stream to close on cancel")
	}
}

type badSubscription struct{}

func (badSubscription) UserCreated(ctx context.Context) (string, error) {
	return "not-a-channel", nil
}

type failingSubscription struct{}

func (failingSubscription) UserCreated(ctx context.Context) (<-chan *testUser, error) {
	return nil, errors.New("source failed")
}

type failSubscribeResponseSendPlugin struct{ plugin.Base }

func (failSubscribeResponseSendPlugin) ResponseSend(context.Context, core.Response) error {
	return fmt.Errorf("response halted by plugin")
}

func TestExecutorSubscribeRejectsInvalidSources(t *testing.T) {
	for _, tc := range []struct {
		name         string
		subscription any
	}{
		{name: "non channel", subscription: badSubscription{}},
		{name: "resolver error", subscription: failingSubscription{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schemaValue, err := schema.Build(schema.Config{
				Query:        testQuery{},
				Subscription: tc.subscription,
			})
			if err != nil {
				t.Fatalf("build schema: %v", err)
			}
			executor := New(schemaValue, nil)
			if _, err := executor.Subscribe(context.Background(), core.Request{Query: `subscription { userCreated { id } }`}); err == nil {
				t.Fatal("expected subscribe error")
			}
		})
	}
}

type failSubscribeParsingPlugin struct{ plugin.Base }

func (failSubscribeParsingPlugin) ParsingStart(context.Context, core.Request) error {
	return fmt.Errorf("parsing blocked")
}

type failSubscribeValidationPlugin struct{ plugin.Base }

func (failSubscribeValidationPlugin) ValidationStart(context.Context, core.Request) error {
	return fmt.Errorf("validation blocked")
}

type failSubscribeExecutionPlugin struct{ plugin.Base }

func (failSubscribeExecutionPlugin) ExecutionStart(context.Context, core.Request) error {
	return fmt.Errorf("execution blocked")
}

type panicSubscribeRoot struct{}

func (panicSubscribeRoot) UserCreated(context.Context) (<-chan *testUser, error) {
	panic("subscription resolver panic")
}

func TestExecutorSubscribePropagatesParsingHookFailure(t *testing.T) {
	source := make(chan *testUser)
	base := newTestSubscriptionExecutor(t, source)
	ex := New(base.Schema, []plugin.Plugin{failSubscribeParsingPlugin{}})
	defer close(source)

	if _, err := ex.Subscribe(context.Background(), core.Request{Query: `subscription { userCreated { id } }`}); err == nil || !strings.Contains(err.Error(), "parsing blocked") {
		t.Fatalf("expected parsing hook error, got %v", err)
	}
}

func TestExecutorSubscribePropagatesValidationHookFailure(t *testing.T) {
	source := make(chan *testUser)
	base := newTestSubscriptionExecutor(t, source)
	ex := New(base.Schema, []plugin.Plugin{failSubscribeValidationPlugin{}})
	defer close(source)

	if _, err := ex.Subscribe(context.Background(), core.Request{Query: `subscription { userCreated { id } }`}); err == nil || !strings.Contains(err.Error(), "validation blocked") {
		t.Fatalf("expected validation hook error, got %v", err)
	}
}

func TestExecutorSubscribePropagatesExecutionHookFailure(t *testing.T) {
	source := make(chan *testUser)
	base := newTestSubscriptionExecutor(t, source)
	ex := New(base.Schema, []plugin.Plugin{failSubscribeExecutionPlugin{}})
	defer close(source)

	if _, err := ex.Subscribe(context.Background(), core.Request{Query: `subscription { userCreated { id } }`}); err == nil || !strings.Contains(err.Error(), "execution blocked") {
		t.Fatalf("expected execution hook error, got %v", err)
	}
}

func TestExecutorSubscribeRecoversPanicInSourceResolver(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{
		Query:        testQuery{},
		Subscription: panicSubscribeRoot{},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	ex := New(schemaValue, nil)

	if _, err := ex.Subscribe(context.Background(), core.Request{Query: `subscription { userCreated { id } }`}); err == nil || !strings.Contains(err.Error(), "panic") {
		t.Fatalf("expected masked panic subscription error, got %v", err)
	}
}

func TestExecutorSubscribePropagatesResponseSendHookFailure(t *testing.T) {
	source := make(chan *testUser, 1)
	source <- &testUser{ID: "1", Name: "Ada"}
	close(source)

	base := newTestSubscriptionExecutor(t, source)
	executor := New(base.Schema, []plugin.Plugin{failSubscribeResponseSendPlugin{}})
	stream, err := executor.Subscribe(context.Background(), core.Request{
		Query: `subscription { userCreated { id name } }`,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	res := <-stream
	if len(res.Errors) == 0 {
		t.Fatalf("expected response-send hook failure, got %#v", res)
	}
}

func TestExecutorSubscribeSecurityAndRootErrors(t *testing.T) {
	executor := newTestSubscriptionExecutor(t, nil)
	if _, err := executor.Subscribe(context.Background(), core.Request{Query: `subscription { missing }`}); err == nil {
		t.Fatal("expected unknown subscription field error")
	}

	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	noRoot := New(schemaValue, nil)
	if _, err := noRoot.Subscribe(context.Background(), core.Request{Query: `subscription { userCreated { id } }`}); err == nil {
		t.Fatal("expected missing subscription root error")
	}
}
