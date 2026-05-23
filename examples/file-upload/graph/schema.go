// Package graph defines a schema that accepts file uploads via the GraphQL
// multipart request specification
// (https://github.com/jaydenseric/graphql-multipart-request-spec).
//
// The custom Upload scalar is backed by core.Upload. The default HTTP transport
// decodes a multipart/form-data request, substitutes the uploaded file for the
// matching variable, and the executor binds it to the resolver argument.
package graph

import (
	"context"
	"fmt"
	"io"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

type Mutation struct{}

type Query struct{}

// UploadResult summarizes a received file.
type UploadResult struct {
	Filename string `gql:"filename,nonNull"`
	Size     int    `gql:"size,nonNull"`
	Contents string `gql:"contents,nonNull"`
}

// NewSchema registers the Upload scalar and the uploadFile mutation.
func NewSchema() schema.Config {
	return schema.Config{
		Query:    Query{},
		Mutation: Mutation{},
		Scalars: []schema.ScalarConfig{
			{
				Type: core.Upload{},
				Name: "Upload",
				Parse: func(input any) (any, error) {
					switch u := input.(type) {
					case *core.Upload:
						return *u, nil
					case core.Upload:
						return u, nil
					default:
						return nil, fmt.Errorf("Upload must be a multipart file, got %T", input)
					}
				},
				Serialize: func(value any) (any, error) {
					if u, ok := value.(core.Upload); ok {
						return u.Filename, nil
					}
					return nil, fmt.Errorf("expected Upload, got %T", value)
				},
			},
		},
	}
}

// Ping keeps the query root non-empty.
func (Query) Ping(ctx context.Context) (string, error) { return "ok", nil }

// UploadFile reads the uploaded file and returns a summary. Call it with a
// multipart request:
//
//	curl http://localhost:4005/graphql \
//	  -F operations='{"query":"mutation($f: Upload!){ uploadFile(file: $f){ filename size contents } }","variables":{"f":null}}' \
//	  -F map='{"0":["variables.f"]}' \
//	  -F 0=@hello.txt
type UploadArgs struct {
	File core.Upload `gql:"file,nonNull"`
}

func (Mutation) UploadFile(ctx context.Context, args UploadArgs) (*UploadResult, error) {
	f, err := args.File.Open()
	if err != nil {
		return nil, fmt.Errorf("open upload: %w", err)
	}
	defer f.Close()

	contents, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}

	return &UploadResult{
		Filename: args.File.Filename,
		Size:     len(contents),
		Contents: string(contents),
	}, nil
}
