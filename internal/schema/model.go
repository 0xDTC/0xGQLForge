package schema

import (
	"encoding/json"
	"time"
)

// TypeKind represents the kind of a GraphQL type.
type TypeKind string

const (
	KindScalar      TypeKind = "SCALAR"
	KindObject      TypeKind = "OBJECT"
	KindInterface   TypeKind = "INTERFACE"
	KindUnion       TypeKind = "UNION"
	KindEnum        TypeKind = "ENUM"
	KindInputObject TypeKind = "INPUT_OBJECT"
	KindList        TypeKind = "LIST"
	KindNonNull     TypeKind = "NON_NULL"
)

// SchemaSource indicates how the schema was obtained.
type SchemaSource string

const (
	SourceIntrospection  SchemaSource = "introspection"
	SourceReconstruction SchemaSource = "reconstruction"
	SourceImport         SchemaSource = "import"
)

// Schema is the top-level container for a parsed GraphQL schema.
type Schema struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Source           SchemaSource `json:"source"`
	QueryType        string       `json:"queryType"`
	MutationType     string       `json:"mutationType,omitempty"`
	SubscriptionType string       `json:"subscriptionType,omitempty"`
	Types            []Type       `json:"types"`
	Directives       []Directive  `json:"directives,omitempty"`
	CreatedAt        time.Time    `json:"createdAt"`
}

// Type represents a GraphQL type (object, enum, scalar, input, interface, union).
type Type struct {
	Name          string      `json:"name"`
	Kind          TypeKind    `json:"kind"`
	Description   string      `json:"description,omitempty"`
	Fields        []Field     `json:"fields,omitempty"`
	InputFields   []Field     `json:"inputFields,omitempty"`
	EnumValues    []EnumValue `json:"enumValues,omitempty"`
	Interfaces    []string    `json:"interfaces,omitempty"`
	PossibleTypes []string    `json:"possibleTypes,omitempty"`
}

// Field represents a field on a GraphQL type.
type Field struct {
	Name              string     `json:"name"`
	Description       string     `json:"description,omitempty"`
	Type              TypeRef    `json:"type"`
	Args              []Argument `json:"args,omitempty"`
	IsDeprecated      bool       `json:"isDeprecated,omitempty"`
	DeprecationReason string     `json:"deprecationReason,omitempty"`
}

// TypeRef represents a reference to a type, supporting wrapping (NON_NULL, LIST).
type TypeRef struct {
	Kind   TypeKind `json:"kind"`
	Name   *string  `json:"name,omitempty"`
	OfType *TypeRef `json:"ofType,omitempty"`
}

// BaseName unwraps NON_NULL and LIST wrappers to return the underlying type name.
func (t TypeRef) BaseName() string {
	if t.Name != nil {
		return *t.Name
	}
	if t.OfType != nil {
		return t.OfType.BaseName()
	}
	return ""
}

// IsNonNull returns true if this type reference is wrapped in NON_NULL.
func (t TypeRef) IsNonNull() bool {
	return t.Kind == KindNonNull
}

// IsList returns true if this type reference is a LIST (possibly wrapped in NON_NULL).
func (t TypeRef) IsList() bool {
	if t.Kind == KindList {
		return true
	}
	if t.Kind == KindNonNull && t.OfType != nil {
		return t.OfType.IsList()
	}
	return false
}

// IsScalar returns true if the base type is a scalar.
func (t TypeRef) IsScalar() bool {
	if t.Kind == KindScalar {
		return true
	}
	if t.OfType != nil {
		return t.OfType.IsScalar()
	}
	return false
}

// Signature returns a human-readable type signature like "[String!]!" or "Int".
func (t TypeRef) Signature() string {
	switch t.Kind {
	case KindNonNull:
		if t.OfType != nil {
			return t.OfType.Signature() + "!"
		}
		return "!"
	case KindList:
		if t.OfType != nil {
			return "[" + t.OfType.Signature() + "]"
		}
		return "[]"
	default:
		if t.Name != nil {
			return *t.Name
		}
		return "Unknown"
	}
}

// Argument represents a field or directive argument.
type Argument struct {
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	Type         TypeRef `json:"type"`
	DefaultValue *string `json:"defaultValue,omitempty"`
}

// IsRequired returns true if the argument is non-null and has no default value.
func (a Argument) IsRequired() bool {
	return a.Type.IsNonNull() && a.DefaultValue == nil
}

// EnumValue represents a value in a GraphQL enum type.
type EnumValue struct {
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	IsDeprecated      bool   `json:"isDeprecated,omitempty"`
	DeprecationReason string `json:"deprecationReason,omitempty"`
}

// Directive represents a GraphQL directive.
type Directive struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Locations   []string   `json:"locations,omitempty"`
	Args        []Argument `json:"args,omitempty"`
}

// Operation represents a single query, mutation, or subscription operation.
type Operation struct {
	Name        string     `json:"name"`
	Kind        string     `json:"kind"` // "query", "mutation", "subscription"
	Description string     `json:"description,omitempty"`
	Args        []Argument `json:"args,omitempty"`
	ReturnType  TypeRef    `json:"returnType"`
}

// Relationship represents a typed edge between two types in the schema graph.
type Relationship struct {
	FromType  string `json:"fromType"`
	ToType    string `json:"toType"`
	FieldName string `json:"fieldName"`
	IsList    bool   `json:"isList"`
	IsNonNull bool   `json:"isNonNull"`
}

// GraphData holds nodes and edges for D3.js visualization.
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

// GraphField is one field row inside a GraphNode for ERD visualization.
type GraphField struct {
	Name    string `json:"name"`
	TypeSig string `json:"typeSig,omitempty"` // e.g. "String!", "[User]", "Int"
	IsLink  bool   `json:"isLink,omitempty"`  // true if the field type maps to another graph node
}

// GraphNode represents a type node for visualization.
type GraphNode struct {
	ID          string       `json:"id"`
	Kind        TypeKind     `json:"kind"`
	FieldCount  int          `json:"fieldCount"`
	Description string       `json:"description,omitempty"`
	IsRoot      bool         `json:"isRoot,omitempty"`
	RootKind    string       `json:"rootKind,omitempty"` // "query", "mutation", "subscription"
	Fields      []GraphField `json:"fields,omitempty"`
}

// GraphLink represents a relationship edge for visualization.
type GraphLink struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	FieldName string `json:"fieldName"`
	IsList    bool   `json:"isList"`
	IsNonNull bool   `json:"isNonNull"`
}

// Project represents a proxy capture session with associated traffic and optional inferred schema.
type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ProxyAddr    string    `json:"proxyAddr,omitempty"`
	SchemaID     *string   `json:"schemaId,omitempty"`
	TrafficCount int       `json:"trafficCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// CapturedRequest holds proxy-captured GraphQL traffic.
type CapturedRequest struct {
	ID            string            `json:"id"`
	Timestamp     time.Time         `json:"timestamp"`
	Method        string            `json:"method"`
	URL           string            `json:"url"`
	Host          string            `json:"host"`
	Headers       map[string]string `json:"headers,omitempty"`
	OperationName string            `json:"operationName,omitempty"`
	Query         string            `json:"query,omitempty"`
	Variables     json.RawMessage   `json:"variables,omitempty"`
	ResponseCode  int               `json:"responseCode,omitempty"`
	ResponseBody  json.RawMessage   `json:"responseBody,omitempty"`
	Fingerprint   string            `json:"fingerprint,omitempty"`
	ClusterID     *string           `json:"clusterId,omitempty"`
	SchemaID      *string           `json:"schemaId,omitempty"`
	ProjectID     *string           `json:"projectId,omitempty"`
}

// DepthResult contains query depth analysis output.
type DepthResult struct {
	OperationName string   `json:"operationName"`
	MaxDepth      int      `json:"maxDepth"`
	FieldPaths    []string `json:"fieldPaths"`
}

// ComplexityResult contains query complexity estimation output.
type ComplexityResult struct {
	OperationName string  `json:"operationName"`
	Score         float64 `json:"score"`
	FieldCount    int     `json:"fieldCount"`
	ListNesting   int     `json:"listNesting"`
	Risk          string  `json:"risk"` // "low", "medium", "high", "critical"
}

// IDORCandidate represents a potential IDOR vulnerability.
type IDORCandidate struct {
	FieldName      string   `json:"fieldName"`
	ArgName        string   `json:"argName"`
	ObservedValues []string `json:"observedValues,omitempty"`
	Pattern        string   `json:"pattern"` // "sequential_int", "uuid", "encoded"
	Risk           string   `json:"risk"`
}

// DangerousMutation represents a potentially dangerous mutation.
type DangerousMutation struct {
	Name       string   `json:"name"`
	Reason     string   `json:"reason"`
	Indicators []string `json:"indicators"`
	Severity   string   `json:"severity"` // "low", "medium", "high", "critical"
}

// BypassResult holds introspection bypass attempt results.
type BypassResult struct {
	Technique   string `json:"technique"`
	Description string `json:"description"`
	Payload     string `json:"payload"`
	Success     bool   `json:"success"`
	Response    string `json:"response,omitempty"`
}

// SchemaDiff represents differences between two schema versions.
type SchemaDiff struct {
	ID        string        `json:"id"`
	SchemaAID string        `json:"schemaAId"`
	SchemaBID string        `json:"schemaBId"`
	Added     DiffSet       `json:"added"`
	Removed   DiffSet       `json:"removed"`
	Changed   []FieldChange `json:"changed,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
}

// DiffSet groups added or removed schema elements.
type DiffSet struct {
	Types  []string `json:"types,omitempty"`
	Fields []string `json:"fields,omitempty"` // "TypeName.fieldName" format
	Args   []string `json:"args,omitempty"`   // "TypeName.fieldName.argName" format
}

// FieldChange represents a changed field between schema versions.
type FieldChange struct {
	Path     string `json:"path"` // "TypeName.fieldName"
	OldType  string `json:"oldType"`
	NewType  string `json:"newType"`
	Breaking bool   `json:"breaking"`
}

// RBACEntry maps an auth context to its observed permissions.
type RBACEntry struct {
	AuthToken  string   `json:"authToken"`
	Role       string   `json:"role,omitempty"`
	Operations []string `json:"operations"`
	DeniedOps  []string `json:"deniedOps,omitempty"`
}

// FuzzResult holds the result of a field fuzzing attempt.
type FuzzResult struct {
	TypeName     string   `json:"typeName"`
	TestedFields []string `json:"testedFields"`
	ValidFields  []string `json:"validFields"`
	Suggestions  []string `json:"suggestions,omitempty"` // from "Did you mean" errors
}

// QueryCluster groups similar captured queries.
type QueryCluster struct {
	ID          string            `json:"id"`
	Fingerprint string            `json:"fingerprint"`
	Count       int               `json:"count"`
	Queries     []CapturedRequest `json:"queries,omitempty"`
	CommonArgs  []string          `json:"commonArgs,omitempty"`
	VaryingArgs []string          `json:"varyingArgs,omitempty"`
}
