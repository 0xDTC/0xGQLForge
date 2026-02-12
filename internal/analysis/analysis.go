package analysis

import (
	"github.com/0xDTC/0xGQLForge/internal/generator"
	"github.com/0xDTC/0xGQLForge/internal/schema"
)

// RunAll executes all security analysis modules on a schema and returns a map of results.
func RunAll(s *schema.Schema) map[string]any {
	results := make(map[string]any)

	results["depth"] = generator.AnalyzeDepth(s)
	results["complexity"] = analyzeComplexity(s)
	results["mutations"] = DetectDangerousMutations(s)
	results["idor"] = DetectIDOR(s)
	results["authz"] = AnalyzeAuthPatterns(s)

	return results
}

// analyzeComplexity runs complexity estimation for all operations.
func analyzeComplexity(s *schema.Schema) []schema.ComplexityResult {
	ops := schema.GetOperations(s)
	var results []schema.ComplexityResult
	for _, op := range ops {
		results = append(results, generator.EstimateComplexity(s, op.Name))
	}
	return results
}
