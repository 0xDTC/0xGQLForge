package analysis

import (
	"time"

	"github.com/0xDTC/0xGQLForge/internal/schema"
)

// DiffSchemas compares two schemas and returns the differences.
func DiffSchemas(a, b *schema.Schema, diffID string) schema.SchemaDiff {
	diff := schema.SchemaDiff{
		ID:        diffID,
		SchemaAID: a.ID,
		SchemaBID: b.ID,
		CreatedAt: time.Now().UTC(),
	}

	aTypes := typeMap(a)
	bTypes := typeMap(b)

	// Find added and removed types
	for name := range bTypes {
		if _, exists := aTypes[name]; !exists {
			diff.Added.Types = append(diff.Added.Types, name)
		}
	}
	for name := range aTypes {
		if _, exists := bTypes[name]; !exists {
			diff.Removed.Types = append(diff.Removed.Types, name)
		}
	}

	// Find changed fields in types that exist in both
	for name, aType := range aTypes {
		bType, exists := bTypes[name]
		if !exists {
			continue
		}

		aFields := fieldMap(aType)
		bFields := fieldMap(bType)

		for fname := range bFields {
			if _, exists := aFields[fname]; !exists {
				diff.Added.Fields = append(diff.Added.Fields, name+"."+fname)
			}
		}
		for fname := range aFields {
			if _, exists := bFields[fname]; !exists {
				diff.Removed.Fields = append(diff.Removed.Fields, name+"."+fname)
			}
		}

		// Check for type changes on existing fields
		for fname, aField := range aFields {
			bField, exists := bFields[fname]
			if !exists {
				continue
			}
			oldSig := aField.Type.Signature()
			newSig := bField.Type.Signature()
			if oldSig != newSig {
				diff.Changed = append(diff.Changed, schema.FieldChange{
					Path:     name + "." + fname,
					OldType:  oldSig,
					NewType:  newSig,
					Breaking: isBreakingChange(aField.Type, bField.Type),
				})
			}
		}
	}

	return diff
}

func typeMap(s *schema.Schema) map[string]*schema.Type {
	m := make(map[string]*schema.Type)
	for i := range s.Types {
		t := &s.Types[i]
		if len(t.Name) > 2 && t.Name[:2] == "__" {
			continue
		}
		m[t.Name] = t
	}
	return m
}

func fieldMap(t *schema.Type) map[string]*schema.Field {
	m := make(map[string]*schema.Field)
	fields := t.Fields
	if t.Kind == schema.KindInputObject {
		fields = t.InputFields
	}
	for i := range fields {
		m[fields[i].Name] = &fields[i]
	}
	return m
}

// isBreakingChange determines if a type change is breaking.
// Making a field nullable or changing its base type is breaking.
func isBreakingChange(old, new schema.TypeRef) bool {
	// Removing non-null wrapper is not breaking (widening)
	// Adding non-null wrapper IS breaking (narrowing for inputs)
	if !old.IsNonNull() && new.IsNonNull() {
		return true
	}
	// Changing base type is always breaking
	if old.BaseName() != new.BaseName() {
		return true
	}
	return false
}
