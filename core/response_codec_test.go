package core

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestResponseJSONBranches(t *testing.T) {
	obj := NewOrderedObject(2)
	obj.Set("name", "Ada")
	obj.Set("score", 1.5)

	raw, err := json.Marshal(Response{Data: obj})
	if err != nil {
		t.Fatalf("marshal ordered: %v", err)
	}
	if string(raw) != `{"data":{"name":"Ada","score":1.5}}` {
		t.Fatalf("ordered json = %s", raw)
	}

	raw, err = json.Marshal(Response{Data: map[string]any{"ok": true}})
	if err != nil {
		t.Fatalf("marshal map: %v", err)
	}
	if !strings.Contains(string(raw), `"data":{"ok":true}`) {
		t.Fatalf("map json = %s", raw)
	}

	empty := NewOrderedObject(0)
	raw, err = empty.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal empty ordered: %v", err)
	}
	if string(raw) != `{}` {
		t.Fatalf("empty ordered = %s", raw)
	}

	nested := NewOrderedObject(2)
	nested.Set("nil", nil)
	nested.Set("bool", true)
	nested.Set("string", "x")
	nested.Set("ints", []any{int8(1), int16(2), int32(3), int64(4), uint(5), uint8(6), uint16(7), uint32(8), uint64(9)})
	nested.Set("floats", []any{float32(1.25), 2.5})
	nested.Set("badFloat", []any{math.Inf(1)})
	nested.Set("objects", []*OrderedObject{obj, nil})
	nested.Set("custom", json.RawMessage(`{"raw":true}`))
	raw, err = nested.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal nested ordered: %v", err)
	}
	if !strings.Contains(string(raw), `"objects":[{"name":"Ada","score":1.5},null]`) {
		t.Fatalf("nested ordered = %s", raw)
	}

	errObj := NewOrderedObject(1)
	errObj.Set("bad", failingMarshaler{})
	if _, err := errObj.MarshalJSON(); err == nil {
		t.Fatal("expected ordered object marshal error")
	}

	hasNext := false
	encoded := `{"data":{"ok":true},"errors":[{"message":"x"}],"incremental":[{"label":"l"}],"hasNext":false,"extensions":{"id":"1"}}`
	var decoded Response
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatalf("unmarshal full response: %v", err)
	}
	if decoded.HasNext == nil || *decoded.HasNext != hasNext || len(decoded.Incremental) != 1 || decoded.Extensions["id"] != "1" {
		t.Fatalf("decoded response mismatch: %#v", decoded)
	}

	for _, bad := range []string{
		`{"data":}`,
		`{"data":true,"errors":{}}`,
		`{"data":true,"incremental":{}}`,
		`{"data":true,"hasNext":"x"}`,
		`{"data":true,"extensions":[]}`,
	} {
		var out Response
		if err := json.Unmarshal([]byte(bad), &out); err == nil {
			t.Fatalf("expected unmarshal error for %s", bad)
		}
	}
}

type failingMarshaler struct{}

func (failingMarshaler) MarshalJSON() ([]byte, error) {
	return nil, errors.New("marshal failed")
}
