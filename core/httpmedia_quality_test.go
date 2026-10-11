package core

import (
	"mime"
	"strconv"
	"strings"
	"testing"
)

func TestAcceptQualityFastPathMatchesMIMEParsing(t *testing.T) {
	for _, part := range []string{
		"application/json;q=0.8", "application/json;q=0", "application/json;q=1",
		" APPLICATION/JSON ; q=0.5 ", "application/json;q=+1", "application/json;q=1e-1",
		"application/json;q=0x1p-1", "application/json;q=0.5_0", "application/json;q=.5",
		"application/json;Q=0.8", "application/json;q=\"0.8\"", "application/json;q=0.8;level=1",
		"application/json;q=0.8;q=0", "application/json;q=NaN", "application/json;q=+Inf",
		"application/json;q=-1", "application/json;q=2", "application/json;q=", "application/json;q=0.8 garbage",
		"application/json;q=0.\n8", "application/json;q=0.8;", "application/jſon;q=0.8",
	} {
		t.Run(part, func(t *testing.T) {
			mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(part))
			actual, ok := parseAcceptCandidate(part)
			if err != nil {
				if ok {
					t.Fatalf("accepted malformed media range: %+v", actual)
				}
				return
			}
			quality := 1.0
			if raw, present := params["q"]; present {
				parsed, parseErr := strconv.ParseFloat(raw, 64)
				quality = 0
				if parseErr == nil && parsed >= 0 && parsed <= 1 {
					quality = parsed
				}
			}
			if !ok || actual.mediaType != mediaType || actual.quality != quality {
				t.Fatalf("candidate=%+v accepted=%v; want media=%q quality=%g", actual, ok, mediaType, quality)
			}
		})
	}
}
