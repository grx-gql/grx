package http

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	nethttp "net/http"
	"strconv"
	"strings"

	"github.com/grx-gql/grx/core"
)

// defaultMultipartMemory bounds the bytes ParseMultipartForm keeps in memory;
// larger file parts spill to temporary files managed by net/http.
const defaultMultipartMemory = 32 << 20 // 32 MiB

// decodeGraphQLHTTP decodes one or more GraphQL operations from an HTTP
// request. POST bodies may be a single JSON object or a JSON array (Apollo-style
// batching). GET requests always yield exactly one body.
func decodeGraphQLHTTP(r *nethttp.Request, config Config) ([]core.GraphQLBody, error) {
	switch r.Method {
	case nethttp.MethodPost:
		if core.IsMultipartRequest(r) {
			return parseMultipartGraphQL(r, config)
		}
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

// parseMultipartGraphQL decodes a GraphQL multipart request per
// https://github.com/jaydenseric/graphql-multipart-request-spec. The form must
// carry an "operations" field (the GraphQL request, single object or batch
// array) and a "map" field associating uploaded file parts with null
// placeholders in the operations. Each referenced placeholder is replaced with
// a *core.Upload before the bodies are executed.
func parseMultipartGraphQL(r *nethttp.Request, config Config) ([]core.GraphQLBody, error) {
	if err := r.ParseMultipartForm(defaultMultipartMemory); err != nil {
		return nil, fmt.Errorf("invalid multipart request: %s", err.Error())
	}

	operations := r.FormValue("operations")
	if strings.TrimSpace(operations) == "" {
		return nil, fmt.Errorf("multipart request missing \"operations\" field")
	}
	mapField := r.FormValue("map")
	if strings.TrimSpace(mapField) == "" {
		return nil, fmt.Errorf("multipart request missing \"map\" field")
	}

	var fileMap map[string][]string
	if err := json.Unmarshal([]byte(mapField), &fileMap); err != nil {
		return nil, fmt.Errorf("invalid multipart \"map\" field: %s", err.Error())
	}

	rawOps := bytes.TrimSpace([]byte(operations))
	isBatch := len(rawOps) > 0 && rawOps[0] == '['

	bodies, err := parsePostGraphQLJSONLenient(rawOps, config)
	if err != nil {
		return nil, err
	}

	for partName, paths := range fileMap {
		header := firstFileHeader(r, partName)
		if header == nil {
			return nil, fmt.Errorf("multipart request references missing file part %q", partName)
		}
		upload := core.NewUpload(header)
		for _, path := range paths {
			if err := injectUpload(bodies, isBatch, path, upload); err != nil {
				return nil, err
			}
		}
	}

	for i := range bodies {
		if strings.TrimSpace(bodies[i].Query) == "" {
			return nil, fmt.Errorf("missing GraphQL query")
		}
	}
	return bodies, nil
}

// parsePostGraphQLJSONLenient decodes operations bytes into one or more bodies
// without enforcing the non-empty-query rule, which is checked after uploads
// are injected.
func parsePostGraphQLJSONLenient(raw []byte, config Config) ([]core.GraphQLBody, error) {
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
	return []core.GraphQLBody{body}, nil
}

func firstFileHeader(r *nethttp.Request, partName string) *multipart.FileHeader {
	if r.MultipartForm == nil {
		return nil
	}
	headers := r.MultipartForm.File[partName]
	if len(headers) == 0 {
		return nil
	}
	return headers[0]
}

// injectUpload resolves a multipart object path (e.g. "variables.file" or, for
// a batch, "1.variables.files.0") and replaces the value at that location with
// the given upload.
func injectUpload(bodies []core.GraphQLBody, isBatch bool, path string, upload *core.Upload) error {
	segments := strings.Split(path, ".")
	if len(segments) == 0 {
		return fmt.Errorf("invalid multipart map path %q", path)
	}

	index := 0
	if isBatch {
		parsed, err := strconv.Atoi(segments[0])
		if err != nil {
			return fmt.Errorf("invalid multipart map path %q: expected batch index", path)
		}
		index = parsed
		segments = segments[1:]
	}
	if index < 0 || index >= len(bodies) {
		return fmt.Errorf("multipart map path %q references operation index out of range", path)
	}
	if len(segments) < 2 || segments[0] != "variables" {
		return fmt.Errorf("multipart map path %q must target a variable", path)
	}

	if bodies[index].Variables == nil {
		return fmt.Errorf("multipart map path %q references unknown variable", path)
	}
	return setAtPath(bodies[index].Variables, segments[1:], upload)
}

// setAtPath walks container following segments and assigns value at the leaf.
// container is either a map[string]any (object key) or []any (numeric index).
func setAtPath(container any, segments []string, value any) error {
	if len(segments) == 0 {
		return fmt.Errorf("empty upload path")
	}
	seg := segments[0]
	last := len(segments) == 1

	switch node := container.(type) {
	case map[string]any:
		if last {
			node[seg] = value
			return nil
		}
		child, ok := node[seg]
		if !ok {
			return fmt.Errorf("upload path segment %q not found", seg)
		}
		return setAtPath(child, segments[1:], value)
	case []any:
		idx, err := strconv.Atoi(seg)
		if err != nil || idx < 0 || idx >= len(node) {
			return fmt.Errorf("invalid upload path index %q", seg)
		}
		if last {
			node[idx] = value
			return nil
		}
		return setAtPath(node[idx], segments[1:], value)
	default:
		return fmt.Errorf("cannot descend into upload path segment %q", seg)
	}
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
