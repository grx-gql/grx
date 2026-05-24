---
title: File uploads
description: multipart/form-data uploads per the GraphQL multipart request spec  -  Upload scalar, map field, curl testing.
outline: deep
---

# File uploads

grx accepts file uploads via the [**GraphQL multipart request specification**](https://github.com/jaydenseric/graphql-multipart-request-spec):

- **`Content-Type: multipart/form-data`** on `POST`.
- Form fields **`operations`** (JSON: query + variables with null placeholders where files attach) and **`map`** (JSON: path → multipart part keys).
- One file field per **`map`** entry; the **`http`** transport injects **`[core.Upload](https://pkg.go.dev/github.com/grx-gql/grx/core#Upload)`** into **`variables`** before execution.

Scalars are explicit: declare an **`Upload`** scalar on **`schema.Config.Scalars`** (see runnable **`examples/file-upload/`**).

## Register the **`Upload`** scalar

```go
package graph

import (
	"fmt"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

// Omit Query/Mutation declarations here  -  declare them beside your resolver structs.
//
//	appgraph.Config{
//
//	Query: Query{}, Mutation: Mutation{},
//	Scalars: []schema.ScalarConfig{ ...
var UploadScalarSnippet = schema.ScalarConfig{
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
		u, ok := value.(core.Upload)
		if !ok {
			return nil, fmt.Errorf("expected Upload, got %T", value)
		}
		return u.Filename, nil
	},
}
```

Resolver arguments still use structs with **`core.Upload`** (or pointer) **`gql` tags**:

```go
package graph

import (
	"context"

	"github.com/grx-gql/grx/core"
)

type UploadArgs struct {
	File core.Upload `gql:"file,nonNull"`
}

func (Mutation) UploadFile(ctx context.Context, args UploadArgs) (*Payload, error) {
	f, err := args.File.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// stream or copy  -  args.File holds Filename, MIME, etc.

	return nil, nil // return your payload
}
```

## Curl shape

Mirror the multipart contract (see **`examples/file-upload/graph/schema.go`** comments):

```bash
curl http://localhost:4005/graphql \
  -F 'operations={"query":"mutation($f: Upload!){ uploadFile(file: $f){ filename } }","variables":{"f":null}}' \
  -F 'map={"0":["variables.f"]}' \
  -F 0=@hello.txt
```

Lists of uploads use **`[Upload!]`** and matching **`variables`** placeholders with **`null` entries`; **`map`** keys point each file path.

## Operational notes

- Size limits belong at **reverse proxy**, **`MaxHTTPRequestBytes`**, **or middleware** scanning **`Content-Length`** before parsing - multipart bodies can dominate memory if unbounded (**[Limits](/guides/request-limits)**).
- Do **stream** **`Upload.Open()`** outputs to storage rather than blindly **`ReadAll`** for large uploads.

---

## See also

- **`examples/file-upload/`** (`go run ./examples/file-upload`)
- **`[core.Upload]`** (`core/upload.go`) and **`decode.go`** multipart wiring in **`http`**
