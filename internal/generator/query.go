package generator

import (
	"fmt"
	"strings"

	"github.com/0xdtc/graphscope/internal/schema"
)

// Config controls query generation behavior.
type Config struct {
	MaxDepth          int  // Maximum nesting depth (default 3)
	IncludeDeprecated bool // Include deprecated fields
	IncludeArgs       bool // Include arguments with placeholders
}

// DefaultConfig returns the default generation config.
func DefaultConfig() Config {
	return Config{
		MaxDepth:          3,
		IncludeDeprecated: false,
		IncludeArgs:       true,
	}
}

// GenerateQuery builds a complete GraphQL query string for an operation.
func GenerateQuery(s *schema.Schema, opName string, opKind string, cfg Config) (string, map[string]any) {
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 3
	}

	// Find the root type for this operation kind
	var rootTypeName string
	switch opKind {
	case "query":
		rootTypeName = s.QueryType
	case "mutation":
		rootTypeName = s.MutationType
	case "subscription":
		rootTypeName = s.SubscriptionType
	}

	rootType := schema.FindType(s, rootTypeName)
	if rootType == nil {
		return "", nil
	}

	// Find the specific field (operation)
	var field *schema.Field
	for i := range rootType.Fields {
		if rootType.Fields[i].Name == opName {
			field = &rootType.Fields[i]
			break
		}
	}
	if field == nil {
		return "", nil
	}

	// Build type index for resolving references
	typeIndex := make(map[string]*schema.Type)
	for i := range s.Types {
		typeIndex[s.Types[i].Name] = &s.Types[i]
	}

	var b strings.Builder
	variables := make(map[string]any)
	varDefs := buildVarDefs(field.Args, variables)

	// Write operation header
	if varDefs != "" {
		b.WriteString(fmt.Sprintf("%s %s(%s) {\n", opKind, opName, varDefs))
	} else {
		b.WriteString(fmt.Sprintf("%s %s {\n", opKind, opName))
	}

	// Write operation field with arguments
	argStr := buildArgString(field.Args)
	if argStr != "" {
		b.WriteString(fmt.Sprintf("  %s(%s)", opName, argStr))
	} else {
		b.WriteString(fmt.Sprintf("  %s", opName))
	}

	// Expand return type
	returnTypeName := field.Type.BaseName()
	returnType := typeIndex[returnTypeName]

	if returnType != nil && (returnType.Kind == schema.KindObject || returnType.Kind == schema.KindInterface) {
		b.WriteString(" {\n")
		expandFields(&b, returnType, typeIndex, cfg, 2, make(map[string]bool))
		b.WriteString("  }")
	}

	b.WriteString("\n}\n")

	return b.String(), variables
}

// expandFields recursively writes field selections.
func expandFields(b *strings.Builder, t *schema.Type, typeIndex map[string]*schema.Type, cfg Config, depth int, visited map[string]bool) {
	if depth > cfg.MaxDepth+1 {
		return
	}

	// Prevent infinite recursion on circular types
	if visited[t.Name] {
		return
	}
	visited[t.Name] = true
	defer func() { delete(visited, t.Name) }()

	indent := strings.Repeat("    ", depth)

	fields := t.Fields
	if t.Kind == schema.KindInputObject {
		fields = t.InputFields
	}

	for _, f := range fields {
		if f.IsDeprecated && !cfg.IncludeDeprecated {
			continue
		}

		baseName := f.Type.BaseName()
		target := typeIndex[baseName]

		if target == nil || target.Kind == schema.KindScalar || target.Kind == schema.KindEnum {
			b.WriteString(fmt.Sprintf("%s%s\n", indent, f.Name))
			continue
		}

		if target.Kind == schema.KindUnion || target.Kind == schema.KindInterface {
			b.WriteString(fmt.Sprintf("%s%s {\n", indent, f.Name))
			// Write inline fragments for each possible type
			for _, ptName := range target.PossibleTypes {
				pt := typeIndex[ptName]
				if pt == nil {
					continue
				}
				b.WriteString(fmt.Sprintf("%s    ... on %s {\n", indent, ptName))
				expandFields(b, pt, typeIndex, cfg, depth+2, visited)
				b.WriteString(fmt.Sprintf("%s    }\n", indent))
			}
			b.WriteString(fmt.Sprintf("%s}\n", indent))
			continue
		}

		if depth < cfg.MaxDepth+1 {
			b.WriteString(fmt.Sprintf("%s%s {\n", indent, f.Name))
			expandFields(b, target, typeIndex, cfg, depth+1, visited)
			b.WriteString(fmt.Sprintf("%s}\n", indent))
		}
	}
}

// buildVarDefs creates the variable definition string for the operation header.
func buildVarDefs(args []schema.Argument, variables map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for _, arg := range args {
		varName := "$" + arg.Name
		typeSig := arg.Type.Signature()
		parts = append(parts, fmt.Sprintf("%s: %s", varName, typeSig))
		variables[arg.Name] = exampleValue(arg)
	}
	return strings.Join(parts, ", ")
}

// buildArgString creates the argument pass-through string using variable references.
func buildArgString(args []schema.Argument) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for _, arg := range args {
		parts = append(parts, fmt.Sprintf("%s: $%s", arg.Name, arg.Name))
	}
	return strings.Join(parts, ", ")
}
