package tsgen

import (
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen/model"
)

// schemaCtx is where a schema is mapped.
type schemaCtx struct {
	// hint is the name for the schema if it is an inline object or composition, such as "CreatePetBody",
	// "GetPet200", or "NotFoundResponse"; the type name registry may add a suffix.
	// Other schemas ignore it, and if it is empty, an inline object or composition is mapped inline.
	hint string
	// where describes the location for warnings, such as "type Pet" or "the response 200 of GET /pets".
	where string
	// location identifies an inline object or composition in a shared component, such as a response
	// referenced from components.responses, so all uses get the one type named c.hint.
	// If it is empty, each use declares its own type.
	location string
}

// schemaMapper maps schemas to model types and declares the named ones in typeDeclarations.
type schemaMapper struct {
	warn  func(string)
	types *typeDeclarations
	// names holds the names of the named types by location: the RefPath of a reference, or schemaCtx.location.
	names map[string]string
	// usesFileData is set once a type uses FileData, so the client library imports it.
	usesFileData bool
}

func newSchemaMapper(types *typeDeclarations, warn func(string)) *schemaMapper {
	return &schemaMapper{warn: warn, types: types, names: make(map[string]string)}
}

// typeOf maps ref to a type.
// A referenced schema becomes a named type, named after the reference.
// An inline object or composition becomes a named type named c.hint, or an inline type if c.hint is empty.
// Inline schemas nested in other schemas, including the members of compositions, are always inline.
func (m *schemaMapper) typeOf(ref *openapi3.SchemaRef, c schemaCtx) model.Type {
	if ref == nil || ref.Value == nil {
		return model.Unknown
	}

	switch {
	case ref.Ref != "":
		name := pascalCase(refName(ref.Ref))
		if name == "" {
			name = "Schema"
		}

		return m.named(refLocation(ref.Ref, ref.RefPath()), ref.Value, name, "the schema "+ref.Ref)
	case c.hint != "" && (isObject(ref.Value) || isComposition(ref.Value)):
		return m.named(c.location, ref.Value, c.hint, c.where)
	default:
		return m.build(ref.Value, c)
	}
}

// named returns a reference to the named type for schema, declared with a unique name based on name.
// A schema with a location is declared once, from its first use; keying by location rather than by schema
// matters in OpenAPI 3.1, where kin-openapi gives each $ref with sibling keywords its own copy of the schema.
// A schema without a location is declared on every use.
func (m *schemaMapper) named(location string, schema *openapi3.Schema, name, what string) model.Type {
	if existing, ok := m.names[location]; ok {
		return model.Named{Name: existing}
	}

	return m.types.declare(name, what, schema.Description, func(name string) model.Type {
		if location != "" {
			m.names[location] = name
		}

		return m.build(schema, schemaCtx{where: "type " + name})
	})
}

// refLocation returns the resolved location of the $ref ref, which kin-openapi sets for every resolved $ref.
func refLocation(ref string, refPath *url.URL) string {
	if refPath == nil {
		panic(fmt.Sprintf("kin-openapi did not resolve the location of the reference %q", ref))
	}

	return refPath.String()
}

// build maps the content of schema, without naming it.
// nullable, from OpenAPI 3.0, adds null to the whole type, including its allOf, oneOf, and anyOf,
// because a nullable composition is how OpenAPI 3.0 makes a referenced schema nullable.
func (m *schemaMapper) build(schema *openapi3.Schema, c schemaCtx) model.Type {
	var t model.Type

	switch {
	case schema.Const != nil:
		t = literal(schema.Const)
	case len(schema.Enum) > 0:
		members := make([]model.Type, 0, len(schema.Enum))
		for _, v := range schema.Enum {
			members = append(members, literal(v))
		}

		t = union(members...)
	default:
		var parts []model.Type

		if base := m.structural(schema, c); base != nil {
			parts = append(parts, base)
		}

		compose := func(refs openapi3.SchemaRefs, combine func(...model.Type) model.Type) {
			if len(refs) == 0 {
				return
			}

			members := make([]model.Type, 0, len(refs))
			for _, ref := range refs {
				members = append(members, m.typeOf(ref, schemaCtx{where: c.where}))
			}

			parts = append(parts, combine(members...))
		}

		compose(schema.AllOf, intersection)
		compose(schema.OneOf, union)
		compose(schema.AnyOf, union)

		t = model.Unknown
		if len(parts) > 0 {
			t = intersection(parts...)
		}
	}

	if schema.Nullable && t != model.Unknown {
		return union(t, model.Null)
	}

	return t
}

// structural maps the type keywords of schema, or returns nil if it has none.
// A schema without a type is inferred to be an object if it has only object keywords
// (properties or additionalProperties), and an array if it has only items.
// If it has both, its type is ambiguous: it maps to unknown with a warning.
func (m *schemaMapper) structural(schema *openapi3.Schema, c schemaCtx) model.Type {
	types := schema.Type.Slice()
	objectKeywords := len(schema.Properties) > 0 || schema.AdditionalProperties.Has != nil ||
		schema.AdditionalProperties.Schema != nil
	arrayKeywords := schema.Items != nil

	if len(types) == 0 {
		switch {
		case objectKeywords && arrayKeywords:
			m.warn(fmt.Sprintf("the type of a schema in %s could not be inferred, because it has no type "+
				"but both object and array keywords; it is mapped to unknown", c.where))

			return model.Unknown
		case objectKeywords:
			types = []string{openapi3.TypeObject}
		case arrayKeywords:
			types = []string{openapi3.TypeArray}
		default:
			return nil
		}
	}

	members := make([]model.Type, 0, len(types))

	for _, t := range types {
		members = append(members, m.typeName(t, schema, c))
	}

	return union(members...)
}

// typeName maps one value of a schema's type keyword.
// A binary string can be an ArrayBuffer, or FileData with a filename and content type for a multipart part.
func (m *schemaMapper) typeName(t string, schema *openapi3.Schema, c schemaCtx) model.Type {
	switch t {
	case openapi3.TypeString:
		if schema.Format != "binary" {
			return model.String
		}

		m.usesFileData = true

		return union(model.ArrayBuffer, model.FileData)
	case openapi3.TypeInteger, openapi3.TypeNumber:
		return model.Number
	case openapi3.TypeBoolean:
		return model.Boolean
	case openapi3.TypeNull:
		return model.Null
	case openapi3.TypeArray:
		return model.Array{Elem: m.typeOf(schema.Items, schemaCtx{where: c.where})}
	case openapi3.TypeObject:
		return m.object(schema, c)
	default:
		m.warn(fmt.Sprintf("schema type %q in %s is not supported and is mapped to unknown", t, c.where))

		return model.Unknown
	}
}

// object maps an object schema's properties, in lexicographic order, and additionalProperties.
func (m *schemaMapper) object(schema *openapi3.Schema, c schemaCtx) model.Type {
	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}

	slices.Sort(names)

	obj := model.Object{Properties: make([]model.Property, 0, len(names))}

	for _, name := range names {
		ref := schema.Properties[name]

		doc := ""
		if ref != nil && ref.Ref == "" && ref.Value != nil {
			doc = ref.Value.Description
		}

		obj.Properties = append(obj.Properties, model.Property{
			Name:     name,
			Optional: !slices.Contains(schema.Required, name),
			Doc:      doc,
			Type:     m.typeOf(ref, schemaCtx{where: c.where}),
		})
	}

	switch ap := schema.AdditionalProperties; {
	case ap.Schema != nil:
		obj.Index = indexType(m.typeOf(ap.Schema, schemaCtx{where: c.where}), obj.Properties)
	case ap.Has != nil && *ap.Has:
		obj.Index = model.Unknown
	}

	if len(obj.Properties) == 0 && obj.Index == nil {
		return model.Record
	}

	return obj
}

// indexType returns the type of the index signature for additionalProperties of type additional.
// A TypeScript index signature also covers the named properties, unlike additionalProperties,
// so its type includes the type of each property, and undefined if a property is optional.
func indexType(additional model.Type, properties []model.Property) model.Type {
	if len(properties) == 0 || additional == model.Unknown {
		return additional
	}

	members := []model.Type{additional}
	optional := false

	for _, p := range properties {
		members = append(members, p.Type)
		optional = optional || p.Optional
	}

	if optional {
		members = append(members, model.Undefined)
	}

	return union(members...)
}

func isObject(schema *openapi3.Schema) bool {
	return schema.Type.Includes(openapi3.TypeObject) || len(schema.Properties) > 0
}

func isComposition(schema *openapi3.Schema) bool {
	return len(schema.AllOf) > 0 || len(schema.OneOf) > 0 || len(schema.AnyOf) > 0
}

// refName returns the schema name of a reference: the last segment of its JSON pointer,
// or, for a reference without a fragment such as "pet.yaml", the file name without its extension.
func refName(ref string) string {
	file, fragment, _ := strings.Cut(ref, "#")
	if fragment == "" {
		base := path.Base(file)

		return strings.TrimSuffix(base, path.Ext(base))
	}

	segment := fragment[strings.LastIndex(fragment, "/")+1:]
	if unescaped, err := url.PathUnescape(segment); err == nil {
		segment = unescaped
	}

	return strings.NewReplacer("~1", "/", "~0", "~").Replace(segment)
}

// literal maps an enum or const value to a literal type: a string, number, boolean, or null.
// kin-openapi decodes descriptors as JSON, so numbers are float64. Any other value maps to unknown.
func literal(v any) model.Type {
	switch v := v.(type) {
	case nil:
		return model.Null
	case string, bool, float64:
		return model.Literal{Value: v}
	default:
		return model.Unknown
	}
}

// union returns the union of members, flattening nested unions and dropping duplicates
// of named, builtin, and literal types. It returns the only member of a union with one member.
func union(members ...model.Type) model.Type {
	var result model.Union

	for _, member := range members {
		if nested, ok := member.(model.Union); ok {
			for _, n := range nested {
				result = appendUnique(result, n)
			}

			continue
		}

		result = appendUnique(result, member)
	}

	if len(result) == 1 {
		return result[0]
	}

	return result
}

// intersection returns the intersection of members, or the only member of an intersection with one member.
func intersection(members ...model.Type) model.Type {
	if len(members) == 1 {
		return members[0]
	}

	return model.Intersection(members)
}

func appendUnique(types model.Union, t model.Type) model.Union {
	for _, existing := range types {
		if sameType(existing, t) {
			return types
		}
	}

	return append(types, t)
}

// sameType reports whether a and b are the same named, builtin, or literal type.
// Composite types are never the same, because comparing them is not needed to drop duplicates in practice.
func sameType(a, b model.Type) bool {
	switch a := a.(type) {
	case model.Named, model.Builtin:
		return a == b
	case model.Literal:
		bl, ok := b.(model.Literal)

		return ok && a.Value == bl.Value
	default:
		return false
	}
}
