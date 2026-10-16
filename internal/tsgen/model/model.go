// Package model is the data contract between the generator, which decides what the client library contains,
// and the renderer, which spells it as TypeScript.
// It holds no TypeScript syntax, no OpenAPI types, and no logic beyond the Type marker method.
// All text from the descriptor is raw; the renderer escapes it.
package model

// File is a complete client library, optionally followed by an example script.
type File struct {
	Info     Info
	Features Features
	Types    []TypeDecl // in output order
	Client   Client
	Example  *Example // nil without --example
}

// Info is the descriptor's info object, for the header comment.
type Info struct {
	Title       string
	Description string
	Version     string
}

// Features tells which optional imports and private helper methods the client library needs.
type Features struct {
	// Methods: the client has methods, which use the http module, the Response type, URL,
	// and the helpers that merge k6Params and decode response bodies.
	Methods bool
	// Query: URLSearchParams and the helper that builds a query string, also used for URL-encoded bodies.
	Query bool
	// Headers: the helper that converts header parameters to request headers.
	Headers bool
	// Multipart: FormData and the helper that builds a multipart body.
	Multipart bool
	// FileData: the FileData type, which a generated type uses.
	FileData bool
}

// TypeDecl is an exported type declaration.
type TypeDecl struct {
	Name string
	Doc  string
	Type Type
}

// Type is one of Named, Builtin, Literal, Array, Union, Intersection, Exclude, or Object.
type Type interface{ isType() }

// Named references the TypeDecl with this name.
type Named struct{ Name string }

// Builtin is a type that the renderer spells with a fixed identifier or expression.
type Builtin int

// The builtin types.
const (
	String Builtin = iota + 1
	Number
	Boolean
	Null
	Undefined
	Unknown
	ArrayBuffer
	FileData
	Params       // the k6 request params
	Response     // the k6 response
	ResponseBody // the type of the k6 response's body
	Record       // an object with arbitrary keys and unknown values
)

// Literal is a literal type. Value is a string, a float64, or a bool; null is the Null builtin.
type Literal struct{ Value any }

// Array is an array of Elem.
type Array struct{ Elem Type }

// Union is a union of its members.
type Union []Type

// Intersection is an intersection of its members.
type Intersection []Type

// Exclude is the members of the union From that are not assignable to Excluded.
type Exclude struct {
	From     Type
	Excluded Type
}

// Object is an object type.
type Object struct {
	Properties []Property // in output order
	Index      Type       // the type of an index signature, or nil for none
	// Additional is the type of additionalProperties, or nil for none. Index also includes
	// the types of the properties, because a TypeScript index signature covers them.
	Additional Type
}

// Property is a property of an Object.
type Property struct {
	Name     string // raw; the renderer quotes it if it is not an identifier
	Optional bool
	Doc      string
	Type     Type
}

func (Named) isType()        {}
func (Builtin) isType()      {}
func (Literal) isType()      {}
func (Array) isType()        {}
func (Union) isType()        {}
func (Intersection) isType() {}
func (Exclude) isType()      {}
func (Object) isType()       {}

// Client is the client class.
type Client struct {
	Name    string
	Methods []Method // in output order
}

// Method is a client method for one operation.
type Method struct {
	Name            string
	HTTPMethod      string        // uppercase, as passed to http.asyncRequest
	Doc             []string      // raw paragraphs: the summary and the description; the renderer skips blank ones
	PathArgs        []PathArg     // in path order
	Path            []PathSegment // the path template, without the base URL
	Options         Type          // Named reference to the method's Options TypeDecl
	OptionsOptional bool
	HasQuery        bool  // pass options?.query to the query helper
	HasHeaders      bool  // pass options?.headers to the header helper
	Body            *Body // nil if the operation has no request body
	Result          Type  // Named reference to the method's Result TypeDecl
}

// PathArg is a positional argument for a path parameter.
type PathArg struct {
	Name string // a valid, unique parameter name
	Doc  string // raw text
	Type Type
}

// PathSegment is either literal Text or a reference to the PathArg named Arg.
type PathSegment struct {
	Text string
	Arg  string
}

// Body describes how a method encodes its request body.
type Body struct {
	Kind BodyKind
	// ContentType is the Content-Type header the method sets.
	// It is empty for multipart bodies, whose header carries a boundary chosen at runtime,
	// and for media types that contain a wildcard.
	ContentType string
}

// BodyKind is the way a request body is encoded. The renderer compares it with these values.
type BodyKind string

// The request body encodings.
const (
	JSONBody      BodyKind = "json"      // JSON.stringify
	FormBody      BodyKind = "form"      // URL-encoded with the query helper
	MultipartBody BodyKind = "multipart" // multipart with the multipart helper
	RawBody       BodyKind = "raw"       // passed to http.asyncRequest unchanged
)

// Example is the example script appended with --example.
type Example struct {
	ClassName string
	BaseURL   string
}
