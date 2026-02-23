// Package inference builds a synthetic GraphQL schema from captured proxy traffic.
package inference

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/0xDTC/0xGQLForge/internal/parser"
	"github.com/0xDTC/0xGQLForge/internal/schema"
)

var opDeclRe = regexp.MustCompile(`(?i)^\s*(query|mutation|subscription)\b`)

// BuildFromTraffic synthesises a Schema from captured proxy traffic.
//
// Strategy (best-data-first):
//  1. If any response body is a GraphQL introspection response, parse it
//     directly — this gives a complete, accurate schema.
//  2. Otherwise walk every response body's "data" object to infer object
//     types from the actual JSON shape, producing real graph edges.
//  3. Fall back to operation-name-only entries for requests with no
//     parseable response.
func BuildFromTraffic(reqs []schema.CapturedRequest, projectName string) *schema.Schema {
	// Phase 1 — introspection auto-detection.
	for _, req := range reqs {
		if s := tryParseIntrospection(req.ResponseBody, projectName); s != nil {
			return s
		}
	}

	// Phase 2 — response-body type inference.
	typeMap := map[string]schema.Type{}
	queryFields := map[string]schema.Field{}
	mutFields := map[string]schema.Field{}
	subFields := map[string]schema.Field{}

	for _, req := range reqs {
		if req.Query == "" {
			continue
		}
		opKind := parseOpKind(req.Query)
		bucket := queryFields
		if opKind == "mutation" {
			bucket = mutFields
		} else if opKind == "subscription" {
			bucket = subFields
		}

		// Try to extract real types from the response body.
		rootFields, newTypes := inferFromResponse(req.ResponseBody)

		for name, t := range newTypes {
			if existing, ok := typeMap[name]; ok {
				typeMap[name] = mergeType(existing, t)
			} else {
				typeMap[name] = t
			}
		}

		for _, f := range rootFields {
			if _, ok := bucket[f.Name]; !ok {
				bucket[f.Name] = f
			}
		}

		// Fallback: if we got nothing from the response, at least record
		// the operation name so it appears in the schema.
		if len(rootFields) == 0 && req.OperationName != "" {
			if _, ok := bucket[req.OperationName]; !ok {
				bucket[req.OperationName] = schema.Field{Name: req.OperationName, Type: unknownRef()}
			}
		}
	}

	s := &schema.Schema{
		ID:        generateID(),
		Name:      projectName + " (inferred)",
		Source:    schema.SourceReconstruction,
		QueryType: "Query",
		CreatedAt: time.Now().UTC(),
	}

	// Emit all inferred object types first (so graph edges can reference them).
	for _, t := range typeMap {
		s.Types = append(s.Types, t)
	}

	// Query root.
	qt := schema.Type{Name: "Query", Kind: schema.KindObject}
	for _, f := range queryFields {
		qt.Fields = append(qt.Fields, f)
	}
	if len(qt.Fields) == 0 {
		qt.Fields = append(qt.Fields, schema.Field{
			Name:        "_placeholder",
			Type:        unknownRef(),
			Description: "No query operations captured yet",
		})
	}
	s.Types = append(s.Types, qt)

	if len(mutFields) > 0 {
		mt := schema.Type{Name: "Mutation", Kind: schema.KindObject}
		for _, f := range mutFields {
			mt.Fields = append(mt.Fields, f)
		}
		s.Types = append(s.Types, mt)
		s.MutationType = "Mutation"
	}

	if len(subFields) > 0 {
		st := schema.Type{Name: "Subscription", Kind: schema.KindObject}
		for _, f := range subFields {
			st.Fields = append(st.Fields, f)
		}
		s.Types = append(s.Types, st)
		s.SubscriptionType = "Subscription"
	}

	return s
}

// tryParseIntrospection attempts to parse body as a GraphQL introspection
// response. Returns nil if body is empty or not an introspection response.
func tryParseIntrospection(body json.RawMessage, projectName string) *schema.Schema {
	if len(body) == 0 || !bytes.Contains(body, []byte("__schema")) {
		return nil
	}
	s, err := parser.ParseIntrospection(body, generateID(), projectName+" (inferred)")
	if err != nil || s == nil || len(s.Types) == 0 {
		return nil
	}
	s.CreatedAt = time.Now().UTC()
	return s
}

// inferFromResponse parses a GraphQL response `{"data":{...}}` and returns
// the top-level fields (for the root type) plus any object types discovered
// while walking the JSON tree.
func inferFromResponse(body json.RawMessage) (rootFields []schema.Field, types map[string]schema.Type) {
	types = map[string]schema.Type{}
	if len(body) == 0 {
		return
	}

	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || len(resp.Data) == 0 || string(resp.Data) == "null" {
		return
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return
	}

	for fieldName, value := range data {
		typeRef := inferTypeRef(fieldName, value, types)
		rootFields = append(rootFields, schema.Field{
			Name: fieldName,
			Type: typeRef,
		})
	}
	return
}

// inferTypeRef recursively inspects a JSON value and returns the matching
// TypeRef, creating new object-type entries in `types` as it goes.
func inferTypeRef(fieldName string, value json.RawMessage, types map[string]schema.Type) schema.TypeRef {
	if len(value) == 0 || string(value) == "null" {
		return unknownRef()
	}

	switch value[0] {
	case '"':
		if isIDField(fieldName) {
			return scalarRef("ID")
		}
		return scalarRef("String")

	case 't', 'f': // true / false
		return scalarRef("Boolean")

	case '[':
		var arr []json.RawMessage
		if err := json.Unmarshal(value, &arr); err != nil || len(arr) == 0 {
			return listRef(unknownRef())
		}
		// Use the first non-null element to determine the element type.
		for _, elem := range arr {
			if string(elem) != "null" {
				return listRef(inferTypeRef(singularize(fieldName), elem, types))
			}
		}
		return listRef(unknownRef())

	case '{':
		typeName := pascalCase(fieldName)
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(value, &obj); err != nil {
			return objectRef(typeName)
		}
		t := schema.Type{Name: typeName, Kind: schema.KindObject}
		for subField, subValue := range obj {
			subRef := inferTypeRef(subField, subValue, types)
			t.Fields = append(t.Fields, schema.Field{Name: subField, Type: subRef})
		}
		if existing, ok := types[typeName]; ok {
			types[typeName] = mergeType(existing, t)
		} else {
			types[typeName] = t
		}
		return objectRef(typeName)

	default: // number
		s := string(value)
		if strings.ContainsAny(s, ".eE") {
			return scalarRef("Float")
		}
		return scalarRef("Int")
	}
}

// mergeType combines fields from two definitions of the same type, keeping
// all unique field names seen across both.
func mergeType(a, b schema.Type) schema.Type {
	fieldMap := map[string]schema.Field{}
	for _, f := range a.Fields {
		fieldMap[f.Name] = f
	}
	for _, f := range b.Fields {
		if _, exists := fieldMap[f.Name]; !exists {
			fieldMap[f.Name] = f
		}
	}
	merged := a
	merged.Fields = nil
	for _, f := range fieldMap {
		merged.Fields = append(merged.Fields, f)
	}
	return merged
}

// parseOpKind returns "query", "mutation", or "subscription".
func parseOpKind(query string) string {
	for _, line := range strings.Split(query, "\n") {
		m := opDeclRe.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil {
			return strings.ToLower(m[1])
		}
	}
	return "query"
}

func isIDField(name string) bool {
	lower := strings.ToLower(name)
	return lower == "id" || strings.HasSuffix(lower, "id") || strings.HasSuffix(lower, "_id")
}

// pascalCase capitalises the first letter: "userProfile" → "UserProfile".
func pascalCase(s string) string {
	if s == "" {
		return "Unknown"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// singularize strips a trailing 's' for common plurals used in list fields.
// "users" → "User", "categories" → "Category", "posts" → "Post".
func singularize(s string) string {
	switch {
	case strings.HasSuffix(s, "ies") && len(s) > 4:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "ses") && len(s) > 4:
		return s[:len(s)-2]
	case strings.HasSuffix(s, "s") && len(s) > 3:
		return s[:len(s)-1]
	}
	return s
}

func scalarRef(name string) schema.TypeRef {
	n := name
	return schema.TypeRef{Kind: schema.KindScalar, Name: &n}
}

func objectRef(name string) schema.TypeRef {
	n := name
	return schema.TypeRef{Kind: schema.KindObject, Name: &n}
}

func listRef(of schema.TypeRef) schema.TypeRef {
	return schema.TypeRef{Kind: schema.KindList, OfType: &of}
}

func unknownRef() schema.TypeRef {
	n := "String"
	return schema.TypeRef{Kind: schema.KindScalar, Name: &n}
}

func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("inf_%d", time.Now().UnixNano())
	}
	return "inf_" + hex.EncodeToString(b)
}
