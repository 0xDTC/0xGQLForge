package analysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/0xdtc/graphscope/internal/schema"
)

// DefaultFieldWordlist is a built-in wordlist for field fuzzing.
var DefaultFieldWordlist = []string{
	// Common query fields
	"user", "users", "me", "viewer", "currentUser",
	"node", "nodes", "search", "find",
	"account", "accounts", "profile", "profiles",
	"post", "posts", "article", "articles",
	"comment", "comments", "message", "messages",
	"order", "orders", "product", "products",
	"payment", "payments", "invoice", "invoices",
	"notification", "notifications",
	"file", "files", "upload", "uploads",
	"setting", "settings", "config", "configuration",
	"role", "roles", "permission", "permissions",
	"team", "teams", "organization", "organizations",
	"project", "projects", "workspace", "workspaces",
	"event", "events", "log", "logs", "audit",
	"token", "tokens", "apiKey", "apiKeys",
	"webhook", "webhooks", "integration", "integrations",
	"analytics", "stats", "statistics", "metrics",
	"health", "status", "version", "info",
	"flag", "flags", "feature", "features",
	// Common mutation fields
	"createUser", "updateUser", "deleteUser",
	"login", "logout", "register", "signup",
	"resetPassword", "changePassword", "forgotPassword",
	"createPost", "updatePost", "deletePost",
	"createOrder", "cancelOrder",
	"updateSettings", "updateProfile",
	"uploadFile", "deleteFile",
	"sendMessage", "deleteMessage",
	"addComment", "deleteComment",
	"assignRole", "removeRole",
	"createToken", "revokeToken",
	"invite", "acceptInvite",
	// Admin / internal
	"admin", "internal", "debug", "test",
	"_service", "_entities", "__debug",
	"systemInfo", "serverInfo",
	"runMigration", "seed", "reset",
}

// FuzzFields sends queries with wordlist field names to discover valid fields.
func FuzzFields(targetURL string, typeName string, words []string) schema.FuzzResult {
	if len(words) == 0 {
		words = DefaultFieldWordlist
	}

	client := &http.Client{Timeout: 10 * time.Second}
	result := schema.FuzzResult{
		TypeName:     typeName,
		TestedFields: words,
	}

	for _, word := range words {
		query := fmt.Sprintf(`{ %s }`, word)
		payload, _ := json.Marshal(map[string]string{"query": query})

		req, err := http.NewRequest("POST", targetURL, bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()

		var gqlResp struct {
			Data   json.RawMessage `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(body, &gqlResp); err != nil {
			continue
		}

		// If data is present and not null, the field exists
		if gqlResp.Data != nil && string(gqlResp.Data) != "null" {
			result.ValidFields = append(result.ValidFields, word)
			continue
		}

		// Check error messages for "Did you mean" suggestions
		for _, e := range gqlResp.Errors {
			msg := strings.ToLower(e.Message)
			if strings.Contains(msg, "did you mean") {
				// Extract suggestions
				suggestions := extractSuggestions(e.Message)
				for _, s := range suggestions {
					result.Suggestions = append(result.Suggestions, s)
				}
			}
			// If the error is about authorization, the field likely exists
			if strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") ||
				strings.Contains(msg, "permission") || strings.Contains(msg, "not allowed") {
				result.ValidFields = append(result.ValidFields, word+" (auth required)")
			}
		}
	}

	// Deduplicate suggestions
	result.Suggestions = dedupe(result.Suggestions)

	return result
}

func extractSuggestions(message string) []string {
	// Look for quoted strings after "Did you mean"
	var suggestions []string
	parts := strings.Split(message, "\"")
	for i := 1; i < len(parts); i += 2 {
		if parts[i] != "" {
			suggestions = append(suggestions, parts[i])
		}
	}
	return suggestions
}

func dedupe(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
