module github.com/patrickkabwe/grx/benchmark

go 1.22

// The benchmark module is intentionally separate from the root grx module so
// the comparison libraries below do not become runtime dependencies of grx
// itself. The replace directive points at the working tree.
replace github.com/patrickkabwe/grx => ../

require (
	github.com/graph-gophers/graphql-go v1.5.0
	github.com/graphql-go/graphql v0.8.1
	github.com/patrickkabwe/grx v0.0.0-00010101000000-000000000000
)
