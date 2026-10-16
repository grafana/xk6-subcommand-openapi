// Package tsgen generates a TypeScript k6 client library from an OpenAPI descriptor.
// It decides what the client library contains and builds it as a model.File,
// which the render package spells as TypeScript.
package tsgen

import (
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen/model"
	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen/render"
)

// Options are the settings of a generation run.
type Options struct {
	// Example appends an example script that constructs the client.
	Example bool
}

// Generate builds the client library for doc, reports skipped or approximated constructs through warn,
// and renders it as TypeScript, followed by the example script if opts.Example is set.
// The error is only returned for rendering failures.
func Generate(doc *openapi3.T, opts Options, warn func(string)) ([]byte, error) {
	f := buildFile(doc, warn)
	if opts.Example {
		f.Example = &model.Example{ClassName: f.Client.Name, BaseURL: exampleBaseURL(doc.Servers)}
	}

	return render.Render(f)
}

// defaultBaseURL is the example's base URL if the descriptor has no absolute http or https server URL.
const defaultBaseURL = "http://localhost"

// exampleBaseURL returns the URL of the first server with each variable replaced by its default value,
// if that is an absolute http or https URL, and defaultBaseURL otherwise.
func exampleBaseURL(servers openapi3.Servers) string {
	if len(servers) == 0 || servers[0] == nil {
		return defaultBaseURL
	}

	raw := servers[0].URL
	for _, name := range sortedKeys(servers[0].Variables) {
		if v := servers[0].Variables[name]; v != nil {
			raw = strings.ReplaceAll(raw, "{"+name+"}", v.Default)
		}
	}

	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return defaultBaseURL
	}

	return raw
}

// buildFile builds the model of the client library for doc, reporting skipped
// or approximated constructs through warn.
func buildFile(doc *openapi3.T, warn func(string)) *model.File {
	f := &model.File{Client: model.Client{Name: clientName(doc.Info)}}

	types := newTypeDeclarations(warn, f.Client.Name)
	schemas := newSchemaMapper(types, warn)
	ops := newOperationMapper(types, schemas, warn)

	if doc.Info != nil {
		f.Info = model.Info{
			Title:       doc.Info.Title,
			Description: doc.Info.Description,
			Version:     doc.Info.Version,
		}
	}

	f.Client.Methods = ops.clientMethods(doc)
	f.Types = slices.SortedFunc(maps.Values(types.decls), func(a, b model.TypeDecl) int {
		return strings.Compare(a.Name, b.Name)
	})
	f.Features = features(f.Client.Methods, schemas.usesFileData)

	return f
}

// features returns the optional imports and helpers that methods need.
// usesFileData tells whether a generated type uses FileData.
func features(methods []model.Method, usesFileData bool) model.Features {
	some := func(pred func(m model.Method) bool) bool { return slices.ContainsFunc(methods, pred) }
	hasBody := func(kind model.BodyKind) func(model.Method) bool {
		return func(m model.Method) bool { return m.Body != nil && m.Body.Kind == kind }
	}

	return model.Features{
		Methods: len(methods) > 0,
		// The query helper also encodes URL-encoded bodies.
		Query:     some(func(m model.Method) bool { return m.HasQuery }) || some(hasBody(model.FormBody)),
		Headers:   some(func(m model.Method) bool { return m.HasHeaders }),
		Multipart: some(hasBody(model.MultipartBody)),
		FileData:  usesFileData,
	}
}

// sortedKeys returns the keys of m in lexicographic byte order.
func sortedKeys[M ~map[string]V, V any](m M) []string {
	return slices.Sorted(maps.Keys(m))
}

// clientName returns the client class name: the title in PascalCase followed by "Client",
// or "ApiClient" if the title yields no letters or digits.
func clientName(info *openapi3.Info) string {
	if info != nil {
		if name := pascalCase(info.Title); name != "" {
			return name + "Client"
		}
	}

	return "ApiClient"
}

// typeDeclarations holds the exported type declarations of the client library and makes their names unique,
// so every claimed type name has exactly one declaration.
type typeDeclarations struct {
	names *nameRegistry
	decls map[string]model.TypeDecl // by name
}

// newTypeDeclarations returns empty type declarations whose names cannot be reserved names:
// the client class name, the identifiers that the client library imports, and the global types that it uses
// in type positions. The status code types keep their names because they are declared first.
// Identifiers used only as values, such as JSON, are not reserved, because a type alias does not shadow a value.
func newTypeDeclarations(warn func(string), clientName string) *typeDeclarations {
	names := newNameRegistry("type", warn)
	names.reserve(clientName, "FileData", "FormData", "Params", "Response", "URL", "URLSearchParams")
	names.reserve("ArrayBuffer", "Exclude", "Promise", "Record")

	return &typeDeclarations{names: names, decls: make(map[string]model.TypeDecl)}
}

// declare claims a unique name based on name, builds the type, records the declaration, and returns
// a reference to it. what describes the type for the warning about a renamed type.
// build receives the claimed name before it runs, so recursive schemas can reference themselves,
// and types reached while building get their names afterwards, in discovery order.
func (d *typeDeclarations) declare(name, what, doc string, build func(name string) model.Type) model.Named {
	name = d.names.claim(name, what)
	typ := build(name)
	d.decls[name] = model.TypeDecl{Name: name, Doc: doc, Type: typ}

	return model.Named{Name: name}
}
