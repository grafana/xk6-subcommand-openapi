// Package render spells a model.File as TypeScript.
// It makes no decisions about what the client library contains and reports no warnings.
// It is the only place where text from the descriptor becomes TypeScript source.
// Its template functions panic for a model node they do not know, which the template returns as an error,
// so a gap fails instead of producing wrong output.
package render

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen/model"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// templates are parsed once from the embedded files.
var templates = template.Must( //nolint:gochecknoglobals // parsed once, read-only afterwards
	template.New("").Funcs(template.FuncMap{
		"type":      typeExpr,
		"doc":       docComment,
		"methodDoc": methodDoc,
		"comment":   commentText,
		"prop":      propertyName,
		"str":       stringLiteral,
		"path":      pathTemplate,
	}).ParseFS(templateFS, "templates/*.tmpl"),
)

// Render returns the TypeScript source of f: the client library, followed by the example script if f.Example is set.
func Render(f *model.File) ([]byte, error) {
	var out bytes.Buffer

	if err := templates.ExecuteTemplate(&out, "client.ts.tmpl", f); err != nil {
		return nil, fmt.Errorf("rendering the client library: %w", err)
	}

	if f.Example != nil {
		if err := templates.ExecuteTemplate(&out, "example.ts.tmpl", f.Example); err != nil {
			return nil, fmt.Errorf("rendering the example script: %w", err)
		}
	}

	return out.Bytes(), nil
}
