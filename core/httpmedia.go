package core

import (
	"fmt"
	"mime"
	"net/http"
	"strings"
)

// Official GraphQL-over-HTTP media types (graphql/graphql-over-http).
const (
	MediaTypeJSON            = "application/json"
	MediaTypeGraphQLResponse = "application/graphql-response+json"
)

// ValidatePostContentType ensures POST requests declare a supported GraphQL
// request body media type. Servers MUST accept application/json per the spec.
func ValidatePostContentType(r *http.Request) error {
	raw := strings.TrimSpace(r.Header.Get("Content-Type"))
	if raw == "" {
		return fmt.Errorf("missing Content-Type header")
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return fmt.Errorf("unsupported Content-Type %q", raw)
	}
	if mediaType != MediaTypeJSON {
		return fmt.Errorf("unsupported Content-Type %q", mediaType)
	}
	return nil
}

type acceptCandidate struct {
	mediaType string
	quality   float64
}

// NegotiateResponseContentType selects a response media type from Accept
// values per GraphQL-over-HTTP. An absent Accept header selects the legacy
// application/json type for backward compatibility with clients that omit it.
func NegotiateResponseContentType(accept []string) (string, bool) {
	if len(accept) == 0 {
		return MediaTypeJSON, true
	}

	best := acceptCandidate{quality: -1}
	for _, raw := range accept {
		for _, part := range strings.Split(raw, ",") {
			candidate, ok := parseAcceptCandidate(part)
			if !ok {
				continue
			}
			if candidate.quality > best.quality ||
				(candidate.quality == best.quality && preferGraphQLResponse(candidate.mediaType, best.mediaType)) {
				best = candidate
			}
		}
	}
	if best.quality < 0 {
		return "", false
	}
	return best.mediaType, true
}

func parseAcceptCandidate(part string) (acceptCandidate, bool) {
	part = strings.TrimSpace(part)
	if part == "" {
		return acceptCandidate{}, false
	}

	mediaType := part
	quality := 1.0
	if semi := strings.Index(part, ";"); semi >= 0 {
		mediaType = strings.TrimSpace(part[:semi])
		for _, param := range strings.Split(part[semi+1:], ";") {
			param = strings.TrimSpace(param)
			if !strings.HasPrefix(strings.ToLower(param), "q=") {
				continue
			}
			var parsed float64
			if _, err := fmt.Sscanf(param[2:], "%f", &parsed); err == nil {
				quality = parsed
			}
		}
	}

	switch strings.ToLower(mediaType) {
	case MediaTypeGraphQLResponse:
		return acceptCandidate{mediaType: MediaTypeGraphQLResponse, quality: quality}, true
	case MediaTypeJSON:
		return acceptCandidate{mediaType: MediaTypeJSON, quality: quality}, true
	case "*/*":
		return acceptCandidate{mediaType: MediaTypeGraphQLResponse, quality: quality}, true
	case "multipart/mixed":
		// Incremental delivery clients advertise multipart/mixed. The transport
		// upgrades to a streamed response only when the operation uses
		// @defer/@stream; otherwise it serves a normal JSON body, so negotiate
		// JSON here to keep the non-incremental path valid.
		return acceptCandidate{mediaType: MediaTypeJSON, quality: quality}, true
	default:
		return acceptCandidate{}, false
	}
}

func preferGraphQLResponse(left, right string) bool {
	return left == MediaTypeGraphQLResponse && right != MediaTypeGraphQLResponse
}
