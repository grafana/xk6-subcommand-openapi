package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/xk6-subcommand-openapi/internal/tsgen/model"
)

// indentUnit is one level of indentation.
const indentUnit = "  "

// typeExpr prints t as a TypeScript type expression.
// indent is the indentation level of the line the expression starts on:
// the properties of an object type are indented one level deeper, and its closing brace at that level.
// It panics for a node that is not in the model, which the template returns as an error.
func typeExpr(t model.Type, indent int) string {
	switch t := t.(type) {
	case model.Named:
		return t.Name
	case model.Builtin:
		return builtin(t)
	case model.Literal:
		return literal(t)
	case model.Array:
		elem := typeExpr(t.Elem, indent)
		if isComposite(t.Elem) {
			elem = "(" + elem + ")"
		}

		return elem + "[]"
	case model.Union:
		// Intersections bind more tightly than unions, so no member needs parentheses.
		return join(t, " | ", indent, func(model.Type) bool { return false })
	case model.Intersection:
		return join(t, " & ", indent, func(m model.Type) bool { _, ok := m.(model.Union); return ok })
	case model.Exclude:
		return "Exclude<" + typeExpr(t.From, indent) + ", " + typeExpr(t.Excluded, indent) + ">"
	case model.Object:
		return object(t, indent)
	default:
		panic(fmt.Sprintf("unknown type node %#v", t))
	}
}

// builtinNames spells the builtin types.
var builtinNames = map[model.Builtin]string{ //nolint:gochecknoglobals // constant lookup table
	model.String:       "string",
	model.Number:       "number",
	model.Boolean:      "boolean",
	model.Null:         "null",
	model.Undefined:    "undefined",
	model.Unknown:      "unknown",
	model.ArrayBuffer:  "ArrayBuffer",
	model.FileData:     "FileData",
	model.Params:       "Params",
	model.Response:     "Response",
	model.ResponseBody: `Response["body"]`,
	model.Record:       "Record<string, unknown>",
}

func builtin(b model.Builtin) string {
	name, ok := builtinNames[b]
	if !ok {
		panic(fmt.Sprintf("unknown builtin type %d", b))
	}

	return name
}

// literal prints a literal type: a string literal, a number as JavaScript would print it, or a boolean.
// Numbers are always finite, because descriptors are decoded as JSON.
func literal(l model.Literal) string {
	switch v := l.Value.(type) {
	case string:
		return stringLiteral(v)
	case float64:
		b, err := json.Marshal(v)
		if err != nil {
			panic(fmt.Sprintf("number literal %v: %v", v, err))
		}

		return string(b)
	case bool:
		return fmt.Sprint(v)
	default:
		panic(fmt.Sprintf("unknown literal value %#v", l.Value))
	}
}

// join prints members separated by sep, wrapping the members for which parens is true in parentheses.
func join(members []model.Type, sep string, indent int, parens func(model.Type) bool) string {
	if len(members) == 0 {
		panic("empty union or intersection")
	}

	parts := make([]string, 0, len(members))

	for _, m := range members {
		s := typeExpr(m, indent)
		if parens(m) {
			s = "(" + s + ")"
		}

		parts = append(parts, s)
	}

	return strings.Join(parts, sep)
}

// isComposite reports whether t is a union or intersection, which needs parentheses as an array element.
func isComposite(t model.Type) bool {
	switch t.(type) {
	case model.Union, model.Intersection:
		return true
	default:
		return false
	}
}

// object prints an object type with one property per line, each preceded by its doc comment,
// followed by the index signature.
func object(o model.Object, indent int) string {
	if len(o.Properties) == 0 && o.Index == nil {
		return "{}"
	}

	inner := strings.Repeat(indentUnit, indent+1)

	var b strings.Builder

	b.WriteString("{\n")

	for _, p := range o.Properties {
		optional := ""
		if p.Optional {
			optional = "?"
		}

		b.WriteString(docComment(p.Doc, indent+1))
		fmt.Fprintf(&b, "%s%s%s: %s;\n", inner, propertyName(p.Name), optional, typeExpr(p.Type, indent+1))
	}

	if o.Index != nil {
		fmt.Fprintf(&b, "%s[key: string]: %s;\n", inner, typeExpr(o.Index, indent+1))
	}

	b.WriteString(strings.Repeat(indentUnit, indent) + "}")

	return b.String()
}
