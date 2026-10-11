package core

import (
	"fmt"
	"mime"
	"net/http"
	"strconv"
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

type mediaPreference struct {
	quality     float64
	specificity int
}

// NegotiateResponseContentType selects the highest-quality supported response
// media type. Specific ranges override wildcards, including explicit q=0
// exclusions. An absent Accept header selects the legacy application/json.
func NegotiateResponseContentType(accept []string) (string, bool) {
	if len(accept) == 0 {
		return MediaTypeJSON, true
	}
	types := [2]string{MediaTypeGraphQLResponse, MediaTypeJSON}
	preferences := [2]mediaPreference{{specificity: -1}, {specificity: -1}}
	for _, raw := range accept {
		for part := range strings.SplitSeq(raw, ",") {
			candidate, ok := parseAcceptCandidate(part)
			if !ok {
				continue
			}
			for index, mediaType := range types {
				specificity := -1
				switch {
				case candidate.mediaType == mediaType:
					specificity = 2
				case candidate.mediaType == mediaType[:strings.IndexByte(mediaType, '/')]+"/*":
					specificity = 1
				case candidate.mediaType == "*/*":
					specificity = 0
				}
				current := preferences[index]
				if specificity > current.specificity || specificity == current.specificity && candidate.quality > current.quality {
					preferences[index] = mediaPreference{quality: candidate.quality, specificity: specificity}
				}
			}
		}
	}
	best, quality := "", 0.0
	for index, preference := range preferences {
		if preference.specificity >= 0 && preference.quality > quality {
			best, quality = types[index], preference.quality
		}
	}
	return best, best != ""
}

func parseAcceptCandidate(part string) (acceptCandidate, bool) {
	part = strings.TrimSpace(part)
	media, parameters, hasParameters := strings.Cut(part, ";")
	media = strings.TrimSpace(media)
	for _, mediaType := range [...]string{MediaTypeGraphQLResponse, MediaTypeJSON, "application/*", "*/*"} {
		if len(media) != len(mediaType) || !strings.EqualFold(media, mediaType) {
			continue
		}
		if !hasParameters {
			return acceptCandidate{mediaType: mediaType, quality: 1}, true
		}
		if raw, ok := strings.CutPrefix(strings.TrimSpace(parameters), "q="); ok {
			if quality, err := strconv.ParseFloat(raw, 64); err == nil && quality >= 0 && quality <= 1 {
				return acceptCandidate{mediaType: mediaType, quality: quality}, true
			}
		}
		break
	}
	mediaType, params, err := mime.ParseMediaType(part)
	if err != nil {
		return acceptCandidate{}, false
	}
	quality := 1.0
	if raw, exists := params["q"]; exists {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || !(parsed >= 0 && parsed <= 1) {
			quality = 0
		} else {
			quality = parsed
		}
	}
	return acceptCandidate{mediaType: mediaType, quality: quality}, true
}
