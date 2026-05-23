package core

import (
	"bytes"
	"unicode/utf8"
)

func writeEscapedJSONString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	writeEscapedJSONStringContent(buf, s)
	buf.WriteByte('"')
}

// writeEscapedJSONStringContent escapes s for JSON double-quoted contexts
// excluding the wrapping quotes (used for object keys and string values).
func writeEscapedJSONStringContent(buf *bytes.Buffer, s string) {
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < 0x20 {
			buf.WriteString(s[start:i])
			switch c {
			case '\t':
				buf.WriteString(`\t`)
			case '\n':
				buf.WriteString(`\n`)
			case '\r':
				buf.WriteString(`\r`)
			default:
				const hex = "0123456789abcdef"
				buf.WriteString(`\u00`)
				buf.WriteByte(hex[c>>4])
				buf.WriteByte(hex[c&0xf])
			}
			i++
			start = i
			continue
		}
		switch c {
		case '"', '\\':
			buf.WriteString(s[start:i])
			if c == '"' {
				buf.WriteString(`\"`)
			} else {
				buf.WriteString(`\\`)
			}
			i++
			start = i
			continue
		case '<', '>', '&':
			buf.WriteString(s[start:i])
			switch c {
			case '<':
				buf.WriteString(`\u003c`)
			case '>':
				buf.WriteString(`\u003e`)
			default:
				buf.WriteString(`\u0026`)
			}
			i++
			start = i
			continue
		}
		if c < utf8.RuneSelf {
			i++
			continue
		}
		_, wid := utf8.DecodeRuneInString(s[i:])
		i += wid
	}
	buf.WriteString(s[start:])
}
