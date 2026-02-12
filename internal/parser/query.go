package parser

import (
	"strings"
)

// ParsedQuery represents a minimally parsed GraphQL query.
type ParsedQuery struct {
	OperationType string            // "query", "mutation", "subscription"
	OperationName string            // name of the operation, if named
	Fields        []ParsedSelection // top-level selections
	Variables     []ParsedVariable  // declared variables
}

// ParsedSelection represents a field selection in a query.
type ParsedSelection struct {
	Name      string
	Alias     string
	Arguments []ParsedArgument
	Children  []ParsedSelection
}

// ParsedVariable represents a declared variable.
type ParsedVariable struct {
	Name string
	Type string
}

// ParsedArgument represents a field argument in a query.
type ParsedArgument struct {
	Name  string
	Value string
}

// ParseQuery does a minimal parse of a GraphQL query string to extract structure.
// This is NOT a full spec-compliant parser — it handles the common cases needed
// for traffic analysis and schema reconstruction.
func ParseQuery(query string) *ParsedQuery {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	p := &ParsedQuery{}
	tokens := tokenize(query)
	pos := 0

	// Parse operation type and name
	if pos < len(tokens) {
		switch tokens[pos] {
		case "query", "mutation", "subscription":
			p.OperationType = tokens[pos]
			pos++
			// Check for operation name
			if pos < len(tokens) && tokens[pos] != "(" && tokens[pos] != "{" {
				p.OperationName = tokens[pos]
				pos++
			}
			// Skip variable declarations
			if pos < len(tokens) && tokens[pos] == "(" {
				pos = skipParens(tokens, pos)
			}
		case "{":
			p.OperationType = "query" // anonymous query
		}
	}

	// Parse selection set
	if pos < len(tokens) && tokens[pos] == "{" {
		pos++
		p.Fields, pos = parseSelectionSet(tokens, pos)
	}

	return p
}

func parseSelectionSet(tokens []string, pos int) ([]ParsedSelection, int) {
	var selections []ParsedSelection

	for pos < len(tokens) && tokens[pos] != "}" {
		if tokens[pos] == "..." {
			// Inline fragment or fragment spread
			pos++
			if pos < len(tokens) && tokens[pos] == "on" {
				pos++ // skip "on"
				pos++ // skip type name
			}
			if pos < len(tokens) && tokens[pos] == "{" {
				pos++
				_, pos = parseSelectionSet(tokens, pos)
			}
			continue
		}

		sel := ParsedSelection{}

		// Check for alias: "alias: fieldName"
		if pos+2 < len(tokens) && tokens[pos+1] == ":" {
			sel.Alias = tokens[pos]
			pos += 2
		}

		if pos >= len(tokens) || tokens[pos] == "}" {
			break
		}

		sel.Name = tokens[pos]
		pos++

		// Parse arguments
		if pos < len(tokens) && tokens[pos] == "(" {
			pos++
			for pos < len(tokens) && tokens[pos] != ")" {
				arg := ParsedArgument{}
				arg.Name = tokens[pos]
				pos++
				if pos < len(tokens) && tokens[pos] == ":" {
					pos++
					if pos < len(tokens) {
						arg.Value = tokens[pos]
						pos++
					}
				}
				if pos < len(tokens) && tokens[pos] == "," {
					pos++
				}
				sel.Arguments = append(sel.Arguments, arg)
			}
			if pos < len(tokens) {
				pos++ // skip ")"
			}
		}

		// Parse nested selection set
		if pos < len(tokens) && tokens[pos] == "{" {
			pos++
			sel.Children, pos = parseSelectionSet(tokens, pos)
		}

		selections = append(selections, sel)
	}

	if pos < len(tokens) && tokens[pos] == "}" {
		pos++ // consume "}"
	}

	return selections, pos
}

func skipParens(tokens []string, pos int) int {
	if pos >= len(tokens) || tokens[pos] != "(" {
		return pos
	}
	depth := 0
	for pos < len(tokens) {
		if tokens[pos] == "(" {
			depth++
		} else if tokens[pos] == ")" {
			depth--
			if depth == 0 {
				pos++
				break
			}
		}
		pos++
	}
	return pos
}

func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '{' || ch == '}' || ch == '(' || ch == ')' || ch == ':' || ch == ',' || ch == '!' || ch == '$' || ch == '@':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(ch))
		case ch == '.' && i+2 < len(s) && s[i+1] == '.' && s[i+2] == '.':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, "...")
			i += 2
		case ch == '"':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			// Read string literal
			i++
			var str strings.Builder
			str.WriteByte('"')
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					str.WriteByte(s[i])
					i++
					str.WriteByte(s[i])
				} else {
					str.WriteByte(s[i])
				}
				i++
			}
			str.WriteByte('"')
			tokens = append(tokens, str.String())
		case ch == '#':
			// Skip comment to end of line
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
