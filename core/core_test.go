package core

import (
	"encoding/json"
	"testing"
)

func TestResponseMarshalsIncrementalPayloadFields(t *testing.T) {
	hasNext := true
	payload := Response{
		Incremental: []IncrementalPayload{
			{
				Path: []any{"user", "friends", 0},
				Items: []any{
					map[string]any{"id": "friend_1"},
				},
				Errors: []Error{
					{
						Message: "friend resolver failed",
						Locations: []Location{
							{Line: 3, Column: 7},
						},
						Extensions: map[string]any{
							"classification": "field",
						},
					},
				},
				Extensions: map[string]any{
					"traceID": "abc123",
				},
				Label: "friends-stream",
			},
		},
		HasNext: &hasNext,
		Extensions: map[string]any{
			"requestID": "req_123",
		},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if decoded["hasNext"] != true {
		t.Fatalf("expected hasNext=true, got %#v", decoded["hasNext"])
	}
	extensions, ok := decoded["extensions"].(map[string]any)
	if !ok || extensions["requestID"] != "req_123" {
		t.Fatalf("expected top-level extensions, got %#v", decoded["extensions"])
	}
	incremental, ok := decoded["incremental"].([]any)
	if !ok || len(incremental) != 1 {
		t.Fatalf("expected one incremental payload, got %#v", decoded["incremental"])
	}

	entry, ok := incremental[0].(map[string]any)
	if !ok {
		t.Fatalf("expected incremental payload object, got %T", incremental[0])
	}
	if entry["label"] != "friends-stream" {
		t.Fatalf("expected label friends-stream, got %#v", entry["label"])
	}
}
