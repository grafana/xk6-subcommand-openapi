package tsgen

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen/model"
)

// operation is an operation of the descriptor with the path and path item it belongs to.
type operation struct {
	path   string
	method string // as returned by PathItem.Operations: uppercase for the fixed methods
	item   *openapi3.PathItem
	op     *openapi3.Operation
}

// String describes the operation for warnings, such as "GET /pets".
func (o operation) String() string {
	return o.method + " " + o.path
}

// methodOrder is the order of the standard HTTP methods within a path.
// Other methods follow in lexicographic order.
var methodOrder = []string{ //nolint:gochecknoglobals // constant lookup table
	http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete,
	http.MethodOptions, http.MethodHead, http.MethodPatch, http.MethodTrace,
}

// operations returns the operations of doc ordered by path in lexicographic byte order,
// and within a path by HTTP method in methodOrder, followed by other methods in lexicographic order.
func operations(doc *openapi3.T) []operation {
	if doc.Paths == nil {
		return nil
	}

	items := doc.Paths.Map()

	var result []operation

	for _, p := range sortedKeys(items) {
		item := items[p]
		if item == nil {
			continue
		}

		ops := item.Operations()
		methods := make([]string, 0, len(ops))

		for m := range ops {
			methods = append(methods, m)
		}

		slices.SortFunc(methods, func(a, b string) int {
			return cmp.Or(cmp.Compare(methodRank(a), methodRank(b)), strings.Compare(a, b))
		})

		for _, m := range methods {
			result = append(result, operation{path: p, method: m, item: item, op: ops[m]})
		}
	}

	return result
}

func methodRank(method string) int {
	if i := slices.Index(methodOrder, method); i >= 0 {
		return i
	}

	return len(methodOrder)
}

// operationMapper maps operations to client methods and declares their Options types in typeDeclarations.
type operationMapper struct {
	warn        func(string)
	methodNames *nameRegistry
	types       *typeDeclarations
	schemas     *schemaMapper
	statusTypes *statusTypes // nil if the descriptor has no operations
}

func newOperationMapper(types *typeDeclarations, schemas *schemaMapper, warn func(string)) *operationMapper {
	return &operationMapper{
		warn:        warn,
		methodNames: newNameRegistry("method", warn),
		types:       types,
		schemas:     schemas,
	}
}

// clientMethods maps the operations of doc to client methods, in operation order,
// and warns about the security schemes the operations reference that callers cannot supply.
func (o *operationMapper) clientMethods(doc *openapi3.T) []model.Method {
	ops := operations(doc)
	methods := make([]model.Method, 0, len(ops))
	referenced := make(map[string]bool)

	// The status code types are declared first, so they keep their names.
	if len(ops) > 0 {
		o.statusTypes = declareStatusTypes(o.types)
	}

	for _, op := range ops {
		methods = append(methods, o.method(op))

		// An operation's own security, even an empty list, replaces the document's.
		requirements := doc.Security
		if op.op.Security != nil {
			requirements = *op.op.Security
		}

		for _, requirement := range requirements {
			for name := range requirement {
				referenced[name] = true
			}
		}
	}

	o.checkSecuritySchemes(doc, referenced)

	return methods
}

// checkSecuritySchemes warns, in order of name, about each referenced security scheme that callers
// cannot supply through k6Params: an API key in the query, and an http scheme other than
// basic, bearer, or digest. Undeclared schemes are ignored; validation reports them.
func (o *operationMapper) checkSecuritySchemes(doc *openapi3.T, referenced map[string]bool) {
	if doc.Components == nil {
		return
	}

	for _, name := range sortedKeys(referenced) {
		ref := doc.Components.SecuritySchemes[name]
		if ref == nil || ref.Value == nil {
			continue
		}

		scheme := ref.Value

		switch {
		case scheme.Type == "apiKey" && scheme.In == openapi3.ParameterInQuery:
			o.warn(fmt.Sprintf("the security scheme %q sends an API key in the query, "+
				"which the client cannot send", name))
		case scheme.Type == "http" &&
			!slices.Contains([]string{"basic", "bearer", "digest"}, strings.ToLower(scheme.Scheme)):
			o.warn(fmt.Sprintf("the security scheme %q uses the HTTP authentication scheme %q, "+
				"which the client cannot send", name, scheme.Scheme))
		}
	}
}

// method builds the client method for op and declares the types it needs.
// Types are discovered in this order: the types of the path arguments, the Options type,
// the types of the query and header parameters and of the request body, the Result type,
// then the types of the responses.
func (o *operationMapper) method(op operation) model.Method {
	name := o.methodName(op)
	typeName := pascalCase(name)
	params := o.parameters(op)

	m := model.Method{
		Name:       name,
		HTTPMethod: op.method,
		Doc:        []string{op.op.Summary, op.op.Description},
		HasQuery:   len(params.query) > 0,
		HasHeaders: len(params.header) > 0,
	}

	m.PathArgs, m.Path = o.path(op, params.path)

	var members []model.Property

	m.Options = o.types.declare(typeName+"Options", "the options of "+op.String(), "", func(string) model.Type {
		members = []model.Property{{Name: "k6Params", Optional: true, Type: model.Params}}

		if m.HasQuery {
			members = append(members, o.paramsMember("query", typeName+"Query", params.query, op))
		}

		if m.HasHeaders {
			members = append(members, o.paramsMember("headers", typeName+"Headers", params.header, op))
		}

		if body, member := o.requestBody(op, typeName); body != nil {
			m.Body = body
			members = append(members, member)
		}

		slices.SortFunc(members, func(a, b model.Property) int { return strings.Compare(a.Name, b.Name) })

		return model.Object{Properties: members}
	})

	m.OptionsOptional = !slices.ContainsFunc(members, func(p model.Property) bool { return !p.Optional })
	m.Result = o.result(op, typeName)

	return m
}

// The media types that the client encodes itself.
const (
	jsonMediaType      = "application/json"
	formMediaType      = "application/x-www-form-urlencoded"
	multipartMediaType = "multipart/form-data"
)

// requestBody returns how the method for op encodes its request body, and the body member of its Options,
// or nil if op has no request body with a media type.
// The media type is application/json if the body offers it, and otherwise the first in lexicographic order.
func (o *operationMapper) requestBody(op operation, typeName string) (*model.Body, model.Property) {
	if op.op.RequestBody == nil || op.op.RequestBody.Value == nil || len(op.op.RequestBody.Value.Content) == 0 {
		return nil, model.Property{}
	}

	requestBody := op.op.RequestBody.Value
	mediaTypes := sortedKeys(requestBody.Content)

	mediaType := mediaTypes[0]
	if i := slices.IndexFunc(mediaTypes, func(t string) bool { return mediaBase(t) == jsonMediaType }); i >= 0 {
		mediaType = mediaTypes[i]
	}

	media := requestBody.Content[mediaType]
	where := "the request body of " + op.String()

	var body model.Body

	switch mediaBase(mediaType) {
	case jsonMediaType:
		body = model.Body{Kind: model.JSONBody, ContentType: jsonMediaType}
	case formMediaType:
		body = model.Body{Kind: model.FormBody, ContentType: formMediaType}
	case multipartMediaType:
		// The Content-Type header carries the boundary, which is chosen at runtime.
		body = model.Body{Kind: model.MultipartBody}

		if media != nil && len(media.Encoding) > 0 {
			o.warn(fmt.Sprintf("%s declares a per-field encoding, which is ignored", where))
		}
	default:
		body = model.Body{Kind: model.RawBody}
		if !strings.Contains(mediaType, "*") {
			body.ContentType = mediaType
		}
	}

	member := model.Property{Name: "body", Optional: !requestBody.Required, Doc: requestBody.Description}

	switch {
	case body.Kind == model.RawBody:
		// The client sends a raw body as given, and k6 only sends strings and binary data unchanged,
		// so the schema, which describes the content before serialization, is not mapped.
		member.Type = model.Union{model.String, model.ArrayBuffer}
	case media != nil:
		c := schemaCtx{hint: typeName + "Body", where: where}
		if name, location := sharedComponent(op.op.RequestBody.Ref, op.op.RequestBody.RefPath(), "Body"); location != "" {
			c.hint, c.location = name, location
		}

		member.Type = o.schemas.typeOf(media.Schema, c)
		if body.Kind == model.FormBody {
			o.checkNestedFields(member.Type, where)
		}
	default:
		member.Type = model.Unknown
	}

	return &body, member
}

// sharedComponent returns the type name and the location for an inline schema in a component, such as
// a response from components.responses, that an operation references with ref, resolved to refPath.
// The name is the component's name in PascalCase followed by kind, such as NotFoundResponse.
// If ref is empty, the component is not shared, and sharedComponent returns no name and no location.
func sharedComponent(ref string, refPath *url.URL, kind string) (string, string) {
	if ref == "" {
		return "", ""
	}

	return pascalCase(refName(ref)) + kind, refLocation(ref, refPath)
}

// mediaBase returns mediaType without parameters, trimmed and in lowercase.
func mediaBase(mediaType string) string {
	base, _, _ := strings.Cut(mediaType, ";")

	return strings.ToLower(strings.TrimSpace(base))
}

// methodName returns the unique method name for op: its operationId in camelCase,
// or the lowercase HTTP method followed by the path in PascalCase.
func (o *operationMapper) methodName(op operation) string {
	name := camelCase(op.op.OperationID)
	if name == "" {
		name = camelCase(strings.ToLower(op.method) + " " + op.path)
	}

	return o.methodNames.claim(name, "the operation "+op.String())
}

// params are the parameters of an operation by location.
type params struct {
	path   map[string]*openapi3.Parameter // by name
	query  []*openapi3.Parameter          // sorted by name
	header []*openapi3.Parameter          // sorted by name
}

// parameters returns the parameters of op, including those of its path item that op
// does not override with a parameter of the same name and location.
// Cookie parameters are skipped with a warning, and parameters in other locations are ignored.
func (o *operationMapper) parameters(op operation) params {
	type key struct{ in, name string }

	byKey := make(map[key]*openapi3.Parameter)

	for _, refs := range []openapi3.Parameters{op.item.Parameters, op.op.Parameters} {
		for _, ref := range refs {
			if ref != nil && ref.Value != nil {
				byKey[key{ref.Value.In, ref.Value.Name}] = ref.Value
			}
		}
	}

	sorted := slices.SortedFunc(maps.Values(byKey), func(a, b *openapi3.Parameter) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.In, b.In))
	})

	result := params{path: make(map[string]*openapi3.Parameter)}

	for _, p := range sorted {
		switch p.In {
		case openapi3.ParameterInPath:
			result.path[p.Name] = p
		case openapi3.ParameterInQuery:
			o.checkQueryStyle(p, op)
			result.query = append(result.query, p)
		case openapi3.ParameterInHeader:
			result.header = append(result.header, p)
		case openapi3.ParameterInCookie:
			o.warn(fmt.Sprintf("the cookie parameter %q of %s is skipped; "+
				"send it through the cookies of k6Params", p.Name, op))
		}
	}

	return result
}

// checkQueryStyle warns if the query parameter p declares a serialization other than
// style form with explode, the only one the client implements.
func (o *operationMapper) checkQueryStyle(p *openapi3.Parameter, op operation) {
	if (p.Style == "" || p.Style == openapi3.SerializationForm) && (p.Explode == nil || *p.Explode) {
		return
	}

	o.warn(fmt.Sprintf("the query parameter %q of %s declares a serialization other than style %q "+
		"with explode; it is serialized with style %q and explode", p.Name, op,
		openapi3.SerializationForm, openapi3.SerializationForm))
}

// checkNestedFields warns about each field of the URL-encoded body of type t, described by where,
// that allows a nested value, and once about its additional fields if they allow one.
// The fields are the properties of the object types that t combines, and a field that several of them
// declare is reported once.
func (o *operationMapper) checkNestedFields(t model.Type, where string) {
	warned := make(map[string]bool)
	warnedAdditional := false

	for _, m := range o.types.members(t) {
		obj, ok := m.(model.Object)
		if !ok {
			continue
		}

		for _, p := range obj.Properties {
			if !warned[p.Name] && o.types.nested(p.Type, true, true) {
				warned[p.Name] = true
				o.warn(fmt.Sprintf("the field %q of %s allows %s", p.Name, where, nestedWarning))
			}
		}

		if !warnedAdditional && obj.Additional != nil && o.types.nested(obj.Additional, true, true) {
			warnedAdditional = true
			o.warn(fmt.Sprintf("the additional fields of %s allow %s", where, nestedWarning))
		}
	}
}

// nestedWarning ends the warning about a query parameter or URL-encoded body field that allows a nested value.
const nestedWarning = "nested objects or arrays, which are not supported and are omitted"

// placeholderPattern matches a placeholder of a path template, such as {petId}, and captures its name.
var placeholderPattern = regexp.MustCompile(`\{([^}]*)\}`)

// path splits the path template of op into segments and returns one argument per distinct placeholder,
// in the order of their first appearance.
// A placeholder without a declared path parameter becomes a string argument with a warning.
func (o *operationMapper) path(
	op operation, declared map[string]*openapi3.Parameter,
) ([]model.PathArg, []model.PathSegment) {
	var (
		args     []model.PathArg
		segments []model.PathSegment
	)

	text := func(s string) {
		if s != "" {
			segments = append(segments, model.PathSegment{Text: s})
		}
	}

	argNames := newNameRegistry("path argument", o.warn)
	argOf := make(map[string]string) // placeholder to argument name
	end := 0

	for _, match := range placeholderPattern.FindAllStringSubmatchIndex(op.path, -1) {
		text(op.path[end:match[0]])
		end = match[1]

		placeholder := op.path[match[2]:match[3]]

		name, ok := argOf[placeholder]
		if !ok {
			arg := o.pathArg(op, placeholder, declared[placeholder], argNames)
			args = append(args, arg)
			name = arg.Name
			argOf[placeholder] = name
		}

		segments = append(segments, model.PathSegment{Arg: name})
	}

	text(op.path[end:])

	return args, segments
}

// pathArg returns the argument for placeholder, with a unique name claimed from names.
// param is the declared path parameter, or nil if there is none.
func (o *operationMapper) pathArg(
	op operation, placeholder string, param *openapi3.Parameter, names *nameRegistry,
) model.PathArg {
	what := fmt.Sprintf("the path parameter %q of %s", placeholder, op)

	name := argName(placeholder)
	if name == "" {
		name = "arg"
	}

	arg := model.PathArg{Name: names.claim(name, what), Type: model.String}

	if param != nil {
		arg.Doc = param.Description
		arg.Type = o.schemas.typeOf(param.Schema, schemaCtx{where: what})
	} else {
		o.warn(fmt.Sprintf("the path of %s has the placeholder {%s} without a path parameter; "+
			"it is passed as a string", op, placeholder))
	}

	return arg
}

// paramsMember declares a type with a unique name based on name with one field per parameter in ps, and returns
// the Options member that references it, which is required if any of the parameters is required.
func (o *operationMapper) paramsMember(member, name string, ps []*openapi3.Parameter, op operation) model.Property {
	ref := o.types.declare(name, fmt.Sprintf("the %s parameters of %s", member, op), "", func(string) model.Type {
		fields := make([]model.Property, 0, len(ps))

		for _, p := range ps {
			where := fmt.Sprintf("the parameter %q of %s", p.Name, op)
			typ := o.schemas.typeOf(p.Schema, schemaCtx{where: where})

			if p.In == openapi3.ParameterInQuery && o.types.nested(typ, true, true) {
				o.warn(fmt.Sprintf("the query parameter %q of %s allows %s", p.Name, op, nestedWarning))
			}

			fields = append(fields, model.Property{Name: p.Name, Optional: !p.Required, Doc: p.Description, Type: typ})
		}

		return model.Object{Properties: fields}
	})

	required := slices.ContainsFunc(ps, func(p *openapi3.Parameter) bool { return p.Required })

	return model.Property{Name: member, Optional: !required, Type: ref}
}
