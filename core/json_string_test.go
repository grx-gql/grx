package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteEscapedJSONStringsMatchStdlib(t *testing.T) {
	cases := []string{
		"",
		"id",
		`<x>&"`,
		"path\\to",
		"a\nb\tc\rd",
		"plain ASCII",
		string([]byte{0x01, 'x'}),
		"émojis 🚀",
		strings.Repeat("wide ", 20),
	}

	for _, tc := range cases {
		var buf bytes.Buffer
		writeEscapedJSONString(&buf, tc)
		std, err := json.Marshal(tc)
		if err != nil {
			t.Fatalf("marshal %q: %v", tc, err)
		}
		if buf.String() != string(std) {
			t.Fatalf("escape mismatch for %#v\ngot %s\nwant %s", tc, buf.String(), std)
		}
	}
}
