package analysis

import (
	"strings"

	"github.com/0xdtc/graphscope/internal/schema"
)

// idorArgPatterns are argument names that commonly indicate IDOR vulnerability potential.
var idorArgPatterns = []string{
	"id", "userid", "user_id", "orderid", "order_id",
	"accountid", "account_id", "profileid", "profile_id",
	"postid", "post_id", "commentid", "comment_id",
	"documentid", "document_id", "fileid", "file_id",
	"invoiceid", "invoice_id", "paymentid", "payment_id",
	"organizationid", "organization_id", "teamid", "team_id",
	"projectid", "project_id", "messageid", "message_id",
	"transactionid", "transaction_id", "customerid", "customer_id",
}

// DetectIDOR scans schema operations for potential IDOR vulnerabilities.
func DetectIDOR(s *schema.Schema) []schema.IDORCandidate {
	var results []schema.IDORCandidate

	ops := schema.GetOperations(s)
	for _, op := range ops {
		for _, arg := range op.Args {
			if isIDORCandidate(arg) {
				risk := "medium"
				if op.Kind == "mutation" {
					risk = "high"
				}
				pattern := detectIDPattern(arg)

				results = append(results, schema.IDORCandidate{
					FieldName: op.Name,
					ArgName:   arg.Name,
					Pattern:   pattern,
					Risk:      risk,
				})
			}
		}
	}

	// Also check fields on all object types that take ID arguments
	for _, t := range schema.UserTypes(s) {
		if t.Kind != schema.KindObject {
			continue
		}
		for _, f := range t.Fields {
			for _, arg := range f.Args {
				if isIDORCandidate(arg) {
					results = append(results, schema.IDORCandidate{
						FieldName: t.Name + "." + f.Name,
						ArgName:   arg.Name,
						Pattern:   detectIDPattern(arg),
						Risk:      "medium",
					})
				}
			}
		}
	}

	return results
}

func isIDORCandidate(arg schema.Argument) bool {
	baseName := arg.Type.BaseName()
	if baseName != "ID" && baseName != "Int" && baseName != "String" {
		return false
	}

	name := strings.ToLower(arg.Name)
	for _, pattern := range idorArgPatterns {
		if name == pattern {
			return true
		}
	}
	return false
}

func detectIDPattern(arg schema.Argument) string {
	baseName := arg.Type.BaseName()
	switch baseName {
	case "Int":
		return "sequential_int"
	case "ID":
		name := strings.ToLower(arg.Name)
		if strings.Contains(name, "uuid") {
			return "uuid"
		}
		return "opaque_id"
	default:
		return "string_id"
	}
}

// AnalyzeAuthPatterns examines the schema for common authorization-related patterns.
func AnalyzeAuthPatterns(s *schema.Schema) map[string]any {
	result := map[string]any{
		"hasAuth":        false,
		"authDirectives": []string{},
		"publicOps":      []string{},
		"sensitiveOps":   []string{},
	}

	// Check for auth-related directives
	var authDirs []string
	for _, d := range s.Directives {
		name := strings.ToLower(d.Name)
		if strings.Contains(name, "auth") || strings.Contains(name, "permission") ||
			strings.Contains(name, "role") || strings.Contains(name, "guard") {
			authDirs = append(authDirs, d.Name)
		}
	}
	if len(authDirs) > 0 {
		result["hasAuth"] = true
		result["authDirectives"] = authDirs
	}

	// Identify operations that look like they should require auth
	sensitiveKeywords := []string{"user", "account", "profile", "admin", "private", "internal", "secret"}
	ops := schema.GetOperations(s)

	var sensitiveOps []string
	for _, op := range ops {
		name := strings.ToLower(op.Name)
		for _, kw := range sensitiveKeywords {
			if strings.Contains(name, kw) {
				sensitiveOps = append(sensitiveOps, op.Name)
				break
			}
		}
	}
	result["sensitiveOps"] = sensitiveOps

	return result
}
