package exec_test

import (
	"context"
	"fmt"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/loader"
	"github.com/grx-gql/grx/schema"
)

type loaderExampleUser struct {
	ID   string
	Name string
}
type loaderExampleQuery struct{}

func (loaderExampleQuery) Users(context.Context) ([]*loaderExampleUser, error) {
	return []*loaderExampleUser{{ID: "2"}, {ID: "1"}, {ID: "2"}}, nil
}

func ExampleWithLoaders() {
	names, err := loader.New("user names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
		// Replace this map with one database query for the supplied keys.
		users := map[loader.Key]string{"1": "Ada", "2": "Grace"}
		results := make([]loader.Result, len(keys))
		for i, key := range keys {
			results[i].Value = users[key]
		}
		fmt.Println("batch:", keys)
		return results, nil
	}, 100)
	if err != nil {
		panic(err)
	}
	compiled, err := schema.Build(schema.Config{Query: loaderExampleQuery{}})
	if err != nil {
		panic(err)
	}
	// Keep the declared GraphQL return type and replace its runtime resolver.
	user := compiled.Types["loaderExampleUser"].(*schema.Object)
	user.Fields["name"].Resolver = func(ctx context.Context, params schema.ResolveParams) (any, error) {
		return loader.Load(ctx, names, loader.Key(params.Source.(*loaderExampleUser).ID))
	}
	e := exec.New(compiled, nil, exec.WithLoaders([]*loader.Loader{names}))
	response := e.Execute(context.Background(), core.Request{Query: `{ users { name } }`})
	if len(response.Errors) != 0 {
		panic(response.Errors)
	}
	encoded, err := response.Data.(*core.OrderedObject).MarshalJSON()
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
	// Output:
	// batch: [2 1]
	// {"users":[{"name":"Grace"},{"name":"Ada"},{"name":"Grace"}]}
}
