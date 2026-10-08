# Spec Delta

## Purpose

Defines the TypeScript client library that `k6 x openapi` generates from an OpenAPI descriptor: its types, its client class and its asynchronous methods, how methods build HTTP requests, and the typed results their promises resolve to.
The runtime behavior in the `Request building` and `Method result` requirements is not yet verified by running a generated client in k6; that verification follows in a later change.
**Open problems:** the design's `Open Questions` section lists unresolved problems that can change the `Request building`, `Method arguments`, `Schema types`, and `Warnings` behavior specified here, such as object-valued query parameters and `+json` request bodies; they MUST be resolved before this change is archived.
The client library imports its runtime dependencies from fixed jslib.k6.io URLs, so running it needs network access to jslib.k6.io; this is an acknowledged limitation that a later change will remove, for example by making the dependencies available offline.

## ADDED Requirements

### Requirement: Client library file
The client library SHALL be a single TypeScript module that k6 can run without a build step.
It SHALL start with a comment stating that the file is generated and must not be edited by hand, followed by the descriptor's `info.title`, `info.description`, and `info.version` when present.
It SHALL import the `Params` type from `k6/http`.
It SHALL import `http` and the `Response` type from `k6/http`, and `URL` from `https://jslib.k6.io/url/1.0.0/index.js`, if, and only if, the descriptor has at least one operation.
It SHALL also import `URLSearchParams` from the same module if, and only if, at least one operation has query parameters or an `application/x-www-form-urlencoded` request body.
It SHALL import `FormData` from `https://jslib.k6.io/formdata/0.0.2/index.js` if, and only if, at least one operation has a `multipart/form-data` request body.
It SHALL import the `FileData` type from `k6/http` if, and only if, a generated type uses it.
It SHALL NOT import anything it does not use.
It SHALL export the generated types and exactly one client class, and SHALL NOT have a default export.

#### Scenario: Header and imports
- **WHEN** the descriptor has `info.title: Pet Store`, `info.version: 1.2.0`, and at least one operation
- **THEN** the client library starts with a comment containing `Pet Store` and `1.2.0` and stating that the file is generated
- **AND** it imports from `k6/http` and from `https://jslib.k6.io/url/1.0.0/index.js`

#### Scenario: Imports without query parameters
- **WHEN** the descriptor has operations, but none of them has query parameters or a URL-encoded or multipart request body
- **THEN** the client library imports `URL` but not `URLSearchParams` or `FormData`

#### Scenario: Imports without operations
- **WHEN** the descriptor has no operations
- **THEN** the client library imports only the `Params` type from `k6/http`

#### Scenario: Imports with query parameters
- **WHEN** at least one operation has a query parameter
- **THEN** the client library imports both `URL` and `URLSearchParams`

#### Scenario: Imports with a multipart body
- **WHEN** at least one operation has a `multipart/form-data` request body
- **THEN** the client library imports `FormData` from `https://jslib.k6.io/formdata/0.0.2/index.js`

#### Scenario: Client library loads in k6
- **WHEN** a k6 script consists of a generated client library followed by a default function that constructs its client
- **THEN** `k6 run` on that script exits with exit code 0

### Requirement: Deterministic output
Generating a client from the same descriptor SHALL always produce byte-identical output.
Operations SHALL be ordered by path in lexicographic byte order, and within a path by HTTP method in the order GET, PUT, POST, DELETE, OPTIONS, HEAD, PATCH, TRACE, followed by any other methods in lexicographic order.
Object properties and the fields of parameter types SHALL be ordered lexicographically by name.

#### Scenario: Repeated generation
- **WHEN** the user generates a client from the same descriptor twice
- **THEN** both outputs are byte-identical

#### Scenario: Operation order
- **WHEN** the descriptor defines `POST /pets`, `GET /pets`, and `GET /owners`
- **THEN** the client methods appear in the order `GET /owners`, `GET /pets`, `POST /pets`

### Requirement: Client class
The client class SHALL be named after `info.title` in PascalCase followed by `Client`, or `ApiClient` if the title is empty or yields no letters or digits.
A name that would start with a digit SHALL be prefixed with `_`.
The constructor SHALL take an object with a required `baseUrl` string and optional `commonK6Params` of type `Params`.
Trailing slashes SHALL be removed from `baseUrl`, and `commonK6Params` SHALL apply to every request the client makes.

#### Scenario: Class name from title
- **WHEN** the descriptor has `info.title: pet store api`
- **THEN** the client library exports a class named `PetStoreApiClient`

#### Scenario: Base URL with trailing slash
- **WHEN** a client is constructed with `baseUrl: "https://api.example.test/v1/"`
- **AND** a method for `GET /pets` is called
- **THEN** the request URL is `https://api.example.test/v1/pets`

### Requirement: Client methods
The client class SHALL have one method per operation in the descriptor, including operations with HTTP methods other than the eight standard ones.
A method SHALL be named after the operation's `operationId` in camelCase, or, if the operation has no `operationId`, after the lowercase HTTP method followed by the path in PascalCase.
If two operations would get the same method name, the later one in operation order SHALL get the suffix `2`, the next one `3`, and so on, and a warning SHALL be reported.
Method names SHALL be made unique only among the client methods.
A collision of a method name with a member that the client class declares itself, such as `constructor`, is not handled, and the resulting client library may be invalid.
A method SHALL carry a doc comment with the operation's `summary` and `description` when present, as separate paragraphs, and with an `@param` entry for each path argument whose parameter has a description, naming the argument.
Every method SHALL be an `async` method that returns a `Promise`, as specified in the `Method result` requirement.

#### Scenario: Method from operationId
- **WHEN** the operation `GET /pets/{petId}` has `operationId: get-pet-by-id`
- **THEN** the client class has a method `getPetById`

#### Scenario: Method without operationId
- **WHEN** the operation `GET /pets/{petId}` has no `operationId`
- **THEN** the client class has a method `getPetsPetId`

#### Scenario: Method doc comment
- **WHEN** the operation `GET /pets/{pet-id}` has `summary: Get a pet`, `description: Returns a pet.`, and the path parameter `pet-id` has `description: The pet's ID.`
- **THEN** the method's doc comment contains `Get a pet` and `Returns a pet.` as separate paragraphs and the line `@param petId The pet's ID.`

#### Scenario: Conflicting method names
- **WHEN** `GET /pets` has `operationId: listPets` and `GET /animals` has `operationId: list-pets`
- **THEN** the method for `GET /animals` is named `listPets` and the method for `GET /pets` is named `listPets2`
- **AND** a warning is reported

### Requirement: Method arguments
A method SHALL take one positional argument per path parameter, in the order the parameters appear in the path template, named after the parameter in camelCase, followed by one `options` argument.
Path arguments SHALL always be required.
`options` SHALL be of an exported type named after the method in PascalCase followed by `Options`, with these members:
- `query`: an object with one field per query parameter, present only if the operation has query parameters.
- `headers`: an object with one field per header parameter, present only if the operation has header parameters.
- `body`: the request body, present only if the operation has a request body.
  It SHALL be of the type of the request body's schema if the chosen media type is `application/json`, `application/x-www-form-urlencoded`, or `multipart/form-data`, and `string | ArrayBuffer` for any other media type, which the method sends unchanged.
- `k6Params`: k6 `Params` for this request, always present.

`query` and `headers` SHALL be required members if at least one of their parameters is marked `required`, and optional members otherwise.
`body` SHALL be a required member if the request body is marked `required`, and an optional member otherwise.
`k6Params` SHALL always be an optional member.
The `options` argument SHALL be optional if all its members are optional, and required otherwise.
Optional arguments and optional members SHALL be declared with `?`, and a value of `undefined` SHALL mean that the argument or member is absent.
The fields of `query` and `headers` SHALL be optional unless the parameter is marked `required`.
Parameters defined on the path item SHALL apply to each of its operations unless the operation defines a parameter with the same name and location.
A name placeholder in the path template that has no matching path parameter SHALL become a `string` path argument, and a warning SHALL be reported.
Cookie parameters SHALL be skipped with a warning.

#### Scenario: Path, query, and header parameters
- **WHEN** the operation `GET /pets/{pet-id}` with `operationId: getPet` has a path parameter `pet-id` of type integer, an optional query parameter `fields`, and an optional header parameter `X-Trace`
- **THEN** the method signature is `getPet(petId: number, options?: GetPetOptions)`
- **AND** `GetPetOptions` has the optional members `query`, `headers`, and `k6Params`
- **AND** `client.getPet(1)` is a valid call

#### Scenario: Required query parameter
- **WHEN** the operation `GET /pets` with `operationId: listPets` has a required query parameter `limit` and an optional query parameter `offset`
- **THEN** the method signature is `listPets(options: ListPetsOptions)`, and `ListPetsOptions` has the required member `query` and the optional member `k6Params`

#### Scenario: Required request body
- **WHEN** the operation `POST /pets` with `operationId: createPet` has an optional query parameter `dryRun` and a required request body with schema `Pet`
- **THEN** the method signature is `createPet(options: CreatePetOptions)`, and `CreatePetOptions` has the optional member `query`, the required member `body` of type `Pet`, and the optional member `k6Params`
- **AND** `client.createPet({ body: { name: "Rex" } })` sends the request without a query string

#### Scenario: Optional request body
- **WHEN** the operation `PUT /pets/{petId}` with `operationId: updatePet` has a request body with schema `Pet` that is not marked `required`
- **THEN** the method signature is `updatePet(petId: number, options?: UpdatePetOptions)`
- **AND** calling `client.updatePet(1)` sends the request without a body and without a `Content-Type` header

#### Scenario: k6 params without other options
- **WHEN** a method for an operation with optional query and header parameters is called with only `{ k6Params: { tags: { name: "x" } } }`
- **THEN** the request has the tag `name: x`, no query string, and no header parameters

#### Scenario: Parameters from the path item
- **WHEN** the path item `/pets/{petId}` defines the path parameter `petId`, and its `GET` and `DELETE` operations do not
- **THEN** both generated methods take a `petId` argument

#### Scenario: Undeclared path parameter
- **WHEN** the path is `/pets/{petId}` and the operation defines no `petId` path parameter
- **THEN** the method takes a `petId: string` argument and a warning is reported

#### Scenario: Cookie parameter
- **WHEN** an operation has a cookie parameter `session`
- **THEN** the method has no argument for `session` and a warning is reported

### Requirement: Request building
A method SHALL send exactly one request with k6's `http.asyncRequest`, using the operation's HTTP method, and SHALL NOT use the blocking `http.request`.
The request URL SHALL be the base URL followed by the path, with each path placeholder replaced by the path argument value encoded with `encodeURIComponent`.
Query parameters SHALL be appended as a query string, which SHALL be left out if no query parameter has a value: array values SHALL be sent as one `name=value` pair per element, and fields whose value is `undefined` or `null` SHALL be omitted.
A query parameter that declares a `style` other than `form` or `explode: false` SHALL be serialized the same way, and a warning SHALL be reported.
Header parameters whose value is not `undefined` or `null` SHALL be sent as request headers, converted to strings.
If the request body offers an `application/json` media type, the generator SHALL use it; otherwise it SHALL use the request body's first media type in lexicographic order.
The method SHALL encode `body` according to the chosen media type:
- `application/json`: `JSON.stringify` of `body`, with `Content-Type: application/json`.
- `application/x-www-form-urlencoded`: a URL-encoded string built from the fields of `body` by the same rules as the query string, with `Content-Type: application/x-www-form-urlencoded`.
- `multipart/form-data`: a multipart body with one part per field of `body`, with `Content-Type: multipart/form-data` and the body's boundary.
  Fields whose value is `undefined` or `null` SHALL be omitted, array values SHALL be sent as one part per element, `FileData` and `ArrayBuffer` values SHALL be sent as file parts, other objects SHALL be sent as their `JSON.stringify` text, and all other values SHALL be converted to strings.
  A request body that declares a per-field `encoding` SHALL be encoded the same way, and a warning SHALL be reported.
- Any other media type: `body` passed to `http.asyncRequest` unchanged, with `Content-Type` set to the media type unless it contains a wildcard (`*`).

If `body` is absent, the request SHALL have no body and the method SHALL NOT set `Content-Type`.
The k6 request params SHALL be `commonK6Params` merged with `k6Params`, where `k6Params` wins; `headers`, `cookies`, and `tags` SHALL be merged key by key.
Header parameters and the `Content-Type` set by the method SHALL take precedence over headers from the merged k6 request params.

#### Scenario: Path parameter encoding
- **WHEN** the method for `GET /files/{name}` is called with `name` set to `a/b c`
- **THEN** the request URL path ends with `/files/a%2Fb%20c`

#### Scenario: Array query parameter
- **WHEN** the method for `GET /pets` is called with `query` set to `{ tags: ["a", "b"], limit: undefined }`
- **THEN** the request URL ends with `/pets?tags=a&tags=b`

#### Scenario: JSON request body
- **WHEN** an operation's request body has the media type `application/json`
- **AND** the method is called with the body `{ name: "Rex" }`
- **THEN** the request body is `{"name":"Rex"}` and the `Content-Type` header is `application/json`

#### Scenario: URL-encoded request body
- **WHEN** an operation's only request body media type is `application/x-www-form-urlencoded`
- **AND** the method is called with the body `{ name: "Rex", tags: ["a", "b"], age: 3, note: undefined }`
- **THEN** the request body is `name=Rex&tags=a&tags=b&age=3` and the `Content-Type` header is `application/x-www-form-urlencoded`

#### Scenario: Multipart request body
- **WHEN** an operation's only request body media type is `multipart/form-data` with a string property `description` and a binary property `file`
- **AND** the method is called with the body `{ description: "photo", file: http.file(data, "a.png", "image/png") }`
- **THEN** the request is a multipart request whose `Content-Type` header is `multipart/form-data` with a boundary
- **AND** it has a part `description` with the value `photo` and a file part `file` with the filename `a.png` and the content type `image/png`

#### Scenario: Other request body media type
- **WHEN** an operation's only request body media type is `text/plain`
- **AND** the method is called with the body `hello`
- **THEN** the request body is `hello` and the `Content-Type` header is `text/plain`

#### Scenario: Media type preference
- **WHEN** an operation's request body offers `multipart/form-data`, `application/x-www-form-urlencoded`, and `application/json`
- **THEN** the method encodes the body as JSON

#### Scenario: Merged request parameters
- **WHEN** the client is constructed with `commonK6Params` containing the header `A: 1` and the tag `t: x`
- **AND** a method is called with `k6Params` containing the header `B: 2`
- **THEN** the request has the headers `A: 1` and `B: 2` and the tag `t: x`

### Requirement: Security schemes
The descriptor's `securitySchemes` and `security` requirements SHALL NOT change the generated types, methods, or requests, and the generated client SHALL NOT send credentials on its own.
An operation's security requirements SHALL be its own `security` if present, and the document's `security` otherwise.
A warning SHALL be reported for each security scheme that is referenced by the security requirements of at least one generated operation and is either an `apiKey` scheme with `in: query` or an `http` scheme whose `scheme` is not `basic`, `bearer`, or `digest`, compared case-insensitively.
These warnings SHALL be reported once per scheme, in order of scheme name.

#### Scenario: Bearer scheme
- **WHEN** the document's `security` references the `http` scheme `bearerAuth` with `scheme: bearer`
- **THEN** no warning is reported for `bearerAuth`
- **AND** a client constructed with `commonK6Params` containing the header `Authorization: Bearer t` sends that header with every request

#### Scenario: Query API key
- **WHEN** an operation's `security` references the `apiKey` scheme `api_key` with `in: query`
- **THEN** the client library is generated, the method has no argument or member for `api_key`, and a warning that names `api_key` is reported

#### Scenario: Unreferenced scheme
- **WHEN** the document declares the `apiKey` scheme `api_key` with `in: query`, the document's `security` references it, and every operation sets `security: []`
- **THEN** no warning is reported for `api_key`

### Requirement: Method result
A method SHALL return a `Promise` that resolves, once the response has been received, to an object with the fields `ok`, `data`, and `response`.
If `http.asyncRequest` rejects, for example because the k6 `throw` option is set and the request fails, the returned promise SHALL reject with the same error.
`response` SHALL be the k6 `Response`.
`ok` SHALL be `true` if `response.status` is between 200 and 299, and `false` otherwise, including status 0 when k6 received no response.
`data` SHALL be `undefined` if the response body is empty or absent.
Otherwise `data` SHALL be decoded according to the media type in the response's `Content-Type` header, compared case-insensitively and without parameters: parsed as JSON for `application/json` and media types ending in `+json`, and the unparsed response body for any other media type or a missing `Content-Type` header.
Whether the body is parsed as JSON SHALL depend only on the `Content-Type` header, not on whether parsing succeeds.
If a body declared as JSON cannot be parsed, the returned promise SHALL reject with the parse error.
The method's return type SHALL be `Promise<R>`, where `R` is an exported type named after the method in PascalCase followed by `Result`, defined as the union of a success variant `{ ok: true; data: <success data>; response: Response }` and a failure variant `{ ok: false; data: <failure data>; response: Response }`.
The success data type SHALL be the union of the data types of the documented `2XX` range and the documented status codes from 200 to 299.
The failure data type SHALL be the union of the data types of all other documented responses, including `default`.
If the operation documents no success response, the success data type SHALL be the data type of the `default` response if one is documented, and `unknown` otherwise.
If the operation documents no failure response, the failure data type SHALL be `unknown`.
The data type of a single response SHALL be `undefined` if it has no content, and otherwise the union of the data types of its media types, each type included once: the type of the schema for a JSON media type (`application/json` or a media type ending in `+json`) with a schema, `unknown` for a JSON media type without a schema, and `Response["body"]` for any other media type.
These types describe documented responses only; for an undocumented response, or a response whose `Content-Type` is not documented for its status, `data` holds whatever the rules above produce at runtime.

#### Scenario: Narrowing by success
- **WHEN** `GET /pets/{petId}` with `operationId: getPet` documents a `200` response with schema `Pet`, a `404` response with schema `NotFound`, and a `default` response with schema `Error`
- **THEN** the client library exports a type `GetPetResult` that is the union of a success variant `{ ok: true; data: Pet; response: Response }` and a failure variant `{ ok: false; data: NotFound | Error; response: Response }`, regardless of how the type is formatted
- **AND** `getPet` returns `Promise<GetPetResult>`
- **AND** after `const res = await client.getPet(1)`, checking `res.ok` narrows `res.data` to `Pet` or to `NotFound | Error`

#### Scenario: Several success responses
- **WHEN** an operation documents a `200` response with schema `Pet` and a `202` response with schema `Job`
- **THEN** the success variant's `data` type is `Pet | Job`

#### Scenario: Response without content
- **WHEN** an operation documents a `200` response with schema `Pet` and a `204` response without content
- **THEN** the success variant's `data` type is `Pet | undefined`
- **AND** when the server responds with status 204 and an empty body, the method's promise resolves with `data` set to `undefined`

#### Scenario: Several media types
- **WHEN** an operation documents a `200` response with the media type `application/json` with schema `Pet` and the media type `text/plain`
- **THEN** the success variant's `data` type is `Pet | Response["body"]`

#### Scenario: Only a default response
- **WHEN** an operation documents only a `default` response with schema `Pet`
- **THEN** both the success and the failure variant have the `data` type `Pet`

#### Scenario: No documented failure
- **WHEN** an operation documents only a `200` response with schema `Pet`
- **THEN** the failure variant's `data` type is `unknown`

#### Scenario: Runtime failure result
- **WHEN** a method is called and the server responds with status 404, the header `Content-Type: application/json`, and the body `{"message":"not found"}`
- **THEN** the method's promise resolves to an object whose `ok` is `false`, whose `data` is the parsed object, and whose `response.status` is `404`

#### Scenario: JSON media type with parameters
- **WHEN** the server responds with the header `Content-Type: application/problem+json; charset=utf-8` and the body `{"title":"bad"}`
- **THEN** `data` is the parsed object

#### Scenario: Non-JSON media type
- **WHEN** the server responds with the header `Content-Type: text/plain` and the body `123`
- **THEN** `data` is the string `123`, not the number `123`

#### Scenario: Missing content type
- **WHEN** the server responds without a `Content-Type` header and with the body `{"name":"Rex"}`
- **THEN** `data` is the unparsed body string

#### Scenario: Invalid JSON body
- **WHEN** the server responds with the header `Content-Type: application/json` and the body `<html>`
- **THEN** the method's promise rejects with the JSON parse error

#### Scenario: No response
- **WHEN** a method is called without the k6 `throw` option and k6 cannot connect to the server
- **THEN** the method's promise resolves to an object whose `ok` is `false` and whose `response.status` is `0`

#### Scenario: Rejected request
- **WHEN** a method is called with the k6 `throw` option set and k6 cannot connect to the server
- **THEN** the method's promise rejects with the error from `http.asyncRequest`

#### Scenario: Concurrent requests
- **WHEN** a script calls `await Promise.all([client.getPet(1), client.getPet(2)])`
- **THEN** both requests are sent without waiting for each other's response, and the result is an array of two `GetPetResult` values

### Requirement: Schema types
Each schema referenced from `components.schemas` or from an external document SHALL be exported as a named type, named after the schema in PascalCase.
The schema's name SHALL be the last segment of the reference's JSON pointer, or, for a `$ref` without a fragment such as `pet.yaml`, the referenced file name without its extension.
Inline object, `allOf`, `oneOf`, and `anyOf` schemas used by parameters, request bodies, or responses SHALL be exported as named types whose names are derived from the method name and their position, for example `GetPetQuery`, `GetPetHeaders`, `CreatePetBody`, and `GetPet200`.
An inline object or composition schema in a response or request body that operations reference from `components`, or from an external document, SHALL instead be exported as one named type for all uses, named after the response or request body in PascalCase followed by `Response` or `Body`, for example `NotFoundResponse` and `NewPetBody`; a response gets one such type for each JSON media type.
Inline schemas nested in other schemas, including the members of `allOf`, `oneOf`, and `anyOf`, SHALL be emitted inline and SHALL NOT become named types.
If two types would get the same name, the later one SHALL get a numeric suffix and a warning SHALL be reported.
Type names SHALL be made unique only among the generated types.
A collision of a type name with an identifier that the client library imports or otherwise uses, such as `Response`, `Params`, `Record`, or `Promise`, or with the client class name, is not handled, and the resulting client library may be invalid.
Only types reachable from the operations SHALL be generated.
Schemas SHALL map to TypeScript types as follows:
- `string` maps to `string`, and `string` with `format: binary` to `ArrayBuffer | FileData` wherever it is used, so callers can pass `http.file(...)` with a filename and content type for a part of a `multipart/form-data` request body.
- `integer` and `number` map to `number`, and `boolean` maps to `boolean`.
- `null`, or `nullable: true` in OpenAPI 3.0, adds `null` to the type.
- A list of types maps to the union of the mapped types.
- `enum` maps to a union of the literal values, and `const` to the literal value.
- `array` maps to an array of the mapped `items` type.
- `object` with `properties` maps to an object type whose properties are optional unless listed in `required`; `additionalProperties` adds an index signature of the mapped type, or `unknown` if it is `true`.
  Because a TypeScript index signature also covers the named properties, an index signature on an object with properties SHALL be the union of the mapped `additionalProperties` type, the type of each property, and `undefined` if any property is optional, unless it is `unknown`.
- `object` with neither `properties` nor `additionalProperties` maps to `Record<string, unknown>`.
- `allOf` maps to the intersection of the mapped types, and `oneOf` and `anyOf` map to their union.
- A schema without a `type` maps as an `object` if it has `properties` or `additionalProperties` and no `items`, and as an `array` if it has `items` and neither `properties` nor `additionalProperties`.
  A schema without a `type` that has both maps to `unknown`, and a warning SHALL be reported that its type could not be inferred.
- Any other schema without a type, and any construct not listed here, maps to `unknown`, and a warning SHALL be reported for an unrecognized `type` value.

Property names that are not valid identifiers SHALL be quoted.
Recursive schemas SHALL be generated as named types that reference themselves.
Every reference to the same schema SHALL map to the same named type, even if keywords next to a `$ref`, which OpenAPI 3.1 allows, differ between the references.
The named type SHALL be built from the first reference in discovery order, including the keywords next to it, and the keywords next to later references SHALL be ignored.
Descriptions of properties and parameters SHALL be emitted as doc comments.
Each text from the descriptor SHALL be written into comments on a single line and SHALL never end a comment early.

#### Scenario: Component schema
- **WHEN** a response references `#/components/schemas/pet` with required property `name` of type string and optional property `age` of type integer
- **THEN** the client library exports `type Pet` with the properties `age?: number` and `name: string`

#### Scenario: Composition
- **WHEN** schema `Pet` is `oneOf` `Cat` and `Dog`, and `Cat` is `allOf` `PetBase` and an inline object
- **THEN** `Pet` is the union of `Cat` and `Dog`, and `Cat` is the intersection of `PetBase` and an inline object type

#### Scenario: Shared response component
- **WHEN** `GET /pets` with `operationId: getPet` and `DELETE /pets` with `operationId: deletePet` both document a `404` response with `$ref: '#/components/responses/NotFound'`, whose schema is an inline object
- **THEN** the client library exports one type `NotFoundResponse`, the failure data type of both `GetPetResult` and `DeletePetResult` is `NotFoundResponse`, and no warning is reported

#### Scenario: Binary property
- **WHEN** schema `Upload` has a binary property `file`, and is used both as a `multipart/form-data` request body and as a JSON response
- **THEN** the client library exports one type `Upload` with the property `file: ArrayBuffer | FileData`, and both uses reference it

#### Scenario: Recursive schema
- **WHEN** schema `Node` has a property `children` that is an array of `Node`
- **THEN** the client library exports `type Node` whose `children` property has type `Node[]`, and generation terminates

#### Scenario: Keywords next to a reference
- **WHEN** in an OpenAPI 3.1 descriptor, schema `Code` is a string, and the properties `billing`, `country`, and `zone` of a schema reference it, `country` with a `description` and `zone` with `enum: [DE, FR]` next to the `$ref`
- **THEN** all three properties have the type `Code`, `Code` is `string`, and no warning is reported

#### Scenario: Properties with additionalProperties
- **WHEN** schema `Inventory` has a required property `updatedAt` of type string, an optional property `warehouse` with schema `Warehouse`, and `additionalProperties` of type integer
- **THEN** `Inventory` has the properties `updatedAt: string` and `warehouse?: Warehouse` and an index signature of type `number | string | Warehouse | undefined`

#### Scenario: Non-identifier property name
- **WHEN** a schema has a property `content-type`
- **THEN** the generated object type contains the property `"content-type"`

#### Scenario: Comment terminator in a description
- **WHEN** an operation's description contains `*/`
- **THEN** the generated client library is still valid TypeScript and the description does not end the doc comment
