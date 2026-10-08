package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen/model"
)

// identifierPattern matches the names that can be written as unquoted property names.
// It approximates the ECMAScript IdentifierName with Unicode categories; anything else is quoted.
var identifierPattern = regexp.MustCompile(`^[\p{L}\p{Nl}$_][\p{L}\p{Nl}\p{Mn}\p{Mc}\p{Nd}\p{Pc}$]*$`)

// propertyName prints name as a property name, quoted if it is not an identifier.
func propertyName(name string) string {
	if identifierPattern.MatchString(name) {
		return name
	}

	return stringLiteral(name)
}

// stringLiteral prints s as a double-quoted string literal.
// JSON string syntax is valid JavaScript, and encoding/json escapes U+2028 and U+2029,
// which would end a line inside a JavaScript string.
func stringLiteral(s string) string {
	var b bytes.Buffer

	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(s); err != nil {
		// Encoding a string into a buffer cannot fail.
		panic(fmt.Sprintf("encoding string literal: %v", err))
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// templateEscaper escapes the characters that end or interpolate a template literal.
var templateEscaper = strings.NewReplacer(`\`, `\\`, "`", "\\`", "${", `\${`) //nolint:gochecknoglobals // constant

// pathTemplate prints the path as a template literal, with each argument encoded with encodeURIComponent.
func pathTemplate(segments []model.PathSegment) string {
	var b strings.Builder

	b.WriteString("`")

	for _, s := range segments {
		if s.Arg != "" {
			b.WriteString("${encodeURIComponent(String(" + s.Arg + "))}")
		} else {
			b.WriteString(templateEscaper.Replace(s.Text))
		}
	}

	b.WriteString("`")

	return b.String()
}

// commentText returns text on a single line, with runs of whitespace, including line breaks, collapsed
// to one space, and "*/" written as "*\/", so it can neither end a line comment nor a block comment.
func commentText(text string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(text), " "), "*/", `*\/`)
}

// docComment prints text as a single-line doc comment at the indentation level indent, followed by a newline,
// or returns "" if text is empty.
func docComment(text string, indent int) string {
	text = commentText(text)
	if text == "" {
		return ""
	}

	return strings.Repeat(indentUnit, indent) + "/** " + text + " */\n"
}

// methodDoc prints the doc comment of m at the indentation level indent, followed by a newline,
// or returns "" if m has nothing to document.
// Each non-blank paragraph and each path argument description is written on one line, as blocks separated
// by blank lines: one block per paragraph, then one block with an @param line per described path argument.
// A single paragraph without @param lines is written as a single-line doc comment.
func methodDoc(m model.Method, indent int) string {
	var blocks, params []string

	for _, p := range m.Doc {
		if text := commentText(p); text != "" {
			blocks = append(blocks, text)
		}
	}

	for _, a := range m.PathArgs {
		if text := commentText(a.Doc); text != "" {
			params = append(params, "@param "+a.Name+" "+text)
		}
	}

	if len(params) > 0 {
		blocks = append(blocks, strings.Join(params, "\n"))
	}

	prefix := strings.Repeat(indentUnit, indent)

	switch {
	case len(blocks) == 0:
		return ""
	case len(blocks) == 1 && len(params) == 0:
		return prefix + "/** " + blocks[0] + " */\n"
	}

	lines := strings.Split(strings.Join(blocks, "\n\n"), "\n")

	var b strings.Builder

	b.WriteString(prefix + "/**\n")

	for _, line := range lines {
		b.WriteString(strings.TrimRight(prefix+" * "+line, " ") + "\n")
	}

	b.WriteString(prefix + " */\n")

	return b.String()
}
