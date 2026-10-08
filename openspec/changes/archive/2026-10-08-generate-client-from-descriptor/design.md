# Design

## Context

The subcommand is a flat root package today: `cmd.go` defines the cobra command, `generate.go` builds the output and writes it to stdout or the `--out` file through `gs.FS`, and `help.md` and `templates/example.ts` are embedded.
The `<openapi-spec>` argument is ignored and the client library is empty.
This change moves the command into `internal/command` and adds the descriptor loader and the generator as further `internal/` packages, as the scaffolding design anticipated; only the registration stays in the root package.
The k6-ci golangci config forbids `os.*` and `fmt.Print*` in our code, also in tests, so file and console I/O go through k6's `state.GlobalState` (`FS`, `Getwd`, `Stdout`, `Logger`).
k6 v2.3.0 has no native `URL` or `URLSearchParams`, and k6 strips TypeScript types without type-checking, so generated types only help in editors.
See proposal.md for motivation and the specs for the required behavior.

## Goals / Non-Goals

**Goals:**
- Load descriptors with an established Go OpenAPI library instead of a hand-written parser.
- Keep generation testable without a network or a k6 binary, and prove with a real k6 that every generated variant loads.
- Keep the generator's structure (descriptor → model → render) so that later changes can add constructs without touching the output plumbing.

**Non-Goals:**
- Swagger 2.0 support, which can be added later as a separate change that converts 2.0 documents to 3.0 before generation.
- Splitting the output into multiple files, a configurable client name, or a configurable base URL default.
- A `--strict` mode that fails on validation errors.
- Calling an operation from the `--example` script.
- Runtime validation of request or response data against the schemas.
- Verifying the runtime behavior of the generated client (the `Request building` and `Method result` requirements) by running it in k6 against a test server; this follows in a later change, as the `generated-client` spec's `Purpose` states.
- Exposing the operation ID at runtime, either as a field of the method result or as a default k6 tag on each request; tagging requests with the operation ID or the path template is a candidate for a later change.
- Generating support for `securitySchemes` and `security` requirements, such as constructor options for credentials that are applied per operation; this is postponed to a later change, not dropped (see Security schemes).
- Running the generated client without network access; the jslib.k6.io imports are hardcoded for now, and removing the network requirement is postponed to a later change, not dropped.

## Decisions

### Library: kin-openapi
Use `github.com/getkin/kin-openapi/openapi3` for loading, `$ref` resolution, and validation.
It supports OpenAPI 3.0 and 3.1, partially 3.2 (including the `QUERY` method and additional operations through `PathItem.Operations()`), JSON and YAML, and external refs.
Alternative considered: `pb33f/libopenapi`.
Rejected because kin-openapi's resolved `*openapi3.Schema` graph, with the resolved location of every `$ref`, maps directly onto the generator's type registry, and its pluggable URI reader is exactly the hook needed for `gs.FS` and timeouts.

### Package layout
```text
register.go                     package openapi: registration only
internal/command/               cobra command, flags, run, output writing
  command.go, output.go, help.md
internal/descriptor/            loading, version check, validation
  descriptor.go
internal/tsgen/                 decisions: operations, schema mapping, naming
  tsgen.go, operations.go, results.go, schema.go, names.go
internal/tsgen/model/           data contract between tsgen and render
  model.go
internal/tsgen/render/          TypeScript syntax and embedded templates
  render.go, types.go, syntax.go
  templates/client.ts.tmpl, templates/example.ts.tmpl
test/                           package test: in-process end-to-end tests
  cmd_test.go, load_test.go, golden_test.go
  testdata/<case>.yaml, <case>_client.ts, <case>_warnings.txt
  testdata/corpus/<name>.yaml|.json, <name>_client.ts, <name>_warnings.txt, README.md
  it.sh
.gitattributes                  test/testdata/** text eol=lf
```

`cmd.go`, `generate.go`, `help.md`, `templates/example.ts`, and `cmd_test.go` are removed from the root.

`register.go` holds the package doc comment and an `init()` that calls `subcommand.RegisterExtension("openapi", command.NewSubcommand)`.
It must stay in the root, because `xk6 build --with <module>=.` imports the module's root package, and `internal/` packages cannot be imported from outside the module.

Dependencies:
- root → `command`
- `command` → `descriptor`, `tsgen`
- `tsgen` → kin-openapi, `model`, `render`
- `render` → `model`
- `descriptor` → kin-openapi, `fsext`
- `model` → standard library only
- `test` → `command`

`descriptor` and `tsgen` do not import each other, and `model` and `render` do not import kin-openapi.
There is no direct `oasdiff/yaml` dependency.

I/O:
- `descriptor` reads the descriptor and its refs through `gs.FS` and HTTP.
- `command` writes the output to `gs.Stdout` or, with `--out`, through `gs.FS`, and reports warnings through `gs.Logger`.
- `tsgen`, `model`, and `render` do no I/O.

### Command
`internal/command/command.go` embeds `help.md` and exposes:

```go
type Config struct{ FetchTimeout time.Duration }

// DefaultConfig is the only place that sets the production timeout of 30 seconds.
func DefaultConfig() Config

// NewSubcommand is registered with k6 and always uses DefaultConfig.
func NewSubcommand(gs *state.GlobalState) *cobra.Command

// NewSubcommandWithConfig is used by tests, for example with a short FetchTimeout.
func NewSubcommandWithConfig(gs *state.GlobalState, cfg Config) *cobra.Command
```

They are exported because the tests in `test/` are a different package; `internal/` keeps them out of the module's public API.
Tests pass a short `FetchTimeout`, so the timeout scenario runs in milliseconds and parallel tests do not share mutable package state.

`command.go` also holds the `options` (argument and flags) and `run(gs *state.GlobalState, cfg Config, opts options) error`, which:
1. Validates the `--out` path with `resolveOutPath`.
2. Loads the descriptor with `descriptor.Load`.
3. Generates the output with `tsgen.Generate`.
4. Writes the output to `gs.Stdout` or the `--out` file.

Nothing is written to stdout or the `--out` file before generation has succeeded.
`output.go` holds `resolveOutPath`, unchanged from today's `generate.go`, and the writing of the output.

`run` takes everything from `gs`:
- The context is `gs.Ctx`.
  k6 runs its root command with `ExecuteC()` and no context, so `cmd.Context()` is not connected to k6's context; `gs.Ctx` is.
- Output goes to `gs.Stdout`.
  k6's root command calls `SetOut(gs.Stdout)`, so in production this is the same writer as today's `cmd.OutOrStdout()`.
- Warnings go to `gs.Logger.Warn`, through a `warn func(string)` passed to `descriptor.Load` and `tsgen.Generate`, so the internal packages stay independent of logrus.
  k6 itself reports the command's returned error through the same logger.

### Loading through kin-openapi's reader with `gs.FS` and a timeout
`internal/descriptor/descriptor.go`:

```go
type Options struct {
	FS           fsext.Fs
	Cwd          string
	FetchTimeout time.Duration // set from command.Config; no default here
	Warn         func(string)
}

// Load reads the descriptor at location (a URL, or a path resolved against Cwd),
// resolves its refs, checks its version, validates it, and reports validation
// errors through Warn.
func Load(ctx context.Context, location string, opts Options) (*openapi3.T, error)
```

`Load` works in these steps:
1. An argument starting with `http://` or `https://` is parsed as a URL.
   Any other argument is a file path, joined with `Cwd` if relative, and converted to a URL path with forward slashes, so relative `$ref`s resolve against the descriptor's directory, not the process's.
   On Windows, `C:\test\api.yaml` becomes `C:/test/api.yaml`, the same form kin-openapi's own `LoadFromFile` uses; no leading `/` is added, because `URIMapCache` only caches paths that `filepath.IsAbs` accepts, and on Windows it rejects `/C:/test/api.yaml`.
2. The loader is configured with:

   ```go
   loader.IsExternalRefsAllowed = true
   loader.Context = ctx
   loader.ReadFromURIFunc = openapi3.URIMapCache(openapi3.ReadFromURIs(
   	openapi3.ReadFromHTTP(&http.Client{Timeout: opts.FetchTimeout}),
   	readFromFS(opts.FS),
   ))
   ```

   `ReadFromHTTP` already fails on status codes above 399.
   kin-openapi does not pass the context to HTTP requests, so the client timeout bounds them, and Ctrl-C terminates `k6 x` subcommands directly.
3. `loader.LoadFromURI(location)` reads the root document through the same reader and resolves relative refs against its location.
4. The version is checked on the loaded document (see below).
5. The document is validated (see below).

All errors are wrapped with the argument as given by the user, so messages name the descriptor.

`readFromFS` is the `ReadFromURIFunc` for URLs without a host and with an empty or `file` scheme.
It converts the URL path back to a file path with `filepath.FromSlash` and reads it with `fsext.ReadFile`, so local reads stay on k6's filesystem abstraction and in-process tests use the in-memory FS.
It returns `openapi3.ErrURINotSupported` for every other URL.
A `$ref` written as a file URL with a Windows drive letter, such as `file:///C:/specs/pet.yaml`, is not supported, as in kin-openapi's own `ReadFromFile`; relative refs work on every platform.

### Version check after loading
kin-openapi v0.149.0 loads a Swagger 2.0 document and an `openapi: "4.0.0"` document without error, so the loaded document is checked:
- If `doc.Extensions["swagger"]` is present, loading fails with "Swagger 2.0 descriptors are not supported".
- If `doc.OpenAPI` does not have major version 3, loading fails with the unsupported version.

Checking the loaded document needs no second parse of the root document and no direct YAML dependency.
Alternative considered: decoding only the `swagger` and `openapi` fields with `oasdiff/yaml` before loading.
Rejected because the Swagger 2.0 documents tried, including ones with `#/definitions` and `#/parameters` refs, load without error, so the pre-check only adds a dependency and a second read (see Risks for documents that fail to load).
Swagger 2.0 support can later be added at this point by converting the document with `openapi2conv`.

### Validation and generator problems become warnings
After the version check, `doc.Validate(ctx, openapi3.EnableMultiError())` runs; without `EnableMultiError`, `Validate` stops at the first error.
Each error in the resulting `MultiError` becomes one warning, with whitespace collapsed to a single line.
The generator also reports warnings (skipped cookie parameters, renamed methods or types, undeclared path parameters, unsupported query styles, per-field multipart `encoding`, unknown schema types, security schemes that callers cannot supply).
Both packages report warnings through the `warn` callback as they find them, and the command logs each one with `gs.Logger.Warn`.
Warnings therefore never reach the output, and they reach stdout only if the user sets `--log-output=stdout`.
Generation never depends on validation having passed, so the generator itself guards against inputs that would produce broken code: method and type names are made unique, and undeclared path placeholders become arguments.
Alternative considered: fail on validation errors with an opt-out flag.
Rejected because real-world descriptors often have harmless validation errors that users cannot fix.
Alternative considered: writing `warning: ` lines to `cmd.ErrOrStderr()`.
Rejected because k6 reports its own messages, including the command's error, through its logger, so warnings get the same format and respect `--log-output` and `--log-format`.

### Generator: tsgen decides, model carries, render spells
`internal/tsgen/tsgen.go`:

```go
type Options struct{ Example bool }

// Generate builds the model from doc, reports skipped or approximated
// constructs through warn, and renders the model.
// The error is only returned for rendering failures.
func Generate(doc *openapi3.T, opts Options, warn func(string)) ([]byte, error)
```

`tsgen.go` also builds the `model.File`, sorts the type declarations, computes the feature flags for imports and helpers from the finished methods, and holds the type declarations with their unique names, which the operation and schema mappers share.
The rest of `tsgen` is split by concern:
- `operations.go`: the operation mapper, with operation order, method names, path arguments and segments, the `Options` type declaration, request body media type and encoding selection, and security scheme warnings.
- `results.go`: the `Result` type declaration, with the response data types and the success and failure unions.
- `schema.go`: the schema mapper, with schema to `model.Type` mapping, the named types keyed by location (of a referenced schema, or of the shared response or request body around an inline schema), and names for inline types derived from the method name and position or from the shared component.
- `names.go`: word splitting, PascalCase and camelCase, reserved words for path arguments, and the name registry that adds numeric suffixes with a warning.

`internal/tsgen/model/model.go` holds exported types and fields only, with no logic beyond the type-union marker method:

```go
type File struct {
	Info     Info       // raw title, description, version
	Features Features   // methods, query, headers, multipart, FileData flags for imports and helpers
	Types    []TypeDecl // in output order
	Client   Client
	Example  *Example   // nil without --example
}

type TypeDecl struct {
	Name string
	Doc  string // raw text
	Type Type
}

// Type is one of Named, Builtin, Literal, Array, Union, Intersection, Object.
type Type interface{ isType() }

type Named struct{ Name string } // reference to a TypeDecl in this file
type Builtin int                 // String, Number, Boolean, Null, Undefined, Unknown,
                                 // ArrayBuffer, FileData, Params, Response, ResponseBody
type Literal struct{ Value any } // string, float64, or bool; null is the Null builtin
type Array struct{ Elem Type }
type Union []Type
type Intersection []Type
type Object struct {
	Properties []Property // in output order
	Index      Type       // additionalProperties, or nil
}

type Property struct {
	Name     string // raw; render quotes it if it is not an identifier
	Optional bool
	Doc      string
	Type     Type
}

type Client struct {
	Name    string
	Methods []Method
}

type Method struct {
	Name            string
	HTTPMethod      string
	Doc             []string      // raw paragraphs: summary, description
	PathArgs        []PathArg     // in path order
	Path            []PathSegment // literal Text, or a reference to a path Arg
	Options         Type          // Named reference to the method's Options TypeDecl
	OptionsOptional bool
	HasQuery        bool          // pass options?.query to the query helper
	HasHeaders      bool          // pass options?.headers to the header helper
	Body            *Body         // media type and encoding kind; nil if none
	Result          Type          // Named reference to the method's Result TypeDecl
}

type PathArg struct {
	Name string
	Doc  string // raw description of the path parameter
	Type Type
}

type Example struct {
	ClassName string
	BaseURL   string
}
```

The `Options` and `Result` types are ordinary `TypeDecl`s, so `render` needs no knowledge of their shape.
A method needs no per-parameter query or header data, because the generated code passes `options?.query` and `options?.headers` to the runtime helpers as whole objects.

`internal/tsgen/render` embeds `templates/*.tmpl`, parses them once, and exposes `func Render(f *model.File) ([]byte, error)`:
- `render.go`: the embedded templates, the FuncMap, and `Render`.
- `types.go`: prints a `model.Type` as a TypeScript type expression, adding `?` for optional properties, parentheses where precedence needs them (`(A | B)[]`), multi-line object literals with property doc comments and indentation, and the identifiers for builtins (`Params`, `Response["body"]`, `Record<string, unknown>`).
- `syntax.go`: property name quoting, string literals, path template literals, and comment text (see Escaping).
- `templates/client.ts.tmpl` and `templates/example.ts.tmpl` own only the file structure, including that every method is declared `async` with the return type `Promise<{{ .Result }}>`.

Rules for the split:
- `model` holds neither TypeScript syntax nor `openapi3` types.
- `render` makes no decisions and emits no warnings; its template functions panic on an unknown node, which `Render` returns as an error, so golden tests fail instead of output going silently wrong.
- If something changes the meaning of a type or a name, `tsgen` decides it; if it only changes how it is spelled, `render` decides it.
- `tsgen` normalizes enum and `const` values to string, number, boolean, or null, maps anything else to `unknown`, and computes the feature flags for imports and helpers.
- `render` is the only place where text from the descriptor becomes TypeScript source.
- Generated TypeScript uses double quotes everywhere, as Prettier's default and `encoding/json` produce, including in the fixed parts of the templates such as imports.

This split keeps the templates readable as TypeScript, keeps escaping in one place, and lets golden tests pin down decisions and spelling separately from the plumbing.

Example, for this descriptor excerpt:

```yaml
paths:
  /pets/{petId}:
    get:
      operationId: getPet
      parameters:
        - { name: petId, in: path, required: true, schema: { type: integer } }
        - { name: fields, in: query, schema: { type: array, items: { type: string } } }
      responses:
        "200": { content: { application/json: { schema: { $ref: "#/components/schemas/Pet" } } } }
        "404": { description: not found }
components:
  schemas:
    Pet:
      type: object
      required: [name]
      properties:
        name: { type: string, description: The pet's name }
        content-type: { type: string, enum: [a, b], nullable: true }
```

`tsgen` builds this model (abridged):

```go
Types: []model.TypeDecl{
	{Name: "GetPetOptions", Type: model.Object{Properties: []model.Property{
		{Name: "k6Params", Optional: true, Type: model.Params},
		{Name: "query", Optional: true, Type: model.Named{Name: "GetPetQuery"}},
	}}},
	{Name: "GetPetQuery", Type: model.Object{Properties: []model.Property{
		{Name: "fields", Optional: true, Type: model.Array{Elem: model.String}},
	}}},
	{Name: "GetPetResult", Type: model.Union{
		model.Object{Properties: []model.Property{
			{Name: "data", Type: model.Named{Name: "Pet"}},
			{Name: "ok", Type: model.Literal{Value: true}},
			{Name: "response", Type: model.Response},
		}},
		model.Object{Properties: []model.Property{
			{Name: "data", Type: model.Undefined},
			{Name: "ok", Type: model.Literal{Value: false}},
			{Name: "response", Type: model.Response},
		}},
	}},
	{Name: "Pet", Type: model.Object{Properties: []model.Property{
		{Name: "content-type", Optional: true, Type: model.Union{
			model.Literal{Value: "a"}, model.Literal{Value: "b"}, model.Null}},
		{Name: "name", Doc: "The pet's name", Type: model.String},
	}}},
},
Client: model.Client{Name: "PetStoreClient", Methods: []model.Method{{
	Name:            "getPet",
	HTTPMethod:      "GET",
	PathArgs:        []model.PathArg{{Name: "petId", Type: model.Number}},
	Path:            []model.PathSegment{{Text: "/pets/"}, {Arg: "petId"}},
	Options:         model.Named{Name: "GetPetOptions"},
	OptionsOptional: true,
	HasQuery:        true,
	Result:          model.Named{Name: "GetPetResult"},
}}},
```

With the template excerpt

```go-template
{{- range .Types }}
{{ doc .Doc 0 }}export type {{ .Name }} = {{ type .Type 0 }};
{{ end }}
```

`render` prints `Pet` as:

```ts
export type Pet = {
  "content-type"?: "a" | "b" | null;
  /** The pet's name */
  name: string;
};
```

### Asynchronous methods with `http.asyncRequest`
Every generated method is `async`, sends its request with `http.asyncRequest`, and returns `Promise<<Method>Result>`; the client has no synchronous variant.
Scripts call methods with `await`, which k6 supports natively, also in the default function.
k6 promotes async APIs as the default way forward, so the generated client follows that direction instead of building on the blocking API.
k6 resolves the promise with a status-0 response when the request fails and the `throw` option is not set, and rejects it when `throw` is set, which matches the synchronous API's return-or-throw behavior, so the method only `await`s and needs no error handling of its own.
Alternatives considered:
- Synchronous methods with `http.request`.
  Rejected because k6 is moving away from blocking APIs, so the client would start out on the path k6 no longer promotes, and moving to async later would break the whole generated API.
- Both a synchronous and an asynchronous method per operation.
  Rejected because it doubles the generated API and keeps promoting the blocking style that k6 is moving away from.

### Method signature: path arguments plus one options object
Path parameters are positional because they are always required and their order is fixed by the path template.
Everything else (`query`, `headers`, `body`, `k6Params`) goes into one per-operation `<Method>Options` object, and `?` is used only for optional members and for an `options` argument whose members are all optional.
Absence is always `undefined`, which is idiomatic TypeScript, and nothing ever needs a positional `undefined` placeholder.
Adding an optional parameter to the descriptor adds an optional member and does not break existing calls.
The member for k6's `Params` is named `k6Params`, and the constructor option `commonK6Params`, because every other member comes from the descriptor, and a plain `params` is easily confused with the query parameters.
Alternatives considered:
- Positional `query`, `headers`, body, and k6 params, with `?` on trailing optional arguments and `| undefined` before required ones.
  Rejected because methods can have up to four optional arguments, so common calls such as tagging a request need several `undefined` placeholders.
- Positional arguments reordered so that required ones come first, which avoids `| undefined`.
  Rejected because the argument order then depends on which parameters are required, so a descriptor change can silently reorder arguments.
- Path parameters inside the options object as `path`.
  Rejected because it makes the most common call, `client.getPet({ path: { petId: 1 } })`, verbose for no gain.

### Naming and order
- Words are split on non-alphanumeric characters and lower-to-upper case transitions, then joined in PascalCase or camelCase.
- Identifiers that would start with a digit get a `_` prefix; path argument names that are JavaScript reserved words, or that collide with `options` or with the identifiers the generated method body uses (`http` and the locals `headers`, `query`, `url`, `formData`, `body`, and `response`), get a `_` suffix.
- Uniqueness is enforced by one name registry for methods and one for types per output: a colliding name gets the next free numeric suffix and a warning.
  The registries cover generated methods and types only; collisions with imported or built-in identifiers and with the client class's own members are not handled (see Risks).
- Method name suffixes are assigned in operation order, as the `Deterministic output` requirement defines it.
- Type name suffixes are assigned in discovery order: operations in operation order, and within an operation parameters, then request body, then responses, depth-first.
- Type declarations are sorted by name in lexicographic byte order, after all names are final.
- Referenced schemas, from `components.schemas` or an external document, are keyed in the registry by their resolved location, `SchemaRef.RefPath()`, not by name, so two different schemas named `Pet` in different documents become `Pet` and `Pet2`, and the same schema reached through different refs, including a chain of refs, is generated once.
  kin-openapi sets the location for every resolved `$ref`, so a missing one breaks an assumption about the library and panics instead of being reported as a problem of the descriptor.
- The location, not the `*openapi3.Schema` pointer, is the key, because in OpenAPI 3.1 kin-openapi merges the keywords next to a `$ref` into a fresh copy of the referenced schema, so each such use has its own pointer.
  The type is built from the schema of the first use in discovery order, so it includes the keywords next to the first use and ignores those next to later uses (see Risks).
  Ignoring a narrowing keyword such as `enum` makes the type at that use wider than the schema, which never rejects a valid value.
  Alternatives considered: building the type from the original schema in the root document's `components.schemas`, rejected because refs into other documents would still need the first use, and one rule for every ref is simpler; reading kin-openapi's unexported sibling fields with `reflect`, rejected as fragile and still unable to recover the original when every use has siblings; and keeping the widest of several copies, rejected because kin-openapi overwrites fields instead of narrowing, so copies can be incomparable, and it needs a subtype check on model types.
- An inline schema in a response or request body that is referenced from `components` or another document is keyed by the component's `RefPath()`, plus the media type for a response, because a response has one data type per media type and a request body uses only one.
  All uses get one type, named after the component followed by `Response` or `Body`, so the name does not depend on which operation comes first.
  The kind suffix avoids most collisions with `components.schemas`, where a request body and a schema often share a name.
  Other inline schemas in an operation are reached once and are not keyed.
  Alternatives considered: one identical type per position, such as `GetPet404` and `DeletePet404`, rejected because shared error responses are common and callers would have to pick one of many names for the same type; and keying by the schema pointer with the name of the first operation, rejected because the name then depends on operation order and misleads other operations.
- A name is reserved before its body is built, so recursive schemas terminate and reference themselves by name.

### Escaping
All escaping lives in `render/syntax.go`:
- Property names that are not identifiers are quoted.
- String literals (enum values, `const`, quoted property names, header names) are emitted with `encoding/json`, whose output is a valid JavaScript string literal and escapes U+2028 and U+2029.
- Path template literals are written as `` `/pets/${encodeURIComponent(String(petId))}` ``, with `\`, `` ` ``, and `${` escaped in literal segments.
- Each text from the descriptor is collapsed to one line in comments, and `*/` is written as `*\/`.
- A method's doc comment is a multi-line JSDoc block with the summary, the description, and one `@param <argument> <description>` line per described path argument, separated by blank lines; a method with only a summary or only a description gets a single-line doc comment.
  `@param` uses the generated argument name, so editors attach the description to the argument.

### Request building in the generated code
- The request is sent with `await http.asyncRequest(method, url, body, params)`.
- The URL is built as ``new URL(this.cleanBaseUrl + `/pets/${encodeURIComponent(String(petId))}`)``; `URL` comes from jslib.k6.io as specified.
- Imports and helpers are computed by `tsgen` as feature flags in the model, so the template imports `URLSearchParams` only when some operation has query parameters or a URL-encoded body.
- Query strings are built by one private helper on the client class, emitted only when an operation needs it: it creates a `URLSearchParams`, appends each array element as its own pair, skips `undefined` and `null`, and appends `?` plus the result only if it is not empty.
  Passing the object to the `URLSearchParams` constructor is not used, because it would serialize arrays as `a,b`.
- Header arguments go through a second helper that drops `undefined` and `null` and converts values with `String`.
- The query-string helper is also used for `application/x-www-form-urlencoded` bodies, so both follow the same array, `undefined`, and string conversion rules.
  k6's own encoding of object bodies was not used because it cannot send repeated keys for arrays.
- `multipart/form-data` bodies go through a third helper that walks the fields at runtime and appends them to a jslib `FormData`: `FileData` values as they are, `ArrayBuffer` values wrapped with `http.file`, arrays as one part per element, other objects as JSON text, everything else with `String`; the method sends `formData.body()` with `Content-Type: multipart/form-data; boundary=` plus `formData.boundary`.
  Walking fields at runtime keeps the generated method short and also covers `additionalProperties`.
  k6's own multipart encoding was not used because k6 only builds a multipart body when a field is a `FileData`, and sends any other object URL-encoded.
  A hand-generated multipart encoder was rejected because jslib's `FormData` already does this and the client already depends on jslib.
- Other media types pass `body` through unchanged and set `Content-Type` to the declared media type, so the request matches the descriptor without the caller adding headers.
- Each helper and each jslib import is emitted only when an operation needs it.
- Merging uses a private `_mergeK6Params` that spreads `commonK6Params` and `k6Params` and merges `headers`, `cookies`, and `tags` key by key.

### Security schemes: ignored, with a warning where `Params` cannot help
The generator does not use `securitySchemes` or `security` requirements, and the client sends no credentials on its own.
Credentials sent as a header or cookie can be supplied through `headers` or `cookies` in `commonK6Params` or `k6Params`: `http` `basic` and `bearer`, `apiKey` in a header or cookie, and tokens the caller has obtained for `oauth2` or `openIdConnect`.
`http` `digest` works with the credentials percent-encoded in `baseUrl`, as in `http://user:pass@host`, and `auth: "digest"` in `commonK6Params`, because jslib's `URL` keeps the user info and k6 then answers the challenge; k6 supports only the MD5 algorithm, which the descriptor cannot express, so this is documented and not warned about.
`mutualTLS` is configured with k6's `tlsAuth` option, outside the client.
An `apiKey` with `in: query` cannot be supplied, because k6's `Params` has no query member and `query` only holds declared parameters.
Other `http` schemes, such as signature schemes, need more than static credentials.
`tsgen` reports one warning for each such scheme that is referenced by the security requirements of at least one generated operation, in order of scheme name.
An operation's requirements are its own `security` if set, including an empty list, and the document's `security` otherwise; kin-openapi's `*SecurityRequirements` on the operation is `nil` when it is not set.
Unreferenced schemes and schemes that can be supplied produce no warning, because most descriptors declare a bearer or header scheme and a warning on every run would be noise.
Alternatives considered:
- A generic `commonQuery` constructor option.
  Rejected because it adds API surface that the later security-scheme change would likely supersede.
- Generating credential options from `securitySchemes` now.
  Postponed, because alternative requirements (an OR of AND sets), operation-level overrides, and `security: []` need a design of their own.

### Result: success flag instead of per-status variants
Each result is a union of exactly two variants discriminated by the boolean literal `ok`: `if (res.ok)` narrows `data` to exactly the success data type, without the caller listing status codes.
Most operations have one success type and one or two error types spread over several status codes, and load-test code mostly asks whether a request succeeded, so a success flag fits the common case.
The generator collects the data types of the documented responses into the two unions as specified; `unknown` is used only when a side has no documented response or a documented JSON media type has no schema, because a member typed `unknown` collapses the whole union to `unknown`.
For the same reason a response without content has the data type `undefined`, and the runtime sets `data` to `undefined` for an empty or absent body, so `Pet | undefined` stays usable.
After `const response = await http.asyncRequest(...)`, the method returns `{ ok: response.status >= 200 && response.status < 300, data: this._decodeBody(response), response }`, which resolves its promise.
Callers write `const res = await client.getPet(1)` and then `if (res.ok)`.
Alternatives considered:
- A single `data` typed as the first 2xx schema, as is common in generated k6 clients.
  Rejected because it gives error responses the success type.
- One variant per documented status code with a literal `status` discriminant, and a `status: number` variant for `default` and ranges.
  Rejected because `status: number` also matches every documented code, so narrowing on `status === 200` still yields `Pet | Error`.
- The same per-status union with the catch-all typed as `Exclude<HttpStatus, ...>` over a generated list of all status codes.
  Rejected because it needs a large generated status list in every file.
- A per-status union keyed by the matched `responses` key (`200`, `"4XX"`, `"default"`, `"undocumented"`), mapped at runtime.
  Rejected as more complexity than the common case needs; it can be added later as an extra field without breaking `ok`.

### Response decoding by `Content-Type`
The body is decoded according to what the response declares in its `Content-Type` header, never by trying a parser and falling back when it fails.
A client with at least one method has one private `_decodeBody(response)`, which every method uses:
1. An empty or absent body (`null`, `""`, or an empty `ArrayBuffer`, as with `responseType: "none"`, status 204, or `HEAD`) gives `undefined`.
2. The media type is the `Content-Type` value before the first `;`, trimmed and lowercased.
3. `application/json` and media types ending in `+json` give `response.json()`, which also parses `ArrayBuffer` bodies from `responseType: "binary"`.
4. Any other media type, or no `Content-Type`, gives `response.body` unchanged.

If `response.json()` throws, the error propagates out of the `async` method and rejects its promise, so a body that claims to be JSON but is not never reaches the caller under the wrong type.
Supporting another media type later means adding one case to `_decodeBody` and one rule to the type mapping, with no change to the method template or the result shape.
On the type side, a response's data type is the union over its documented media types (the schema type for JSON, `unknown` for JSON without a schema, `Response["body"]` for anything else), so every value the decoder can produce for a documented media type is covered by the type.
Alternatives considered:
- Parsing as JSON and falling back to the raw body when parsing fails, as openapi-to-k6 does.
  Rejected because the result depends on the body's content: a `text/plain` body `123` becomes a number, and an HTML error page silently becomes a string typed as the documented error schema.
- Choosing the decoder from the descriptor per documented status code.
  Rejected because it needs the same status, range, and `default` matching at runtime that the success flag avoids, and it still has no answer for undocumented responses or servers that send another media type than documented.
- Setting `data` to the raw body or to `undefined` when a body declared as JSON does not parse.
  Rejected because it hides a server fault and gives `data` a value outside its type; under load, truncated or malformed bodies should surface as errors.

### Example script
With `--example`, `tsgen` computes the example's base URL and sets `model.Example`: the first `servers` entry with each `{variable}` replaced by its `default`, used only if `net/url` parses it with an `http` or `https` scheme and a host, otherwise `http://localhost`.
`render/templates/example.ts.tmpl` renders an `async` default function that constructs the client with that base URL, so users can add `await client.<method>(...)` calls without changing the function's signature.

### Tests
Following the project's preference for end-to-end tests, all Go tests except the naming edge-case tests below are in package `test` under `test/`, contain only `_test.go` files, and run the command in-process through `command.NewSubcommandWithConfig`:
- `cmd_test.go`: help, arguments, output destination, example, and the shared `execute` helper.
  The helper mirrors k6's root command (output writers set to `ts.Stdout` and `ts.Stderr`, usage and errors silenced) and returns `ts.Stdout`, the messages of the warnings logged to `ts.LoggerHook`, and the error.
- `load_test.go`: loading, load failures, versions, and warnings, with inline fixtures in `ts.FS` and `httptest` servers.
  This includes the external-ref cases and the timeout case with a short `FetchTimeout` against a server that never responds.
- `golden_test.go`: the golden test and the determinism check.

Golden test:
1. For each `.yaml` and `.json` file in `test/testdata/` and its subdirectories, read the file from disk, write it into `ts.FS`, and run the command in-process on it.
2. Compare stdout with `<case>_client.ts` and the logged warning messages, one per line, with `<case>_warnings.txt`, and fail with a diff on any difference.
3. Only with `go test ./test -run TestGolden -update`, write the current output into the golden files instead of comparing.
   The developer then reviews the change with `git diff`.

The fixtures in `test/testdata/` cover each schema mapping, parameter kind, request body kind, result type shape, naming conflict, recursion, escaping, OpenAPI 3.1, and validation warnings.
They are YAML with a comment header naming the requirements they cover.
`test/testdata/corpus/` adds real-world descriptors copied verbatim: the OpenAPI 3.x examples of the OpenAPI Initiative (CC BY 4.0) in YAML, and the example descriptors of grafana/openapi-to-k6 (AGPL-3.0) in JSON, with their origin and license in its `README.md`.
The corpus provides the JSON coverage, because JSON allows no comments for the requirement headers, and kin-openapi decodes JSON and YAML with different decoders.
Every case has a `_warnings.txt`, empty when there are no warnings.
Disk access goes through `fsext.NewOsFs()`, because forbidigo rejects `os.ReadFile` and `os.WriteFile`, also in tests.
`.gitattributes` forces LF in `test/testdata/`, because Git on Windows runners checks out text files with CRLF and k6-ci runs `go test` on Windows.
There are no separate unit tests for type mapping or rendering, since the golden tests exercise them through the command.
The only unit tests are `internal/tsgen/names_test.go`, a table of naming edge cases (acronyms, digits, separators, non-ASCII letters, reserved argument names) that would otherwise each need a fixture; the naming scenarios of the spec are covered by the golden tests.

`make test` runs `go test ./...` unchanged.
`make it` runs `test/it.sh`, which:
1. Builds `build/it/k6` with xk6.
2. Checks that `k6 x --help` lists `openapi`.
3. Generates each fixture in `test/testdata/` and `test/testdata/corpus/` with `--example --out build/it/<case>.ts` and runs it with `k6 run`, proving that every generated variant loads in k6.
   A run fails if k6 exits with a non-zero code or logs an error, because k6 exits with 0 when the default function throws and only logs the exception.

## Risks / Trade-offs

- [The generated client imports from jslib.k6.io, so `k6 run` needs network access, including in `make it` and CI] → Accepted for now as an acknowledged limitation; the imports are pinned to fixed versions, and a later change will remove the network requirement, for example by vendoring or embedding the dependencies.
- [The runtime behavior of the generated client (URLs, encoding, headers, body, merging, result values) is not verified by running it in this change] → `make it` proves that every variant loads; a later change adds a test server and runtime checks.
- [kin-openapi's 3.1 and 3.2 support is partial, so some constructs are silently ignored] → The spec says unrepresented features are ignored; unknown schema types produce a warning; fixtures cover the 3.1 features that are mapped.
- [A Swagger 2.0 document that kin-openapi fails to load as OpenAPI 3 reports the load error instead of "Swagger 2.0 descriptors are not supported"] → The documents tried load without error; `load_test.go` covers a Swagger 2.0 document with `#/definitions` refs.
- [Unrestricted `$ref` access, decision pending] Installing a custom `ReadFromURIFunc` makes kin-openapi skip its own external-ref policy, and this change allows every local and remote ref.
  The command runs with the user's permissions, but the content of a remote descriptor is controlled by whoever controls its URL.
  A malicious or compromised descriptor can therefore make the command read local files through `file:` refs (for example a kubeconfig), whose parsed content can surface in error messages or warnings in k6's log (such as CI logs) or in the generated file; hang the command through refs to files like `/dev/zero` or a FIFO, since the fetch timeout only applies to HTTP; and make requests to internal hosts or cloud metadata endpoints from CI runners.
  This is harmless for trusted descriptors, which is the common case, so this change does not restrict access.
  → Decide in a later change whether to restrict it, for example by rejecting `file:` refs when the root descriptor is remote, before the subcommand is recommended for third-party descriptors.
- [`data` is typed from documented responses only, so an undocumented response, such as a gateway error page or status 0 without a response, gets a wrong type] → Only the type is wrong, never the runtime value; documented in `README.md`. Typing failure data as `unknown` would be honest but would lose the documented error types in the common case.
- [A caller who forgets `await` gets a `Promise`, whose `ok` is `undefined`, so `if (res.ok)` silently takes the failure branch, and k6 does not type-check scripts] → The examples in `README.md` always use `await`, and editors flag the mistake from the `Promise<...>` return type.
- [A body declared as JSON that does not parse rejects the method's promise, which fails the iteration unless the caller catches it] → Intended, because the server violated its own `Content-Type`; the k6 request metrics are still recorded, and callers who expect such bodies can catch the rejection.
- [The success flag does not narrow between several success or several error schemas] → Callers check `response.status` or the shape of `data`; a per-status discriminant can be added later as an extra field.
- [The `FormData` jslib is pinned to `0.0.2`, a pre-1.0 version, and is a second remote import] → It is imported only by clients with multipart bodies, and `make it` checks that such a client loads in k6.
- [Per-field multipart `encoding` (part content types and headers) is ignored] → A warning is reported; callers needing it can pass `FileData` with a content type.
- [A binary string maps to `ArrayBuffer | FileData` everywhere, also in JSON bodies and responses, where the runtime value is neither] → Accepted, so that each schema has one type and the schema mapper needs no knowledge of where a schema is used; binary strings are only meaningful in multipart and raw bodies, and raw bodies do not map their schema.
- [Generated names can collide with identifiers the client library imports or declares itself, for example a schema named `Response` or `Promise`, or an operation named `constructor`] → Accepted as an edge case; a later change can rename or alias them.
- [Warnings are reported as they are found, so a run that later fails, for example while rendering, still logs the warnings found before] → Acceptable, because warnings go to k6's log and never to the output.
- [APIs that require an API key in the query, or an `http` scheme other than `basic`, `bearer`, or `digest`, cannot be called through the generated client] → A warning names each such scheme; `README.md` documents how to send credentials through `Params`; support follows in a later change.
- [The error for an unresolvable `$ref` names only the missing component, for example `Missing`, not the full reference `#/components/schemas/Missing`] → This is caused by a bug in kin-openapi v0.149.0: in `Loader.resolveComponent`, the first lookup builds an error that names the reference, but the retry on a fresh parse of the document runs the same `drill` closure, which assigns to the captured `err` variable and overwrites that error with the bare `map key "Missing" not found`.
  We cannot work around this without finding the broken reference ourselves, which would duplicate kin-openapi's ref resolution, so the `Unresolvable reference` scenario only requires the error to name the missing component.
  Once kin-openapi fixes the bug, the scenario can be tightened again to require the full reference.
- [In OpenAPI 3.1, a named type is built from the first use of its schema, so the keywords next to that use's `$ref` apply to every use: a sibling `description` becomes the type's doc, and a sibling such as `enum` narrows the type everywhere; which use is first depends on discovery order] → Accepted as a known limitation, because kin-openapi v0.149.0 exposes neither the sibling keywords nor the schema without them. Removing it needs kin-openapi to expose the siblings of a `$ref` or to offer a loader option that does not merge them, which can be requested upstream.
- [`const: null` is ignored, because kin-openapi v0.149.0 stores a missing `const` and `const: null` the same way, as `nil`] → Accepted as a known limitation; such a schema maps by its other keywords.
- [kin-openapi v0.149.0 validation reports some valid constructs as errors, for example the OpenAPI 3.2 tag fields `kind`, `parent`, and `summary`, and `http` scheme names it does not know or that are not lowercase, such as `Basic`, `Bearer`, or `signature`, so such descriptors produce `invalid descriptor` warnings] → Accepted, because validation only produces warnings and never blocks generation; the corpus golden files record the affected cases.
- [kin-openapi API changes] → The version is pinned in `go.mod`; breaking changes fail at compile time or in the golden tests.

## Open Questions

**These problems were found during implementation and are not decided yet.**
**They are accepted as known limitations of this change, listed under `Open problems` in the `generated-client` spec, and will be resolved in separate changes.**

### 1. Object query parameters are sent as `[object Object]`
A query parameter with an object schema, such as `filter` in `test/testdata/schemas.yaml`, is typed correctly as `filter?: { color?: string }`.
The query helper converts each value with `String(item)`, so `{ filter: { color: "red" } }` is sent as `?filter=%5Bobject+Object%5D`.
The same helper encodes `application/x-www-form-urlencoded` bodies, so an object field of a form body has the same problem.
The `Request building` requirement says nothing about object values.
Options:
- (a) Serialize objects as OpenAPI's default `style: form` with `explode: true` does, one `name=value` pair per property, so `{ color: "red" }` gives `color=red`, and warn about nested objects and `deepObject`.
- (b) Keep the current serialization and warn about object-valued query parameters.
- (c) Document it as a limitation.

Recommendation: (a).
Still open for (a): whether an object field of a form body is flattened like a query parameter or sent as `JSON.stringify` text like a multipart part.
Needs a rule in the `Request building` requirement, a note under `Request building in the generated code`, and a fixture case.

### 2. `+json` request bodies are sent raw
Only `application/json` request bodies are encoded with `JSON.stringify` and typed from their schema.
Every other media type, including `application/merge-patch+json`, `application/json-patch+json`, and `application/vnd.api+json`, is a raw body typed `string | ArrayBuffer`, as `patchPet` in `test/testdata/request_bodies.yaml` shows, so callers must stringify the body themselves and get no type for its content.
This is inconsistent with responses, where media types ending in `+json` are parsed as JSON and typed from their schema.
Related: `updatePet` in the same fixture declares `application/json; charset=utf-8` and is encoded as JSON, but is sent with `Content-Type: application/json`, so the declared `charset` is dropped.

Option: treat `application/json` and media types ending in `+json` as JSON request bodies, typed from the schema, encoded with `JSON.stringify`, and sent with the declared media type as `Content-Type`.
Still open: whether the declared parameters such as `charset` are kept in the `Content-Type`, which applies to `application/json` too, and whether `application/json` is still preferred over a `+json` media type when a request body offers both.
Recommendation: do it.
Needs changes to the `Method arguments` and `Request building` requirements, to the `Media type preference` scenario if the preference changes, and to the design's request body notes.

### 3. Smaller items
- A single `type: file` gives three warnings, two from validation and one from the generator, and the generator's schema warnings name only the enclosing type, such as "a schema in type Misc", not the property (`test/testdata/schemas_warnings.txt`).
- The `Schema` fallback name for a referenced schema whose name has no letters or digits, such as `#/components/schemas/---` in `test/testdata/schemas.yaml`, is tested but not specified; the spec only specifies the `ApiClient` fallback for the class name.
- `*/` is escaped as `*\/` in the `//` header comments too, for example `0.1.0-beta *\/` in `test/testdata/no_operations_client.ts`, although `*/` cannot end a line comment; this follows the `Escaping` rule, which applies to all comments.
- In a client without operations, `cleanBaseUrl` and `commonK6Params` are never read, so `tsc --noUnusedLocals` reports them (`test/testdata/no_operations_client.ts`); the client passes `tsc --strict`.

## Migration Plan

There is no release yet, so the breaking changes to the default output, the example script, the handling of a missing descriptor, and the move of the command into `internal/command` need no migration.
