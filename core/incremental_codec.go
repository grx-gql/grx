package core

import (
	"bytes"
	"encoding/json"
)

type incrementalPayloadJSON IncrementalPayload

// MarshalJSON preserves explicit items:null for a failed incremental stream.
func (p IncrementalPayload) MarshalJSON() ([]byte, error) {
	if len(p.Errors) > 0 || len(p.Extensions) > 0 {
		return marshalIncrementalPayloadMetadata(p)
	}
	buf := responseDataBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer responseDataBufPool.Put(buf)
	if err := writeIncrementalPayload(buf, p); err != nil {
		return nil, err
	}
	return append([]byte(nil), buf.Bytes()...), nil
}

func writeIncrementalPayload(buf *bytes.Buffer, p IncrementalPayload) error {
	if len(p.Errors) > 0 || len(p.Extensions) > 0 {
		encoded, err := marshalIncrementalPayloadMetadata(p)
		if err != nil {
			return err
		}
		buf.Write(encoded)
		return nil
	}
	start := buf.Len()
	buf.WriteByte('{')
	if p.Label != "" {
		writeIncrementalFieldPrefix(buf, start, `"label":`)
		writeEscapedJSONString(buf, p.Label)
	}
	if len(p.Path) > 0 {
		writeIncrementalFieldPrefix(buf, start, `"path":`)
		if err := writeIncrementalJSONArray(buf, p.Path); err != nil {
			return err
		}
	}
	if p.Data != nil {
		writeIncrementalFieldPrefix(buf, start, `"data":`)
		if err := writeIncrementalJSON(buf, p.Data); err != nil {
			return err
		}
	}
	if p.ItemsNull {
		writeIncrementalFieldPrefix(buf, start, `"items":`)
		buf.WriteString("null")
	} else if len(p.Items) > 0 {
		writeIncrementalFieldPrefix(buf, start, `"items":`)
		if err := writeIncrementalJSONArray(buf, p.Items); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func marshalIncrementalPayloadMetadata(p IncrementalPayload) ([]byte, error) {
	if !p.ItemsNull {
		return json.Marshal(incrementalPayloadJSON(p))
	}
	return json.Marshal(struct {
		incrementalPayloadJSON
		Items any `json:"items"`
	}{incrementalPayloadJSON: incrementalPayloadJSON(p)})
}

// marshalIncrementalResponse writes patches into one owned envelope buffer so
// nested MarshalJSON methods do not repeatedly allocate and validate JSON.
func marshalIncrementalResponse(response Response) ([]byte, error) {
	buf := responseDataBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer responseDataBufPool.Put(buf)
	buf.WriteByte('{')
	if response.Data != nil {
		buf.WriteString(`"data":`)
		if err := writeIncrementalResponseData(buf, response.Data); err != nil {
			return nil, err
		}
	} else if response.DataNull {
		buf.WriteString(`"data":null`)
	}
	if len(response.Errors) > 0 {
		writeIncrementalFieldPrefix(buf, 0, `"errors":`)
		if err := writeIncrementalJSON(buf, response.Errors); err != nil {
			return nil, err
		}
	}
	writeIncrementalFieldPrefix(buf, 0, `"incremental":[`)
	for index, patch := range response.Incremental {
		if index > 0 {
			buf.WriteByte(',')
		}
		if err := writeIncrementalPayload(buf, patch); err != nil {
			return nil, err
		}
	}
	buf.WriteByte(']')
	if response.HasNext != nil {
		if *response.HasNext {
			buf.WriteString(`,"hasNext":true`)
		} else {
			buf.WriteString(`,"hasNext":false`)
		}
	}
	if len(response.Extensions) > 0 {
		writeIncrementalFieldPrefix(buf, 0, `"extensions":`)
		if err := writeIncrementalJSON(buf, response.Extensions); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return append([]byte(nil), buf.Bytes()...), nil
}

func writeIncrementalResponseData(buf *bytes.Buffer, data any) error {
	if ordered, ok := data.(*OrderedObject); ok {
		return writeGraphQLJSON(buf, ordered)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	buf.Write(encoded)
	return nil
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

func writeIncrementalFieldPrefix(buf *bytes.Buffer, start int, prefix string) {
	if buf.Len() > start+1 {
		buf.WriteByte(',')
	}
	buf.WriteString(prefix)
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
