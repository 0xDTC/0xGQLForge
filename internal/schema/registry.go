package schema

import (
	"sync"
)

// Registry provides in-memory schema access with thread-safe caching.
type Registry struct {
	mu      sync.RWMutex
	schemas map[string]*Schema
}

// NewRegistry creates a new schema registry.
func NewRegistry() *Registry {
	return &Registry{
		schemas: make(map[string]*Schema),
	}
}

// Put stores a schema in the in-memory cache.
func (r *Registry) Put(s *Schema) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.schemas[s.ID] = s
}

// Get retrieves a schema from cache by ID.
func (r *Registry) Get(id string) (*Schema, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.schemas[id]
	return s, ok
}

// Delete removes a schema from cache.
func (r *Registry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.schemas, id)
}

// All returns all cached schemas.
func (r *Registry) All() []*Schema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Schema, 0, len(r.schemas))
	for _, s := range r.schemas {
		result = append(result, s)
	}
	return result
}

// FindType locates a type by name within a schema.
func FindType(s *Schema, name string) *Type {
	for i := range s.Types {
		if s.Types[i].Name == name {
			return &s.Types[i]
		}
	}
	return nil
}

// GetOperations extracts all query, mutation, and subscription operations from a schema.
func GetOperations(s *Schema) []Operation {
	var ops []Operation

	if s.QueryType != "" {
		if t := FindType(s, s.QueryType); t != nil {
			for _, f := range t.Fields {
				ops = append(ops, Operation{
					Name:        f.Name,
					Kind:        "query",
					Description: f.Description,
					Args:        f.Args,
					ReturnType:  f.Type,
				})
			}
		}
	}

	if s.MutationType != "" {
		if t := FindType(s, s.MutationType); t != nil {
			for _, f := range t.Fields {
				ops = append(ops, Operation{
					Name:        f.Name,
					Kind:        "mutation",
					Description: f.Description,
					Args:        f.Args,
					ReturnType:  f.Type,
				})
			}
		}
	}

	if s.SubscriptionType != "" {
		if t := FindType(s, s.SubscriptionType); t != nil {
			for _, f := range t.Fields {
				ops = append(ops, Operation{
					Name:        f.Name,
					Kind:        "subscription",
					Description: f.Description,
					Args:        f.Args,
					ReturnType:  f.Type,
				})
			}
		}
	}

	return ops
}

// UserTypes returns all non-internal types (excludes __-prefixed types and built-in scalars).
func UserTypes(s *Schema) []Type {
	builtinScalars := map[string]bool{
		"String": true, "Int": true, "Float": true, "Boolean": true, "ID": true,
	}
	var types []Type
	for _, t := range s.Types {
		if len(t.Name) > 2 && t.Name[:2] == "__" {
			continue
		}
		if t.Kind == KindScalar && builtinScalars[t.Name] {
			continue
		}
		types = append(types, t)
	}
	return types
}
