package wordlist

import (
	_ "embed"
	"strings"
)

//go:embed common_fields.txt
var commonFieldsRaw string

// CommonFields returns the built-in field wordlist for fuzzing.
func CommonFields() []string {
	var fields []string
	for _, line := range strings.Split(commonFieldsRaw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			fields = append(fields, line)
		}
	}
	return fields
}
