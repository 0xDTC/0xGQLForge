package parser

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/0xDTC/0xGQLForge/internal/schema"
)

// ReconstructSchema attempts to infer a schema from captured GraphQL traffic.
// This is used when introspection is disabled on the target.
func ReconstructSchema(requests []schema.CapturedRequest, id, name string) *schema.Schema {
	s := &schema.Schema{
		ID:        id,
		Name:      name,
		Source:    schema.SourceReconstruction,
		QueryType: "Query",
		CreatedAt: time.Now().UTC(),
	}

	typeFields := make(map[string]map[string]inferredField) // typeName -> fieldName -> field info
	typeFields["Query"] = make(map[string]inferredField)

	hasMutation := false
	hasSubscription := false

	for _, req := range requests {
		parsed := ParseQuery(req.Query)
		if parsed == nil {
			continue
		}

		rootType := "Query"
		switch parsed.OperationType {
		case "mutation":
			rootType = "Mutation"
			hasMutation = true
			if _, ok := typeFields[rootType]; !ok {
				typeFields[rootType] = make(map[string]inferredField)
			}
		case "subscription":
			rootType = "Subscription"
			hasSubscription = true
			if _, ok := typeFields[rootType]; !ok {
				typeFields[rootType] = make(map[string]inferredField)
			}
		}

		// Infer fields from query structure
		inferFromSelections(typeFields, rootType, parsed.Fields)

		// Infer return types from response body
		if len(req.ResponseBody) > 0 {
			inferFromResponse(typeFields, rootType, parsed.Fields, req.ResponseBody)
		}
	}

	if hasMutation {
		s.MutationType = "Mutation"
	}
	if hasSubscription {
		s.SubscriptionType = "Subscription"
	}

	// Convert inferred type map to schema types
	for typeName, fields := range typeFields {
		t := schema.Type{
			Name: typeName,
			Kind: schema.KindObject,
		}
		for fieldName, info := range fields {
			f := schema.Field{
				Name: fieldName,
				Type: info.typeRef,
				Args: info.args,
			}
			t.Fields = append(t.Fields, f)
		}
		s.Types = append(s.Types, t)
	}

	return s
}

type inferredField struct {
	typeRef schema.TypeRef
	args    []schema.Argument
}

func inferFromSelections(typeFields map[string]map[string]inferredField, parentType string, selections []ParsedSelection) {
	for _, sel := range selections {
		if sel.Name == "" || strings.HasPrefix(sel.Name, "__") {
			continue
		}

		if _, ok := typeFields[parentType]; !ok {
			typeFields[parentType] = make(map[string]inferredField)
		}

		field := typeFields[parentType][sel.Name]

		// Infer arguments
		for _, arg := range sel.Arguments {
			if arg.Name == "" || arg.Name == "$" {
				continue
			}
			argName := strings.TrimPrefix(arg.Name, "$")
			found := false
			for _, existing := range field.args {
				if existing.Name == argName {
					found = true
					break
				}
			}
			if !found {
				field.args = append(field.args, schema.Argument{
					Name: argName,
					Type: inferArgType(arg.Value),
				})
			}
		}

		// If field has children, it returns an object type
		if len(sel.Children) > 0 {
			childTypeName := capitalize(sel.Name)
			nameStr := childTypeName
			field.typeRef = schema.TypeRef{
				Kind: schema.KindObject,
				Name: &nameStr,
			}

			// Recurse into children
			inferFromSelections(typeFields, childTypeName, sel.Children)
		} else if field.typeRef.Name == nil {
			// Leaf field — assume scalar
			scalarName := "String"
			field.typeRef = schema.TypeRef{
				Kind: schema.KindScalar,
				Name: &scalarName,
			}
		}

		typeFields[parentType][sel.Name] = field
	}
}

func inferFromResponse(typeFields map[string]map[string]inferredField, rootType string, selections []ParsedSelection, responseBody json.RawMessage) {
	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil || resp.Data == nil {
		return
	}

	for _, sel := range selections {
		data, ok := resp.Data[sel.Name]
		if !ok {
			continue
		}

		inferTypeFromJSON(typeFields, rootType, sel.Name, data)
	}
}

func inferTypeFromJSON(typeFields map[string]map[string]inferredField, parentType, fieldName string, data json.RawMessage) {
	if len(data) == 0 || string(data) == "null" {
		return
	}

	if _, ok := typeFields[parentType]; !ok {
		typeFields[parentType] = make(map[string]inferredField)
	}

	field := typeFields[parentType][fieldName]

	switch data[0] {
	case '{':
		// Object — infer fields from keys
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(data, &obj); err != nil {
			return
		}

		childType := capitalize(fieldName)
		nameStr := childType
		field.typeRef = schema.TypeRef{Kind: schema.KindObject, Name: &nameStr}

		if _, ok := typeFields[childType]; !ok {
			typeFields[childType] = make(map[string]inferredField)
		}

		for key, val := range obj {
			inferTypeFromJSON(typeFields, childType, key, val)
		}

	case '[':
		// Array — infer from first element
		var arr []json.RawMessage
		if err := json.Unmarshal(data, &arr); err != nil || len(arr) == 0 {
			return
		}
		// Mark as list
		innerField := typeFields[parentType][fieldName]
		inferTypeFromJSON(typeFields, parentType, fieldName, arr[0])
		updatedField := typeFields[parentType][fieldName]

		listRef := schema.TypeRef{
			Kind:   schema.KindList,
			OfType: &updatedField.typeRef,
		}
		innerField.typeRef = listRef
		typeFields[parentType][fieldName] = innerField
		return

	case '"':
		nameStr := "String"
		field.typeRef = schema.TypeRef{Kind: schema.KindScalar, Name: &nameStr}

	default:
		str := string(data)
		if str == "true" || str == "false" {
			nameStr := "Boolean"
			field.typeRef = schema.TypeRef{Kind: schema.KindScalar, Name: &nameStr}
		} else if strings.Contains(str, ".") {
			nameStr := "Float"
			field.typeRef = schema.TypeRef{Kind: schema.KindScalar, Name: &nameStr}
		} else {
			nameStr := "Int"
			field.typeRef = schema.TypeRef{Kind: schema.KindScalar, Name: &nameStr}
		}
	}

	typeFields[parentType][fieldName] = field
}

func inferArgType(value string) schema.TypeRef {
	value = strings.TrimSpace(value)
	switch {
	case value == "" || strings.HasPrefix(value, "$"):
		nameStr := "String"
		return schema.TypeRef{Kind: schema.KindScalar, Name: &nameStr}
	case value == "true" || value == "false":
		nameStr := "Boolean"
		return schema.TypeRef{Kind: schema.KindScalar, Name: &nameStr}
	case len(value) > 0 && value[0] >= '0' && value[0] <= '9':
		if strings.Contains(value, ".") {
			nameStr := "Float"
			return schema.TypeRef{Kind: schema.KindScalar, Name: &nameStr}
		}
		nameStr := "Int"
		return schema.TypeRef{Kind: schema.KindScalar, Name: &nameStr}
	default:
		nameStr := "String"
		return schema.TypeRef{Kind: schema.KindScalar, Name: &nameStr}
	}
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return fmt.Sprintf("%c%s", strings.ToUpper(s[:1])[0], s[1:])
}
