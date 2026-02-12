package parser

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/0xdtc/graphscope/internal/schema"
)

// introspectionResponse matches the standard GraphQL introspection response format.
type introspectionResponse struct {
	Data struct {
		Schema rawSchema `json:"__schema"`
	} `json:"data"`
}

// altIntrospectionResponse handles cases where __schema is at root level.
type altIntrospectionResponse struct {
	Schema rawSchema `json:"__schema"`
}

type rawSchema struct {
	QueryType        *rawNameRef    `json:"queryType"`
	MutationType     *rawNameRef    `json:"mutationType"`
	SubscriptionType *rawNameRef    `json:"subscriptionType"`
	Types            []rawType      `json:"types"`
	Directives       []rawDirective `json:"directives"`
}

type rawNameRef struct {
	Name string `json:"name"`
}

type rawType struct {
	Kind          string         `json:"kind"`
	Name          string         `json:"name"`
	Description   *string        `json:"description"`
	Fields        []rawField     `json:"fields"`
	InputFields   []rawField     `json:"inputFields"`
	Interfaces    []rawTypeRef   `json:"interfaces"`
	EnumValues    []rawEnumValue `json:"enumValues"`
	PossibleTypes []rawTypeRef   `json:"possibleTypes"`
}

type rawField struct {
	Name              string     `json:"name"`
	Description       *string    `json:"description"`
	Args              []rawArg   `json:"args"`
	Type              rawTypeRef `json:"type"`
	IsDeprecated      bool       `json:"isDeprecated"`
	DeprecationReason *string    `json:"deprecationReason"`
}

type rawArg struct {
	Name         string     `json:"name"`
	Description  *string    `json:"description"`
	Type         rawTypeRef `json:"type"`
	DefaultValue *string    `json:"defaultValue"`
}

type rawTypeRef struct {
	Kind   *string     `json:"kind"`
	Name   *string     `json:"name"`
	OfType *rawTypeRef `json:"ofType"`
}

type rawEnumValue struct {
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	IsDeprecated      bool    `json:"isDeprecated"`
	DeprecationReason *string `json:"deprecationReason"`
}

type rawDirective struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Locations   []string `json:"locations"`
	Args        []rawArg `json:"args"`
}

// ParseIntrospection parses a raw introspection JSON response into our Schema model.
func ParseIntrospection(data []byte, id, name string) (*schema.Schema, error) {
	raw, err := extractRawSchema(data)
	if err != nil {
		return nil, err
	}

	s := &schema.Schema{
		ID:        id,
		Name:      name,
		Source:    schema.SourceIntrospection,
		CreatedAt: time.Now().UTC(),
	}

	if raw.QueryType != nil {
		s.QueryType = raw.QueryType.Name
	}
	if raw.MutationType != nil {
		s.MutationType = raw.MutationType.Name
	}
	if raw.SubscriptionType != nil {
		s.SubscriptionType = raw.SubscriptionType.Name
	}

	for _, rt := range raw.Types {
		s.Types = append(s.Types, convertType(rt))
	}

	for _, rd := range raw.Directives {
		s.Directives = append(s.Directives, convertDirective(rd))
	}

	return s, nil
}

// extractRawSchema tries multiple JSON structures to find the __schema object.
func extractRawSchema(data []byte) (*rawSchema, error) {
	// Try standard: {"data":{"__schema":{...}}}
	var std introspectionResponse
	if err := json.Unmarshal(data, &std); err == nil && len(std.Data.Schema.Types) > 0 {
		return &std.Data.Schema, nil
	}

	// Try alt: {"__schema":{...}}
	var alt altIntrospectionResponse
	if err := json.Unmarshal(data, &alt); err == nil && len(alt.Schema.Types) > 0 {
		return &alt.Schema, nil
	}

	// Try raw schema directly: {"queryType":{...}, "types":[...]}
	var raw rawSchema
	if err := json.Unmarshal(data, &raw); err == nil && len(raw.Types) > 0 {
		return &raw, nil
	}

	return nil, fmt.Errorf("unrecognized introspection format: no __schema found with types")
}

func convertType(rt rawType) schema.Type {
	t := schema.Type{
		Kind: schema.TypeKind(rt.Kind),
		Name: rt.Name,
	}
	if rt.Description != nil {
		t.Description = *rt.Description
	}

	for _, rf := range rt.Fields {
		t.Fields = append(t.Fields, convertField(rf))
	}
	for _, rf := range rt.InputFields {
		t.InputFields = append(t.InputFields, convertField(rf))
	}
	for _, ev := range rt.EnumValues {
		t.EnumValues = append(t.EnumValues, convertEnumValue(ev))
	}
	for _, iface := range rt.Interfaces {
		if iface.Name != nil {
			t.Interfaces = append(t.Interfaces, *iface.Name)
		}
	}
	for _, pt := range rt.PossibleTypes {
		if pt.Name != nil {
			t.PossibleTypes = append(t.PossibleTypes, *pt.Name)
		}
	}
	return t
}

func convertField(rf rawField) schema.Field {
	f := schema.Field{
		Name:         rf.Name,
		Type:         convertTypeRef(rf.Type),
		IsDeprecated: rf.IsDeprecated,
	}
	if rf.Description != nil {
		f.Description = *rf.Description
	}
	if rf.DeprecationReason != nil {
		f.DeprecationReason = *rf.DeprecationReason
	}
	for _, ra := range rf.Args {
		f.Args = append(f.Args, convertArg(ra))
	}
	return f
}

func convertArg(ra rawArg) schema.Argument {
	a := schema.Argument{
		Name:         ra.Name,
		Type:         convertTypeRef(ra.Type),
		DefaultValue: ra.DefaultValue,
	}
	if ra.Description != nil {
		a.Description = *ra.Description
	}
	return a
}

func convertTypeRef(rt rawTypeRef) schema.TypeRef {
	ref := schema.TypeRef{}
	if rt.Kind != nil {
		ref.Kind = schema.TypeKind(*rt.Kind)
	}
	if rt.Name != nil {
		name := *rt.Name
		ref.Name = &name
	}
	if rt.OfType != nil {
		inner := convertTypeRef(*rt.OfType)
		ref.OfType = &inner
	}
	return ref
}

func convertEnumValue(rev rawEnumValue) schema.EnumValue {
	ev := schema.EnumValue{
		Name:         rev.Name,
		IsDeprecated: rev.IsDeprecated,
	}
	if rev.Description != nil {
		ev.Description = *rev.Description
	}
	if rev.DeprecationReason != nil {
		ev.DeprecationReason = *rev.DeprecationReason
	}
	return ev
}

func convertDirective(rd rawDirective) schema.Directive {
	d := schema.Directive{
		Name:      rd.Name,
		Locations: rd.Locations,
	}
	if rd.Description != nil {
		d.Description = *rd.Description
	}
	for _, ra := range rd.Args {
		d.Args = append(d.Args, convertArg(ra))
	}
	return d
}
