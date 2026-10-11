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

func TestIncrementalResponseDirectCodec(t *testing.T) {
	hasMore := true
	for _, hasNext := range []*bool{nil, new(bool), &hasMore} {
		patches := []IncrementalPayload{
			{Label: "deferred", Path: []any{"user"}, Data: map[string]any{"id": 1}},
			{ItemsNull: true, Errors: []Error{{Message: "null item"}}, Extensions: map[string]any{"code": "NULL"}},
			{Items: []any{nil, []any(nil), (*nilIncrementalMarshaler)(nil)}},
		}
		response := Response{Incremental: patches, HasNext: hasNext}
		encoded, err := response.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		// The ordinary struct encoder is the reference for field presence and order.
		reference, err := json.Marshal(struct {
			Incremental []IncrementalPayload `json:"incremental,omitempty"`
			HasNext     *bool                `json:"hasNext,omitempty"`
		}{patches, hasNext})
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != string(reference) {
			t.Fatalf("got %s want %s", encoded, reference)
		}
		_, err = (Response{Incremental: []IncrementalPayload{{Label: "overwrite pooled buffer", Items: []any{1}}}}).MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != string(reference) {
			t.Fatal("returned JSON aliases pooled memory")
		}
	}
	for _, patch := range []IncrementalPayload{{Items: []any{math.NaN()}}, {Extensions: map[string]any{"bad": func() {}}}} {
		if _, err := (Response{Incremental: []IncrementalPayload{patch}}).MarshalJSON(); err == nil {
			t.Fatal("unsupported patch accepted")
		}
	}
}

func TestIncrementalResponsePreservesEnvelopeFields(t *testing.T) {
	ordered := NewOrderedObject(1)
	ordered.Set("id", "1")
	for _, response := range []Response{
		{Data: ordered}, {Data: (*OrderedObject)(nil)}, {DataNull: true},
		{Data: "invalid UTF8: \xff <>& \u2028"}, {Data: []any(nil)},
		{Data: map[string]any{"ok": true}, Errors: []Error{{Message: "partial"}}, Extensions: map[string]any{"tracing": true}},
	} {
		response.Incremental = []IncrementalPayload{{ItemsNull: true}}
		type wireResponse struct {
			Data        any                  `json:"data,omitempty"`
			Errors      []Error              `json:"errors,omitempty"`
			Incremental []IncrementalPayload `json:"incremental"`
			Extensions  map[string]any       `json:"extensions,omitempty"`
		}
		data := response.Data
		if response.DataNull {
			data = json.RawMessage("null")
		}
		want, err := json.Marshal(wireResponse{data, response.Errors, response.Incremental, response.Extensions})
		if err != nil {
			t.Fatal(err)
		}
		got, err := response.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("got %s want %s", got, want)
		}
	}
	for _, response := range []Response{
		{Data: math.NaN()}, {Errors: []Error{{Extensions: map[string]any{"bad": func() {}}}}},
		{Extensions: map[string]any{"bad": func() {}}},
	} {
		response.Incremental = []IncrementalPayload{{ItemsNull: true}}
		if _, err := response.MarshalJSON(); err == nil {
			t.Fatal("unsupported envelope accepted")
		}
	}
}

func TestIncrementalDirectCodecCustomValues(t *testing.T) {
	object := NewOrderedObject(3)
	object.Set("separators", "\u2028\u2029")
	object.Set("custom", json.RawMessage(` {"html":"<>&","space":"\u2028"} `))
	object.Set("nil", (*nilIncrementalMarshaler)(nil))
	response := Response{Incremental: []IncrementalPayload{{Label: "\u2028\u2029", Items: []any{object}}}}
	got, err := response.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("direct=%s standard=%s", got, want)
	}
	object.Set("custom", json.RawMessage(`invalid`))
	if _, err := response.MarshalJSON(); err == nil {
		t.Fatal("invalid custom JSON accepted")
	}
}
