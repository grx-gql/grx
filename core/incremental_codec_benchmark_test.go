package core

import (
	"encoding/json"
	"testing"
)

type legacyIncrementalPayload struct {
	Label      string         `json:"label,omitempty"`
	Path       []any          `json:"path,omitempty"`
	Data       any            `json:"data,omitempty"`
	Items      []any          `json:"items,omitempty"`
	Errors     []Error        `json:"errors,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}
type legacyIncrementalResponse struct {
	Data        any                        `json:"data,omitempty"`
	DataNull    bool                       `json:"-"`
	Errors      []Error                    `json:"errors,omitempty"`
	Extensions  map[string]any             `json:"extensions,omitempty"`
	Incremental []legacyIncrementalPayload `json:"incremental,omitempty"`
	HasNext     *bool                      `json:"hasNext,omitempty"`
}

func (r legacyIncrementalResponse) MarshalJSON() ([]byte, error) {
	aux := struct {
		Data        json.RawMessage            `json:"data,omitempty"`
		Errors      []Error                    `json:"errors,omitempty"`
		Incremental []legacyIncrementalPayload `json:"incremental,omitempty"`
		HasNext     *bool                      `json:"hasNext,omitempty"`
		Extensions  map[string]any             `json:"extensions,omitempty"`
	}{Errors: r.Errors, Incremental: r.Incremental, HasNext: r.HasNext, Extensions: r.Extensions}
	if r.Data != nil {
		data, err := json.Marshal(r.Data)
		if err != nil {
			return nil, err
		}
		aux.Data = data
	} else if r.DataNull {
		aux.Data = json.RawMessage("null")
	}
	return json.Marshal(&aux)
}

func BenchmarkIncrementalCodecComparison(b *testing.B) {
	item := NewOrderedObject(1)
	item.Set("id", "1")
	legacy := legacyIncrementalPayload{Label: "friends", Path: []any{"friends", 1}, Items: []any{item}}
	current := IncrementalPayload{Label: legacy.Label, Path: legacy.Path, Items: legacy.Items}
	hasNext := false
	legacyEnvelope := struct {
		Incremental []legacyIncrementalPayload `json:"incremental,omitempty"`
		HasNext     bool                       `json:"hasNext"`
	}{[]legacyIncrementalPayload{legacy}, hasNext}
	currentEnvelope := struct {
		Incremental []IncrementalPayload `json:"incremental,omitempty"`
		HasNext     bool                 `json:"hasNext"`
	}{[]IncrementalPayload{current}, hasNext}
	legacyResponse := legacyIncrementalResponse{Incremental: []legacyIncrementalPayload{legacy}, HasNext: &hasNext}
	currentResponse := Response{Incremental: []IncrementalPayload{current}, HasNext: &hasNext}
	legacyFull := legacyIncrementalResponse{Data: item, Incremental: make([]legacyIncrementalPayload, 16), HasNext: &hasNext}
	currentFull := Response{Data: item, Incremental: make([]IncrementalPayload, 16), HasNext: &hasNext}
	for index := range legacyFull.Incremental {
		legacyFull.Incremental[index] = legacy
		currentFull.Incremental[index] = current
	}
	for _, tc := range []struct {
		name            string
		legacy, current func() ([]byte, error)
	}{
		{"payload", func() ([]byte, error) { return json.Marshal(legacy) }, func() ([]byte, error) { return json.Marshal(current) }},
		{"json_envelope", func() ([]byte, error) { return json.Marshal(legacyEnvelope) }, func() ([]byte, error) { return json.Marshal(currentEnvelope) }},
		{"http_transport", func() ([]byte, error) { return json.Marshal(legacyEnvelope) }, currentResponse.MarshalJSON},
		{"buffered_response", func() ([]byte, error) { return json.Marshal(legacyFull) }, func() ([]byte, error) { return json.Marshal(currentFull) }},
		{"response", func() ([]byte, error) { return json.Marshal(legacyResponse) }, func() ([]byte, error) { return json.Marshal(currentResponse) }},
	} {
		b.Run(tc.name, func(b *testing.B) {
			before, err := tc.legacy()
			if err != nil {
				b.Fatal(err)
			}
			after, err := tc.current()
			if err != nil {
				b.Fatal(err)
			}
			if string(before) != string(after) {
				b.Fatalf("JSON differs: %s vs %s", before, after)
			}
			b.Run("legacy", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := tc.legacy(); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("current", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := tc.current(); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
