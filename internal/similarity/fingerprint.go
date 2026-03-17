package similarity

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// Fingerprint generates a structural hash of a GraphQL query string.
// It normalizes the query by:
// - Removing variable values and aliases
// - Sorting field names alphabetically
// - Hashing the resulting structure
func Fingerprint(query string) string {
	normalized := normalizeQuery(query)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:16]) // 32-char hex fingerprint
}

// normalizeQuery strips variable values and normalizes field ordering.
func normalizeQuery(query string) string {
	query = strings.TrimSpace(query)

	// Remove comments
	var lines []string
	for _, line := range strings.Split(query, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			lines = append(lines, trimmed)
		}
	}
	query = strings.Join(lines, " ")

	// Remove string literals (variable values in inline queries)
	query = removeStringLiterals(query)

	// Remove numeric literals
	query = removeNumericLiterals(query)

	// Remove aliases (word followed by colon before field name)
	query = removeAliases(query)

	// Normalize whitespace
	query = normalizeWhitespace(query)

	// Sort fields within each selection set
	query = sortSelections(query)

	return query
}

func removeStringLiterals(s string) string {
	var result strings.Builder
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		if s[i] == '\\' && inString {
			escaped = true
			continue
		}
		if s[i] == '"' {
			inString = !inString
			if !inString {
				result.WriteString(`""`)
			}
			continue
		}
		if !inString {
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

func removeNumericLiterals(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] >= '0' && s[i] <= '9' {
			// Skip the entire number
			for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
				i++
			}
			result.WriteString("0")
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

func removeAliases(s string) string {
	// Simple alias removal: "aliasName: fieldName" → "fieldName"
	var result strings.Builder
	tokens := tokenize(s)
	for i := 0; i < len(tokens); i++ {
		if i+2 < len(tokens) && tokens[i+1] == ":" && isIdentifier(tokens[i]) && isIdentifier(tokens[i+2]) {
			// Skip the alias and colon
			i += 1 // skip colon, next iteration picks up field name
			continue
		}
		result.WriteString(tokens[i])
		if i < len(tokens)-1 {
			result.WriteByte(' ')
		}
	}
	return result.String()
}

func normalizeWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// sortSelections sorts field names within curly brace blocks for consistent fingerprinting.
// Uses a stack to handle nested selection sets correctly.
func sortSelections(s string) string {
	var result strings.Builder
	// Stack of field lists, one per nesting depth.
	var stack [][]string
	var currentField strings.Builder

	flushField := func() {
		if currentField.Len() > 0 {
			field := strings.TrimSpace(currentField.String())
			if field != "" && len(stack) > 0 {
				stack[len(stack)-1] = append(stack[len(stack)-1], field)
			} else if field != "" {
				// Outside any braces — write directly
				result.WriteString(field)
			}
			currentField.Reset()
		}
	}

	for _, ch := range s {
		switch ch {
		case '{':
			flushField()
			result.WriteRune('{')
			stack = append(stack, []string{})
		case '}':
			flushField()
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				sort.Strings(top)
				result.WriteString(strings.Join(top, " "))
			}
			result.WriteRune('}')
		case ' ':
			if len(stack) > 0 {
				flushField()
			} else {
				currentField.WriteRune(ch)
			}
		default:
			currentField.WriteRune(ch)
		}
	}

	flushField()
	return result.String()
}

func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder

	for _, ch := range s {
		switch {
		case ch == '{' || ch == '}' || ch == '(' || ch == ')' || ch == ':' || ch == ',':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(ch))
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func isIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, ch := range s {
		if i == 0 {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_') {
				return false
			}
		} else {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
				return false
			}
		}
	}
	return true
}
