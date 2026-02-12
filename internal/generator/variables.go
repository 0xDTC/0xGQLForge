package generator

import (
	"github.com/0xdtc/graphscope/internal/schema"
)

// exampleValue generates a placeholder value for an argument based on its type.
func exampleValue(arg schema.Argument) any {
	if arg.DefaultValue != nil {
		return *arg.DefaultValue
	}
	return exampleForTypeRef(arg.Type, arg.Name)
}

// exampleForTypeRef generates example values based on the type reference.
func exampleForTypeRef(ref schema.TypeRef, hint string) any {
	switch ref.Kind {
	case schema.KindNonNull:
		if ref.OfType != nil {
			return exampleForTypeRef(*ref.OfType, hint)
		}
		return nil
	case schema.KindList:
		if ref.OfType != nil {
			return []any{exampleForTypeRef(*ref.OfType, hint)}
		}
		return []any{}
	default:
		name := ""
		if ref.Name != nil {
			name = *ref.Name
		}
		return exampleForScalar(name, hint)
	}
}

// exampleForScalar returns a sensible example value for a scalar or named type.
func exampleForScalar(typeName, fieldHint string) any {
	switch typeName {
	case "String":
		return exampleString(fieldHint)
	case "Int":
		return exampleInt(fieldHint)
	case "Float":
		return 1.0
	case "Boolean":
		return true
	case "ID":
		return exampleID(fieldHint)
	default:
		// For custom scalars or input objects, return a descriptive placeholder
		return "<" + typeName + ">"
	}
}

// exampleString returns context-aware string examples.
func exampleString(hint string) string {
	switch {
	case contains(hint, "email"):
		return "user@example.com"
	case contains(hint, "name"):
		return "example_name"
	case contains(hint, "password", "pass"):
		return "P@ssw0rd123"
	case contains(hint, "token"):
		return "eyJhbGciOiJIUzI1NiJ9.example"
	case contains(hint, "url", "uri", "link"):
		return "https://example.com"
	case contains(hint, "phone"):
		return "+1234567890"
	case contains(hint, "address"):
		return "123 Main St"
	case contains(hint, "query", "search", "filter"):
		return "test"
	case contains(hint, "cursor", "after", "before"):
		return "cursor_abc123"
	default:
		return "example_value"
	}
}

// exampleInt returns context-aware integer examples.
func exampleInt(hint string) int {
	switch {
	case contains(hint, "limit", "count", "size", "per"):
		return 10
	case contains(hint, "offset", "skip", "page"):
		return 0
	case contains(hint, "id"):
		return 1
	case contains(hint, "age"):
		return 25
	case contains(hint, "year"):
		return 2025
	default:
		return 1
	}
}

// exampleID returns context-aware ID examples.
func exampleID(hint string) string {
	switch {
	case contains(hint, "user"):
		return "user_123"
	case contains(hint, "order"):
		return "order_456"
	case contains(hint, "product"):
		return "prod_789"
	default:
		return "id_123"
	}
}

// contains checks if hint contains any of the keywords (case-insensitive substring).
func contains(hint string, keywords ...string) bool {
	lower := toLower(hint)
	for _, kw := range keywords {
		if idx := indexLower(lower, kw); idx >= 0 {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		} else {
			b[i] = c
		}
	}
	return string(b)
}

func indexLower(s, sub string) int {
	if len(sub) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
