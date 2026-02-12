package generator

import (
	"github.com/0xdtc/graphscope/internal/schema"
)

// AnalyzeDepth calculates the maximum possible query depth for each operation.
func AnalyzeDepth(s *schema.Schema) []schema.DepthResult {
	typeIndex := make(map[string]*schema.Type)
	for i := range s.Types {
		typeIndex[s.Types[i].Name] = &s.Types[i]
	}

	var results []schema.DepthResult
	ops := schema.GetOperations(s)

	for _, op := range ops {
		visited := make(map[string]bool)
		maxDepth := 0
		var deepestPaths []string

		baseName := op.ReturnType.BaseName()
		measureDepth(typeIndex, baseName, "", 1, visited, &maxDepth, &deepestPaths)

		results = append(results, schema.DepthResult{
			OperationName: op.Name,
			MaxDepth:      maxDepth,
			FieldPaths:    deepestPaths,
		})
	}

	return results
}

// measureDepth recursively measures field depth, tracking the deepest paths found.
func measureDepth(typeIndex map[string]*schema.Type, typeName, path string, depth int, visited map[string]bool, maxDepth *int, deepPaths *[]string) {
	if visited[typeName] {
		return
	}

	t := typeIndex[typeName]
	if t == nil || t.Kind == schema.KindScalar || t.Kind == schema.KindEnum {
		if depth > *maxDepth {
			*maxDepth = depth
			*deepPaths = []string{path}
		} else if depth == *maxDepth && path != "" {
			*deepPaths = append(*deepPaths, path)
		}
		return
	}

	visited[typeName] = true
	defer func() { delete(visited, typeName) }()

	fields := t.Fields
	if t.Kind == schema.KindInputObject {
		fields = t.InputFields
	}

	if len(fields) == 0 {
		if depth > *maxDepth {
			*maxDepth = depth
			*deepPaths = []string{path}
		}
		return
	}

	for _, f := range fields {
		fieldPath := path
		if fieldPath != "" {
			fieldPath += "." + f.Name
		} else {
			fieldPath = f.Name
		}

		baseName := f.Type.BaseName()
		measureDepth(typeIndex, baseName, fieldPath, depth+1, visited, maxDepth, deepPaths)
	}
}

// EstimateComplexity calculates a complexity score for an operation.
func EstimateComplexity(s *schema.Schema, opName string) schema.ComplexityResult {
	typeIndex := make(map[string]*schema.Type)
	for i := range s.Types {
		typeIndex[s.Types[i].Name] = &s.Types[i]
	}

	ops := schema.GetOperations(s)
	var op *schema.Operation
	for i := range ops {
		if ops[i].Name == opName {
			op = &ops[i]
			break
		}
	}
	if op == nil {
		return schema.ComplexityResult{OperationName: opName, Risk: "unknown"}
	}

	visited := make(map[string]bool)
	fieldCount := 0
	maxListNesting := 0

	countComplexity(typeIndex, op.ReturnType.BaseName(), visited, 1, 0, &fieldCount, &maxListNesting)

	score := float64(fieldCount)
	// List nesting multiplies complexity exponentially
	for i := 0; i < maxListNesting; i++ {
		score *= 10
	}

	risk := "low"
	switch {
	case score > 10000:
		risk = "critical"
	case score > 1000:
		risk = "high"
	case score > 100:
		risk = "medium"
	}

	return schema.ComplexityResult{
		OperationName: opName,
		Score:         score,
		FieldCount:    fieldCount,
		ListNesting:   maxListNesting,
		Risk:          risk,
	}
}

func countComplexity(typeIndex map[string]*schema.Type, typeName string, visited map[string]bool, depth, listDepth int, fieldCount, maxListNesting *int) {
	if depth > 10 || visited[typeName] {
		return
	}

	t := typeIndex[typeName]
	if t == nil || t.Kind == schema.KindScalar || t.Kind == schema.KindEnum {
		*fieldCount++
		return
	}

	visited[typeName] = true
	defer func() { delete(visited, typeName) }()

	fields := t.Fields
	if t.Kind == schema.KindInputObject {
		fields = t.InputFields
	}

	for _, f := range fields {
		*fieldCount++
		ld := listDepth
		if f.Type.IsList() {
			ld++
			if ld > *maxListNesting {
				*maxListNesting = ld
			}
		}
		countComplexity(typeIndex, f.Type.BaseName(), visited, depth+1, ld, fieldCount, maxListNesting)
	}
}
