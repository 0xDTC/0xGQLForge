package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// graphqlPayload represents a decoded GraphQL request body.
type graphqlPayload struct {
	Query         string          `json:"query"`
	OperationName string          `json:"operationName"`
	Variables     json.RawMessage `json:"variables"`
	DocID         string          `json:"doc_id,omitempty"`         // persisted query ID (e.g. Instagram/Relay)
	QueryHash     string          `json:"query_hash,omitempty"`     // legacy persisted query hash
	FriendlyName  string          `json:"fb_api_req_friendly_name"` // Meta-style operation name
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

	// Check Content-Type for JSON or form-encoded POST requests
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if r.Method == "POST" && (strings.Contains(ct, "application/json") || strings.Contains(ct, "application/x-www-form-urlencoded")) {
		return true
	}

	return false
}

// ExtractGraphQLPayload reads the GraphQL query, operation name, and variables from a request.
// It replaces the request body so it can still be forwarded.
// Handles JSON, form-encoded, and batch payloads.
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

	ct := strings.ToLower(r.Header.Get("Content-Type"))

	// Try form-encoded body (used by Instagram/Meta GraphQL endpoints)
	if strings.Contains(ct, "application/x-www-form-urlencoded") {
		return parseFormPayload(body)
	}

	// Try to parse as single JSON query
	var p graphqlPayload
	if err := json.Unmarshal(body, &p); err == nil && (p.Query != "" || p.DocID != "") {
		if p.OperationName == "" && p.FriendlyName != "" {
			p.OperationName = p.FriendlyName
		}
		return &p, nil
	}

	// Try batch query (array of queries) — take the first one
	var batch []graphqlPayload
	if err := json.Unmarshal(body, &batch); err == nil && len(batch) > 0 {
		if batch[0].OperationName == "" && batch[0].FriendlyName != "" {
			batch[0].OperationName = batch[0].FriendlyName
		}
		return &batch[0], nil
	}

	return nil, nil
}

// parseFormPayload handles application/x-www-form-urlencoded GraphQL bodies.
// Instagram/Meta send: doc_id=123&variables={"foo":"bar"}&lsd=token&fb_api_req_friendly_name=SomeQuery
func parseFormPayload(body []byte) (*graphqlPayload, error) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, nil
	}

	p := &graphqlPayload{
		Query:         values.Get("query"),
		DocID:         values.Get("doc_id"),
		QueryHash:     values.Get("query_hash"),
		OperationName: values.Get("operationName"),
		FriendlyName:  values.Get("fb_api_req_friendly_name"),
	}

	if vars := values.Get("variables"); vars != "" {
		p.Variables = json.RawMessage(vars)
	}

	// Use friendly name as operation name if none provided
	if p.OperationName == "" && p.FriendlyName != "" {
		p.OperationName = p.FriendlyName
	}

	// Nothing useful extracted
	if p.Query == "" && p.DocID == "" && p.QueryHash == "" {
		return nil, nil
	}

	return p, nil
}

// tryExtractPayloadRetroactive attempts to extract a GraphQL payload from a request
// that was not initially detected as GraphQL (response-based fallback).
func tryExtractPayloadRetroactive(r *http.Request) *graphqlPayload {
	if r.Method != "POST" {
		return nil
	}
	// Body was already consumed and replaced by client.Do — we can't re-read it.
	// Return a minimal payload with URL info so the traffic is at least captured.
	return &graphqlPayload{
		OperationName: "unknown",
		Query:         "",
	}
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
