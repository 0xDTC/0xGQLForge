// Package inference builds a synthetic GraphQL schema from captured proxy traffic.
package inference

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/0xDTC/0xGQLForge/internal/schema"
)

var (
	// matches: query/mutation/subscription OptionalName
	opDeclRe = regexp.MustCompile(`(?i)^\s*(query|mutation|subscription)\b\s*(\w+)?`)
	// matches variable declarations: $name: Type
	varDeclRe = regexp.MustCompile(`\$(\w+)\s*:\s*([\w!\[\] ]+)`)
	// matches top-level field lines (indented 2–4 spaces or 1 tab inside an op)
	topFieldRe = regexp.MustCompile(`(?m)^(?:  |\t)([a-zA-Z_]\w*)\s*[\({]`)
)

// BuildFromTraffic synthesises a Schema from a set of captured requests.
// It extracts operation names, types, and top-level fields to reconstruct
// Query/Mutation/Subscription root types.
func BuildFromTraffic(reqs []schema.CapturedRequest, projectName string) *schema.Schema {
	queryFields := map[string]schema.Field{}
	mutFields := map[string]schema.Field{}
	subFields := map[string]schema.Field{}

	for _, req := range reqs {
		if req.Query == "" {
			continue
		}
		opKind, fields := parseQuery(req.Query)

		// If we already know the operation name from the captured request, add it.
		if req.OperationName != "" {
			f := schema.Field{
				Name: req.OperationName,
				Type: unknownRef(),
			}
			switch opKind {
			case "mutation":
				if _, ok := mutFields[f.Name]; !ok {
					mutFields[f.Name] = f
				}
			case "subscription":
				if _, ok := subFields[f.Name]; !ok {
					subFields[f.Name] = f
				}
			default:
				if _, ok := queryFields[f.Name]; !ok {
					queryFields[f.Name] = f
				}
			}
		}

		// Add top-level fields extracted from the query body.
		for _, f := range fields {
			switch opKind {
			case "mutation":
				if _, ok := mutFields[f.Name]; !ok {
					mutFields[f.Name] = f
				}
			case "subscription":
				if _, ok := subFields[f.Name]; !ok {
					subFields[f.Name] = f
				}
			default:
				if _, ok := queryFields[f.Name]; !ok {
					queryFields[f.Name] = f
				}
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

	// Build Query root type.
	qt := schema.Type{Name: "Query", Kind: schema.KindObject}
	for _, f := range queryFields {
		qt.Fields = append(qt.Fields, f)
	}
	if len(qt.Fields) == 0 {
		qt.Fields = append(qt.Fields, schema.Field{
			Name: "_placeholder", Type: unknownRef(),
			Description: "No query operations captured yet",
		})
	}
	s.Types = append(s.Types, qt)

	// Build Mutation root type only if mutations were seen.
	if len(mutFields) > 0 {
		mt := schema.Type{Name: "Mutation", Kind: schema.KindObject}
		for _, f := range mutFields {
			mt.Fields = append(mt.Fields, f)
		}
		s.Types = append(s.Types, mt)
		s.MutationType = "Mutation"
	}

	// Build Subscription root type only if subscriptions were seen.
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

// parseQuery extracts the operation kind and top-level field names from a raw GraphQL query.
func parseQuery(query string) (kind string, fields []schema.Field) {
	kind = "query"

	// Find the operation declaration on any line.
	for _, line := range strings.Split(query, "\n") {
		m := opDeclRe.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil {
			kind = strings.ToLower(m[1])
			break
		}
	}

	// Collect variable type hints ($varName: TypeName).
	varTypes := map[string]string{}
	for _, m := range varDeclRe.FindAllStringSubmatch(query, -1) {
		varTypes[m[1]] = strings.TrimSpace(m[2])
	}

	// Extract top-level selection fields (indented exactly 2–4 spaces or 1 tab).
	skipWords := map[string]bool{
		"query": true, "mutation": true, "subscription": true,
		"fragment": true, "on": true, "true": true, "false": true,
	}
	seen := map[string]bool{}
	for _, m := range topFieldRe.FindAllStringSubmatch(query, -1) {
		name := m[1]
		if skipWords[name] || seen[name] {
			continue
		}
		seen[name] = true
		fields = append(fields, schema.Field{Name: name, Type: unknownRef()})
	}

	return kind, fields
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
