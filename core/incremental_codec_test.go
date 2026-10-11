package core

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestIncrementalPayloadNullItemsRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload IncrementalPayload
		items   string
	}{
		{"failed stream", IncrementalPayload{ItemsNull: true, Path: []any{"list", 1}, Label: "batch", Errors: []Error{{Message: "null item"}}}, "null"},
		{"nullable item", IncrementalPayload{Items: []any{nil}}, "[null]"},
		{"null without errors", IncrementalPayload{ItemsNull: true}, "null"},
		{"empty", IncrementalPayload{}, ""},
		{"typed nil data", IncrementalPayload{Data: []any(nil)}, ""},
		{"typed nil marshaler", IncrementalPayload{Data: (*nilIncrementalMarshaler)(nil)}, ""},
		{"finite float", IncrementalPayload{Items: []any{1.25}}, "[1.25]"},
		{"nullable error", IncrementalPayload{Items: []any{nil}, Errors: []Error{{Message: "failed"}}}, "[null]"},
		{"items", IncrementalPayload{Items: []any{"ok"}}, `["ok"]`},
		{"defer", IncrementalPayload{Data: map[string]any{"ok": true}}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatal(err)
			}
			if (tc.name == "typed nil data" || tc.name == "typed nil marshaler") && string(wire["data"]) != "null" {
				t.Fatalf("wire=%s", raw)
			}
			if string(wire["items"]) != tc.items {
				t.Fatalf("wire=%s", raw)
			}
			var decoded IncrementalPayload
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.ItemsNull != tc.payload.ItemsNull || !reflect.DeepEqual(decoded.Items, tc.payload.Items) || decoded.Label != tc.payload.Label || !reflect.DeepEqual(decoded.Errors, tc.payload.Errors) {
				t.Fatalf("decoded=%#v", decoded)
			}
		})
	}
}

func TestIncrementalPayloadCodecErrors(t *testing.T) {
	for _, raw := range []string{`{`, `{"items":42}`} {
		var value IncrementalPayload
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, value := range []IncrementalPayload{{Data: math.NaN()}, {Items: []any{[]any{math.Inf(1)}}}, {Data: func() {}}, {Items: []any{func() {}}}, {Path: []any{func() {}}}, {ItemsNull: true, Extensions: map[string]any{"invalid": func() {}}}} {
		if _, err := json.Marshal(value); err == nil {
			t.Fatal("unsupported value accepted")
		}
	}
}

func BenchmarkIncrementalPayloadJSON(b *testing.B) {
	item := NewOrderedObject(1)
	item.Set("id", "1")
	payload := IncrementalPayload{Items: []any{item}, Path: []any{"friends", 1}, Label: "friends"}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := json.Marshal(payload); err != nil {
			b.Fatal(err)
		}
	}
}

type nilIncrementalMarshaler struct{}

func (*nilIncrementalMarshaler) MarshalJSON() ([]byte, error) {
	panic("nil marshaler must not be called")
}
