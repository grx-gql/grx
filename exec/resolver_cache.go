package exec

import (
	"math"
	"reflect"
	"sort"
	"strconv"

	"github.com/grx-gql/grx/schema"
)

// resolverCacheKey uses field identity and a typed, length-delimited input
// encoding. Custom runtime values are not memoized: application encoders may
// erase distinctions that a resolver can still observe.
func resolverCacheKey(field *schema.Field, source any, args map[string]any) (string, bool) {
	sourceKey, ok := sourceIdentity(source)
	if !ok {
		return "", false
	}
	var scratch [256]byte
	encoded := append(scratch[:0], "0x"...)
	encoded = strconv.AppendUint(encoded, uint64(reflect.ValueOf(field).Pointer()), 16)
	encoded = append(encoded, 0)
	encoded = append(encoded, sourceKey...)
	encoded = append(encoded, 0)
	encoded, ok = appendResolverCacheValue(encoded, args)
	if !ok {
		return "", false
	}
	return string(encoded), true
}

func appendResolverCacheValue(dst []byte, value any) ([]byte, bool) {
	switch typed := value.(type) {
	case nil:
		return append(dst, 'n'), true
	case bool:
		if typed {
			return append(dst, 't'), true
		}
		return append(dst, 'f'), true
	case string:
		dst = append(dst, 's')
		dst = strconv.AppendInt(dst, int64(len(typed)), 10)
		dst = append(dst, ':')
		return append(dst, typed...), true
	case int, int8, int16, int32, int64:
		raw := reflect.ValueOf(typed)
		dst = append(dst, byte(raw.Kind()))
		dst = strconv.AppendInt(dst, raw.Int(), 10)
		return append(dst, ';'), true
	case uint, uint8, uint16, uint32, uint64:
		raw := reflect.ValueOf(typed)
		dst = append(dst, byte(raw.Kind()))
		dst = strconv.AppendUint(dst, raw.Uint(), 10)
		return append(dst, ';'), true
	case float32:
		dst = append(dst, byte(reflect.Float32))
		dst = strconv.AppendUint(dst, uint64(math.Float32bits(typed)), 16)
		return append(dst, ';'), true
	case float64:
		dst = append(dst, byte(reflect.Float64))
		dst = strconv.AppendUint(dst, math.Float64bits(typed), 16)
		return append(dst, ';'), true
	case []any:
		if typed == nil {
			return append(dst, 'L'), true
		}
		dst = append(dst, '[')
		for _, item := range typed {
			var ok bool
			dst, ok = appendResolverCacheValue(dst, item)
			if !ok {
				return nil, false
			}
		}
		return append(dst, ']'), true
	case map[string]any:
		if typed == nil {
			return append(dst, 'M'), true
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		dst = append(dst, '{')
		for _, key := range keys {
			dst, _ = appendResolverCacheValue(dst, key)
			var ok bool
			dst, ok = appendResolverCacheValue(dst, typed[key])
			if !ok {
				return nil, false
			}
		}
		return append(dst, '}'), true
	default:
		return nil, false
	}
}
