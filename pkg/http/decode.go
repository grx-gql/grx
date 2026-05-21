package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"

	"github.com/patrickkabwe/grx/core"
)

// decodeGraphQLHTTP decodes one or more GraphQL operations from an HTTP
// request. POST bodies may be a single JSON object or a JSON array (Apollo-style
// batching). GET requests always yield exactly one body.
func decodeGraphQLHTTP(r *nethttp.Request, persisted map[string]string) ([]core.GraphQLBody, error) {
	switch r.Method {
	case nethttp.MethodPost:
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		return parsePostGraphQLJSON(raw, persisted)
	case nethttp.MethodGet:
		body, err := core.DecodeGraphQLRequest(r)
		if err != nil {
			return nil, err
		}
		if err := resolvePersistedQuery(&body, persisted); err != nil {
			return nil, err
		}
		if strings.TrimSpace(body.Query) == "" {
			return nil, fmt.Errorf("missing GraphQL query")
		}
		return []core.GraphQLBody{body}, nil
	default:
		return nil, fmt.Errorf("unsupported HTTP method %s", r.Method)
	}
}

func parsePostGraphQLJSON(raw []byte, persisted map[string]string) ([]core.GraphQLBody, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty request body")
	}
	if raw[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("invalid GraphQL JSON body: %s", err.Error())
		}
		out := make([]core.GraphQLBody, 0, len(items))
		for _, item := range items {
			var body core.GraphQLBody
			if err := json.Unmarshal(item, &body); err != nil {
				return nil, fmt.Errorf("invalid GraphQL JSON body: %s", err.Error())
			}
			if err := resolvePersistedQuery(&body, persisted); err != nil {
				return nil, err
			}
			if strings.TrimSpace(body.Query) == "" {
				return nil, fmt.Errorf("missing GraphQL query")
			}
			out = append(out, body)
		}
		return out, nil
	}

	var body core.GraphQLBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("invalid GraphQL JSON body: %s", err.Error())
	}
	if err := resolvePersistedQuery(&body, persisted); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Query) == "" {
		return nil, fmt.Errorf("missing GraphQL query")
	}
	return []core.GraphQLBody{body}, nil
}

func resolvePersistedQuery(body *core.GraphQLBody, persisted map[string]string) error {
	if body == nil || strings.TrimSpace(body.Query) != "" || len(persisted) == 0 {
		return nil
	}
	ext := body.Extensions
	if ext == nil {
		return fmt.Errorf("missing GraphQL query")
	}
	pq, _ := ext["persistedQuery"].(map[string]any)
	if pq == nil {
		return fmt.Errorf("missing GraphQL query")
	}
	rawHash, _ := pq["sha256Hash"].(string)
	hash := strings.ToLower(strings.TrimSpace(rawHash))
	if hash == "" {
		return fmt.Errorf("missing GraphQL query")
	}
	query, ok := persisted[hash]
	if !ok {
		return fmt.Errorf("unknown persisted query sha256Hash %q", rawHash)
	}
	body.Query = query
	return nil
}
