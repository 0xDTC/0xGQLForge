package schema

import "strings"

// BuildGraphData constructs the nodes and edges for D3.js visualization.
func BuildGraphData(s *Schema) GraphData {
	var gd GraphData

	builtinScalars := map[string]bool{
		"String": true, "Int": true, "Float": true, "Boolean": true, "ID": true,
	}

	// Pre-compute which type names appear as graph nodes (for IsLink resolution).
	validTypeNames := make(map[string]bool)
	for _, t := range s.Types {
		if len(t.Name) > 2 && t.Name[:2] == "__" {
			continue
		}
		if t.Kind == KindScalar && builtinScalars[t.Name] {
			continue
		}
		validTypeNames[t.Name] = true
	}

	// Create nodes for non-internal types
	for _, t := range s.Types {
		if len(t.Name) > 2 && t.Name[:2] == "__" {
			continue
		}
		if t.Kind == KindScalar && builtinScalars[t.Name] {
			continue
		}

		node := GraphNode{
			ID:          t.Name,
			Kind:        t.Kind,
			Description: t.Description,
		}

		// Populate field rows for ERD display
		switch t.Kind {
		case KindObject, KindInterface:
			node.FieldCount = len(t.Fields)
			for _, f := range t.Fields {
				node.Fields = append(node.Fields, GraphField{
					Name:    f.Name,
					TypeSig: f.Type.Signature(),
					IsLink:  validTypeNames[f.Type.BaseName()],
				})
			}
		case KindInputObject:
			node.FieldCount = len(t.InputFields)
			for _, f := range t.InputFields {
				node.Fields = append(node.Fields, GraphField{
					Name:    f.Name,
					TypeSig: f.Type.Signature(),
					IsLink:  validTypeNames[f.Type.BaseName()],
				})
			}
		case KindEnum:
			node.FieldCount = len(t.EnumValues)
			for _, ev := range t.EnumValues {
				node.Fields = append(node.Fields, GraphField{Name: ev.Name})
			}
		}

		switch t.Name {
		case s.QueryType:
			node.IsRoot = true
			node.RootKind = "query"
		case s.MutationType:
			node.IsRoot = true
			node.RootKind = "mutation"
		case s.SubscriptionType:
			node.IsRoot = true
			node.RootKind = "subscription"
		}

		gd.Nodes = append(gd.Nodes, node)
	}

	// Build a set of valid non-internal type names for edge filtering
	validNodes := make(map[string]bool)
	for _, n := range gd.Nodes {
		validNodes[n.ID] = true
	}

	// Create edges from field type references
	seen := make(map[string]bool)
	for _, t := range s.Types {
		if len(t.Name) > 2 && t.Name[:2] == "__" {
			continue
		}

		fields := t.Fields
		if t.Kind == KindInputObject {
			fields = t.InputFields
		}

		for _, f := range fields {
			targetName := f.Type.BaseName()
			if targetName == "" || !validNodes[targetName] || !validNodes[t.Name] {
				continue
			}
			if t.Name == targetName {
				continue // skip self-references for cleaner graph
			}

			edgeKey := t.Name + "->" + targetName + "." + f.Name
			if seen[edgeKey] {
				continue
			}
			seen[edgeKey] = true

			gd.Links = append(gd.Links, GraphLink{
				Source:    t.Name,
				Target:    targetName,
				FieldName: f.Name,
				IsList:    f.Type.IsList(),
				IsNonNull: f.Type.IsNonNull(),
			})
		}
	}

	return gd
}

// BuildRelationships returns all typed relationships in the schema.
func BuildRelationships(s *Schema) []Relationship {
	var rels []Relationship
	for _, t := range s.Types {
		if len(t.Name) > 2 && t.Name[:2] == "__" {
			continue
		}
		fields := t.Fields
		if t.Kind == KindInputObject {
			fields = t.InputFields
		}
		for _, f := range fields {
			target := f.Type.BaseName()
			if target == "" {
				continue
			}
			rels = append(rels, Relationship{
				FromType:  t.Name,
				ToType:    target,
				FieldName: f.Name,
				IsList:    f.Type.IsList(),
				IsNonNull: f.Type.IsNonNull(),
			})
		}
	}
	return rels
}

// TypesByKind groups schema types by their kind.
func TypesByKind(s *Schema) map[TypeKind][]Type {
	result := make(map[TypeKind][]Type)
	for _, t := range UserTypes(s) {
		result[t.Kind] = append(result[t.Kind], t)
	}
	return result
}

// SearchTypes returns types whose name or description contains the query (case-insensitive).
func SearchTypes(s *Schema, query string) []Type {
	query = strings.ToLower(query)
	var results []Type
	for _, t := range UserTypes(s) {
		if strings.Contains(strings.ToLower(t.Name), query) ||
			strings.Contains(strings.ToLower(t.Description), query) {
			results = append(results, t)
		}
	}
	return results
}
