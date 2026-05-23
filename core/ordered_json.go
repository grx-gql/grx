package core

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
)

// writeGraphQLScalar appends JSON for values produced by execution (nested
// objects and lists recurse without per-level json.Marshal slicing).
func writeGraphQLJSON(buf *bytes.Buffer, v any) error {
	switch v := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		writeEscapedJSONString(buf, v)
	case int:
		buf.Write(strconv.AppendInt(nil, int64(v), 10))
	case int8:
		buf.Write(strconv.AppendInt(nil, int64(v), 10))
	case int16:
		buf.Write(strconv.AppendInt(nil, int64(v), 10))
	case int32:
		buf.Write(strconv.AppendInt(nil, int64(v), 10))
	case int64:
		buf.Write(strconv.AppendInt(nil, v, 10))
	case uint:
		buf.Write(strconv.AppendUint(nil, uint64(v), 10))
	case uint8:
		buf.Write(strconv.AppendUint(nil, uint64(v), 10))
	case uint16:
		buf.Write(strconv.AppendUint(nil, uint64(v), 10))
	case uint32:
		buf.Write(strconv.AppendUint(nil, uint64(v), 10))
	case uint64:
		buf.Write(strconv.AppendUint(nil, v, 10))
	case float32:
		writeFiniteFloat(buf, float64(v), 32)
	case float64:
		writeFiniteFloat(buf, v, 64)
	case *OrderedObject:
		if v == nil {
			buf.WriteString("null")
			return nil
		}
		return v.writeJSONObject(buf)
	case []*OrderedObject:
		buf.WriteByte('[')
		for i := range v {
			if i > 0 {
				buf.WriteByte(',')
			}
			elem := v[i]
			if elem == nil {
				buf.WriteString("null")
				continue
			}
			if err := elem.writeJSONObject(buf); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case []any:
		buf.WriteByte('[')
		for i := range v {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeGraphQLJSON(buf, v[i]); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case json.Marshaler:
		if v == nil {
			buf.WriteString("null")
			return nil
		}
		b, err := v.MarshalJSON()
		if err != nil {
			return err
		}
		buf.Write(b)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		buf.Write(b)
	}
	return nil
}

func writeFiniteFloat(buf *bytes.Buffer, f float64, bits int) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		buf.WriteString("null")
		return
	}
	buf.Write(strconv.AppendFloat(nil, f, 'g', -1, bits))
}

func (o *OrderedObject) writeJSONObject(buf *bytes.Buffer) error {
	fields := o.fields
	if len(fields) == 0 {
		buf.WriteString("{}")
		return nil
	}
	buf.WriteByte('{')
	for i := range fields {
		if i > 0 {
			buf.WriteByte(',')
		}
		f := fields[i]
		buf.WriteByte('"')
		writeEscapedJSONStringContent(buf, f.Name)
		buf.WriteByte('"')
		buf.WriteByte(':')
		if err := writeGraphQLJSON(buf, f.Value); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

// MarshalJSON writes the object while preserving insertion order, using one
// growing buffer instead of allocating a [][]byte pyramid.
func (o *OrderedObject) MarshalJSON() ([]byte, error) {
	if len(o.fields) == 0 {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	buf.Grow(len(o.fields) * 32)
	if err := o.writeJSONObject(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
