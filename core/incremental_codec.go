package core

import (
	"bytes"
	"encoding/json"
)

type incrementalPayloadJSON IncrementalPayload

// MarshalJSON preserves explicit items:null for a failed incremental stream.
func (p IncrementalPayload) MarshalJSON() ([]byte, error) {
	if len(p.Errors) == 0 && len(p.Extensions) == 0 {
		buf := responseDataBufPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer responseDataBufPool.Put(buf)
		buf.WriteByte('{')
		if p.Label != "" {
			writeIncrementalFieldPrefix(buf, "label")
			writeEscapedJSONString(buf, p.Label)
		}
		if len(p.Path) > 0 {
			writeIncrementalFieldPrefix(buf, "path")
			if err := writeIncrementalJSONArray(buf, p.Path); err != nil {
				return nil, err
			}
		}
		if p.Data != nil {
			writeIncrementalFieldPrefix(buf, "data")
			if err := writeIncrementalJSON(buf, p.Data); err != nil {
				return nil, err
			}
		}
		if p.ItemsNull {
			writeIncrementalFieldPrefix(buf, "items")
			buf.WriteString("null")
		} else if len(p.Items) > 0 {
			writeIncrementalFieldPrefix(buf, "items")
			if err := writeIncrementalJSONArray(buf, p.Items); err != nil {
				return nil, err
			}
		}
		buf.WriteByte('}')
		return append([]byte(nil), buf.Bytes()...), nil
	}
	if !p.ItemsNull {
		return json.Marshal(incrementalPayloadJSON(p))
	}
	return json.Marshal(struct {
		incrementalPayloadJSON
		Items any `json:"items"`
	}{incrementalPayloadJSON: incrementalPayloadJSON(p)})
}

// UnmarshalJSON distinguishes a failed stream from a deferred data patch.
func (p *IncrementalPayload) UnmarshalJSON(data []byte) error {
	var decoded incrementalPayloadJSON
	aux := struct {
		*incrementalPayloadJSON
		Items json.RawMessage `json:"items"`
	}{incrementalPayloadJSON: &decoded}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(aux.Items) > 0 {
		decoded.ItemsNull = bytes.Equal(bytes.TrimSpace(aux.Items), []byte("null"))
		if !decoded.ItemsNull {
			if err := json.Unmarshal(aux.Items, &decoded.Items); err != nil {
				return err
			}
		}
	}
	*p = IncrementalPayload(decoded)
	return nil
}

func writeIncrementalFieldPrefix(buf *bytes.Buffer, name string) {
	if buf.Len() > 1 {
		buf.WriteByte(',')
	}
	buf.WriteByte('"')
	buf.WriteString(name)
	buf.WriteString(`":`)
}

// Incremental envelopes retain encoding/json's rejection of non-finite values;
// OrderedObject values keep their own existing GraphQL serialization semantics.
func writeIncrementalJSON(buf *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case nil, bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, *OrderedObject:
		return writeGraphQLJSON(buf, value)
	case []any:
		if value == nil {
			buf.WriteString("null")
			return nil
		}
		return writeIncrementalJSONArray(buf, value)
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		buf.Write(encoded)
		return nil
	}
}

func writeIncrementalJSONArray(buf *bytes.Buffer, values []any) error {
	buf.WriteByte('[')
	for index, value := range values {
		if index > 0 {
			buf.WriteByte(',')
		}
		if err := writeIncrementalJSON(buf, value); err != nil {
			return err
		}
	}
	buf.WriteByte(']')
	return nil
}
