package handler

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/0xdtc/graphscope/internal/parser"
	"github.com/0xdtc/graphscope/internal/schema"
)

// IntrospectionParse handles POST /api/introspection — parses pasted introspection JSON.
func (h *Handlers) IntrospectionParse(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "failed to read body: "+err.Error())
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		name = fmt.Sprintf("schema_%s", time.Now().Format("20060102_150405"))
	}

	id := generateID()
	parsed, err := parser.ParseIntrospection(body, id, name)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "parse error: "+err.Error())
		return
	}

	if err := h.SchemaRepo.Save(parsed, string(body)); err != nil {
		jsonErr(w, http.StatusInternalServerError, "save error: "+err.Error())
		return
	}

	ops := schema.GetOperations(parsed)
	userTypes := schema.UserTypes(parsed)

	jsonResp(w, http.StatusOK, map[string]any{
		"id":            parsed.ID,
		"name":          parsed.Name,
		"typeCount":     len(userTypes),
		"queryCount":    countOps(ops, "query"),
		"mutationCount": countOps(ops, "mutation"),
		"subCount":      countOps(ops, "subscription"),
		"redirectURL":   "/schema/" + parsed.ID,
	})
}

func countOps(ops []schema.Operation, kind string) int {
	n := 0
	for _, op := range ops {
		if op.Kind == kind {
			n++
		}
	}
	return n
}
