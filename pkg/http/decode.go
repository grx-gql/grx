package http

import (
	"bytes"
	"crypto/sha256"
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
func decodeGraphQLHTTP(r *nethttp.Request, config Config) ([]core.GraphQLBody, error) {
	switch r.Method {
	case nethttp.MethodPost:
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		return parsePostGraphQLJSON(raw, config)
	case nethttp.MethodGet:
		body, err := core.DecodeGraphQLRequest(r)
		if err != nil {
			return nil, err
		}
		if err := validateVariableBytes(body.Variables, config.MaxVariableBytes); err != nil {
			return nil, err
		}
		if err := resolvePersistedQuery(&body, config); err != nil {
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

func parsePostGraphQLJSON(raw []byte, config Config) ([]core.GraphQLBody, error) {
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
			if err := validateVariableBytes(body.Variables, config.MaxVariableBytes); err != nil {
				return nil, err
			}
			if err := resolvePersistedQuery(&body, config); err != nil {
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
	if err := validateVariableBytes(body.Variables, config.MaxVariableBytes); err != nil {
		return nil, err
	}
	if err := resolvePersistedQuery(&body, config); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Query) == "" {
		return nil, fmt.Errorf("missing GraphQL query")
	}
	return []core.GraphQLBody{body}, nil
}

func validateVariableBytes(variables map[string]any, max int64) error {
	if max <= 0 || len(variables) == 0 {
		return nil
	}
	raw, err := json.Marshal(variables)
	if err != nil {
		return fmt.Errorf("invalid GraphQL variables: %s", err.Error())
	}
	if int64(len(raw)) > max {
		return fmt.Errorf("GraphQL variables exceed %d byte limit", max)
	}
	return nil
}

func resolvePersistedQuery(body *core.GraphQLBody, config Config) error {
	if body == nil {
		return nil
	}
	persisted := config.PersistedQueries
	ext := body.Extensions
	if ext == nil {
		if config.RequirePersistedQuery {
			return fmt.Errorf("persisted query is required")
		}
		return nil
	}
	pq, _ := ext["persistedQuery"].(map[string]any)
	if pq == nil {
		if config.RequirePersistedQuery {
			return fmt.Errorf("persisted query is required")
		}
		return nil
	}
	rawHash, _ := pq["sha256Hash"].(string)
	hash := strings.ToLower(strings.TrimSpace(rawHash))
	if hash == "" {
		if config.RequirePersistedQuery {
			return fmt.Errorf("persisted query sha256Hash is required")
		}
		return nil
	}
	if err := validatePersistedQueryMetadata(pq, hash); err != nil {
		return err
	}

	query := strings.TrimSpace(body.Query)
	if query != "" {
		if config.StrictPersistedQueries {
			sum := sha256.Sum256([]byte(body.Query))
			if hash != fmt.Sprintf("%x", sum[:]) {
				return fmt.Errorf("persisted query sha256Hash does not match GraphQL query")
			}
		}
		return nil
	}
	query, ok := persisted[hash]
	if !ok {
		return fmt.Errorf("unknown persisted query sha256Hash %q", rawHash)
	}
	body.Query = query
	return nil
}

func validatePersistedQueryMetadata(pq map[string]any, hash string) error {
	version, ok := pq["version"]
	if !ok {
		return fmt.Errorf("persisted query version is required")
	}
	if number, ok := version.(float64); !ok || number != 1 {
		return fmt.Errorf("unsupported persisted query version %v", version)
	}
	if len(hash) != 64 {
		return fmt.Errorf("persisted query sha256Hash must be 64 lowercase hex characters")
	}
	for _, ch := range hash {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return fmt.Errorf("persisted query sha256Hash must be 64 lowercase hex characters")
		}
	}
	return nil
}
