package tsgen

import (
	"fmt"
	"slices"

	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen/model"
)

// nested reports whether a query parameter or URL-encoded body field of type t, or a part of it, allows a nested
// value: an object or array that the query helper omits, because it serializes with style form and explode.
// objects and arrays tell whether an object or an array is allowed at t's position:
//
//	top-level value: objects and arrays; an array element, object property, or index signature is checked below
//	object property or index signature: arrays; an array element is checked below
//	array element: neither
//
// A Record at the top level allows no nested value, because its values are unknown.
// Named types are resolved, so it must only be used once every declaration it reaches is complete.
func (d *typeDeclarations) nested(t model.Type, objects, arrays bool) bool {
	return slices.ContainsFunc(d.members(t), func(m model.Type) bool {
		switch m := m.(type) {
		case model.Array:
			return !arrays || d.nested(m.Elem, false, false)
		case model.Object:
			return !objects || (m.Index != nil && d.nested(m.Index, false, true)) ||
				slices.ContainsFunc(m.Properties, func(p model.Property) bool { return d.nested(p.Type, false, true) })
		default:
			return m == model.Record && !objects
		}
	})
}

// members returns the types that t combines, with named types resolved to their declarations and
// unions and intersections replaced by their members, recursively. Each named type is resolved once,
// which bounds the walk on recursive types. It panics if a named type has no declaration yet.
func (d *typeDeclarations) members(t model.Type) []model.Type {
	var result []model.Type

	visited := make(map[string]bool)

	var walk func(t model.Type)
	walk = func(t model.Type) {
		switch t := t.(type) {
		case model.Named:
			if visited[t.Name] {
				return
			}

			decl, ok := d.decls[t.Name]
			if !ok {
				panic(fmt.Sprintf("the type %q is checked for nested values before its declaration is complete", t.Name))
			}

			visited[t.Name] = true
			walk(decl.Type)
		case model.Union:
			for _, m := range t {
				walk(m)
			}
		case model.Intersection:
			for _, m := range t {
				walk(m)
			}
		default:
			result = append(result, t)
		}
	}

	walk(t)

	return result
}
