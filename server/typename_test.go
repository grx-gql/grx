package server

import (
	"testing"

	grxclient "github.com/patrickkabwe/grx/pkg/client"
)

func TestServeHTTPExecutesNamedTypenameOnlyQuery(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "query MyQuery {\n  __typename\n}",
	}))
	assertNoErrors(t, body)

	data := nestedMap(t, body, "data")
	assertExactKeys(t, data, "__typename")
	if data["__typename"] != "Query" {
		t.Fatalf("expected root typename Query, got %#v", data["__typename"])
	}
}
