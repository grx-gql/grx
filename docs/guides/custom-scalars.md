---
title: Custom scalars
description: Register Parse + Serialize closures on schema.Config.Scalars - DateTime Upload JSON values example patterns.
outline: deep
---

# Custom scalars

Anything beyond **`String`**, **`Int`**, **`Float`**, **`Boolean`**, **`ID`** is a **custom scalar**: you choose the Go backing type (`struct`, `string`, `time.Time` wrapper…) and expose **`schema.ScalarConfig{ Name, Parse, Serialize, SpecifiedByURL }`**.

Parsing happens when variables or literal arguments coerce into resolver structs; serialization runs when emitting response JSON.

Runnable references:

| Example | Highlights |
| --- | --- |
| **`examples/scalars`** | **`DateTime` scalar**, **`SpecifiedByURL`**, structs returned from resolving root fields (`NextEvent`). |
| **`examples/file-upload`** | Built-in multipart **`Upload`** scalar wiring (**[File uploads](/guides/file-upload)**). |

Core pattern (trimmed):

```go
package graph

import (
	"time"

	"github.com/grx-gql/grx/schema"
)

// DateTime is a tiny example carrier  -  see examples/scalars for a full implementation.
type DateTime struct{ Time time.Time }

var _ = []schema.ScalarConfig{{
	Type:           DateTime{},
	Name:           "DateTime",
	SpecifiedByURL: "https://www.rfc-editor.org/rfc/rfc3339",
	Parse:          func(input any) (any, error) { /* coerce GraphQL literals / variables */ return nil, nil },
	Serialize:      func(value any) (any, error) { /* coerce Go → JSON wire shape */ return nil, nil },
}}
```

Expose scalars sparingly - they become part of client contracts and tooling must understand coercions. Pair with schema validation tests when changing Parse behaviour.

::: tip Enumerations  
**`examples/enums`** shows tagging strings as enums and binding Go constants - different mechanism than **`Scalars`**, but complementary when modelling closed sets.

:::

## Related

- [`schema.Config.Scalars`](https://pkg.go.dev/github.com/grx-gql/grx/schema#Config)
- **[Define your schema](/concepts/schema-basics)** (`gql` tags + structs)
