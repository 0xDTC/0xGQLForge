package analysis

import (
	"strings"

	"github.com/0xdtc/graphscope/internal/schema"
)

// dangerousPatterns maps mutation name patterns to their risk descriptions.
var dangerousPatterns = []struct {
	keywords []string
	reason   string
	severity string
}{
	{[]string{"delete", "remove", "destroy", "purge"}, "Destructive operation — data deletion", "high"},
	{[]string{"drop", "truncate", "wipe", "clear"}, "Destructive operation — bulk data removal", "critical"},
	{[]string{"admin", "superuser", "root"}, "Administrative operation — privilege escalation risk", "critical"},
	{[]string{"update_role", "set_role", "assign_role", "change_role"}, "Role modification — privilege escalation risk", "critical"},
	{[]string{"grant", "revoke", "permission"}, "Permission modification — authorization bypass risk", "high"},
	{[]string{"reset_password", "change_password", "set_password", "forgot_password"}, "Password operation — account takeover risk", "high"},
	{[]string{"create_user", "register", "signup"}, "User creation — mass registration risk", "medium"},
	{[]string{"transfer", "send", "withdraw"}, "Financial operation — fund transfer risk", "high"},
	{[]string{"execute", "eval", "run_query", "raw"}, "Code execution — injection risk", "critical"},
	{[]string{"export", "download", "dump"}, "Data exfiltration — data leak risk", "medium"},
	{[]string{"disable", "deactivate", "suspend", "ban", "block"}, "Account control — denial of service risk", "high"},
	{[]string{"config", "setting", "configure"}, "Configuration change — system modification risk", "medium"},
	{[]string{"token", "api_key", "secret"}, "Secret management — credential exposure risk", "high"},
	{[]string{"upload", "import"}, "File upload — remote code execution risk", "medium"},
}

// DetectDangerousMutations scans all mutations for dangerous patterns.
func DetectDangerousMutations(s *schema.Schema) []schema.DangerousMutation {
	if s.MutationType == "" {
		return nil
	}

	mutationType := schema.FindType(s, s.MutationType)
	if mutationType == nil {
		return nil
	}

	var results []schema.DangerousMutation

	for _, field := range mutationType.Fields {
		name := strings.ToLower(field.Name)

		for _, pattern := range dangerousPatterns {
			var matched []string
			for _, kw := range pattern.keywords {
				if strings.Contains(name, kw) {
					matched = append(matched, kw)
				}
			}
			if len(matched) > 0 {
				results = append(results, schema.DangerousMutation{
					Name:       field.Name,
					Reason:     pattern.reason,
					Indicators: matched,
					Severity:   pattern.severity,
				})
				break // One match per mutation is enough
			}
		}

		// Check for mutations that accept ID-type args without auth context
		if hasBareIDArg(field) {
			results = append(results, schema.DangerousMutation{
				Name:       field.Name,
				Reason:     "Mutation accepts raw ID argument — potential IDOR",
				Indicators: []string{"id_argument"},
				Severity:   "medium",
			})
		}
	}

	return results
}

func hasBareIDArg(field schema.Field) bool {
	for _, arg := range field.Args {
		baseName := arg.Type.BaseName()
		if baseName == "ID" || baseName == "Int" {
			name := strings.ToLower(arg.Name)
			if name == "id" || strings.HasSuffix(name, "id") || strings.HasSuffix(name, "_id") {
				return true
			}
		}
	}
	return false
}
