package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// graphqlPayload represents a decoded GraphQL request body.
type graphqlPayload struct {
	Query         string          `json:"query"`
	OperationName string          `json:"operationName"`
	Variables     json.RawMessage `json:"variables"`
}

// IsGraphQLRequest determines if an HTTP request is a GraphQL operation.
func IsGraphQLRequest(r *http.Request) bool {
	// Check URL path
	path := strings.ToLower(r.URL.Path)
	if strings.Contains(path, "graphql") || strings.Contains(path, "gql") {
		return true
	}

	// Check for GET with query parameter
	if r.Method == "GET" && r.URL.Query().Get("query") != "" {
		return true
	}

	// Check Content-Type for JSON (POST requests)
	ct := r.Header.Get("Content-Type")
	if r.Method == "POST" && strings.Contains(ct, "application/json") {
		return true
	}

	return false
}

// ExtractGraphQLPayload reads the GraphQL query, operation name, and variables from a request.
// It replaces the request body so it can still be forwarded.
func ExtractGraphQLPayload(r *http.Request) (*graphqlPayload, error) {
	if r.Method == "GET" {
		q := r.URL.Query()
		p := &graphqlPayload{
			Query:         q.Get("query"),
			OperationName: q.Get("operationName"),
		}
		if vars := q.Get("variables"); vars != "" {
			p.Variables = json.RawMessage(vars)
		}
		return p, nil
	}

	// POST — read and restore the body
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Try to parse as single query
	var p graphqlPayload
	if err := json.Unmarshal(body, &p); err == nil && p.Query != "" {
		return &p, nil
	}

	// Try batch query (array of queries) — take the first one
	var batch []graphqlPayload
	if err := json.Unmarshal(body, &batch); err == nil && len(batch) > 0 {
		return &batch[0], nil
	}

	return nil, nil
}

// ExtractOperationName attempts to parse the operation name from a GraphQL query string.
func ExtractOperationName(query string) string {
	query = strings.TrimSpace(query)

	// Look for named operations: query Foo, mutation Bar, subscription Baz
	for _, prefix := range []string{"query ", "mutation ", "subscription "} {
		idx := strings.Index(query, prefix)
		if idx < 0 {
			continue
		}
		rest := query[idx+len(prefix):]
		// Read until ( or { or whitespace
		var name strings.Builder
		for _, c := range rest {
			if c == '(' || c == '{' || c == ' ' || c == '\n' || c == '\r' || c == '\t' {
				break
			}
			name.WriteRune(c)
		}
		if name.Len() > 0 {
			return name.String()
		}
	}

	return ""
}

// DetectGraphQLResponse checks if a response body looks like a GraphQL response.
func DetectGraphQLResponse(body []byte) bool {
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	_, hasData := resp["data"]
	_, hasErrors := resp["errors"]
	return hasData || hasErrors
}
