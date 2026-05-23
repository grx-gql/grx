package core

import (
	"bytes"
	"encoding/json"
	"sync"
)

var responseDataBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// MarshalJSON emits a GraphQL-shaped JSON response. Execution errors that
// fully null the operation result use [Response.DataNull] with nil Data so the
// wire format includes "data":null; failures before execution omit the key.
func (r Response) MarshalJSON() ([]byte, error) {
	// Success-only hot path avoids json.Marshal(aux) reallocating Data bytes.
	if len(r.Incremental) == 0 && len(r.Extensions) == 0 && r.HasNext == nil && len(r.Errors) == 0 {
		if oo, ok := r.Data.(*OrderedObject); ok {
			if r.DataNull {
				const nullData = `{"data":null}`
				return []byte(nullData), nil
			}
			buf := responseDataBufPool.Get().(*bytes.Buffer)
			buf.Reset()
			defer responseDataBufPool.Put(buf)

			if oo == nil {
				buf.WriteString(`{"data":null}`)
			} else {
				buf.Grow(1024)
				buf.WriteString(`{"data":`)
				if err := oo.writeJSONObject(buf); err != nil {
					return nil, err
				}
				buf.WriteByte('}')
			}
			return append([]byte(nil), buf.Bytes()...), nil
		}
	}

	aux := struct {
		Data        json.RawMessage      `json:"data,omitempty"`
		Errors      []Error              `json:"errors,omitempty"`
		Incremental []IncrementalPayload `json:"incremental,omitempty"`
		HasNext     *bool                `json:"hasNext,omitempty"`
		Extensions  map[string]any       `json:"extensions,omitempty"`
	}{
		Errors:      r.Errors,
		Incremental: r.Incremental,
		HasNext:     r.HasNext,
		Extensions:  r.Extensions,
	}
	switch {
	case r.Data != nil:
		b, err := json.Marshal(r.Data)
		if err != nil {
			return nil, err
		}
		aux.Data = b
	case r.DataNull:
		aux.Data = json.RawMessage("null")
	default:
	}

	return json.Marshal(&aux)
}

// UnmarshalJSON decodes GraphQL envelopes including explicit data:null.
func (r *Response) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if v, ok := raw["data"]; ok {
		v = bytes.TrimSpace(v)
		if bytes.Equal(v, []byte("null")) {
			r.DataNull = true
			r.Data = nil
		} else {
			var decoded any
			if err := json.Unmarshal(v, &decoded); err != nil {
				return err
			}
			r.Data = decoded
			r.DataNull = false
		}
	} else {
		r.Data = nil
		r.DataNull = false
	}

	if v, ok := raw["errors"]; ok {
		if err := json.Unmarshal(v, &r.Errors); err != nil {
			return err
		}
	} else {
		r.Errors = nil
	}

	if v, ok := raw["incremental"]; ok {
		if err := json.Unmarshal(v, &r.Incremental); err != nil {
			return err
		}
	} else {
		r.Incremental = nil
	}

	if v, ok := raw["hasNext"]; ok {
		var hn bool
		if err := json.Unmarshal(v, &hn); err != nil {
			return err
		}
		r.HasNext = &hn
	} else {
		r.HasNext = nil
	}

	if v, ok := raw["extensions"]; ok {
		if err := json.Unmarshal(v, &r.Extensions); err != nil {
			return err
		}
	} else {
		r.Extensions = nil
	}

	return nil
}
