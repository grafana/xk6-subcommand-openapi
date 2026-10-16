## MODIFIED Requirements

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
It SHALL type-check without errors with the TypeScript compiler in strict mode, using the k6 type definitions of `@types/k6` and declarations of the jslib modules that it imports.

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

#### Scenario: Client library type-checks
- **WHEN** a client library is generated from any descriptor
- **THEN** the TypeScript compiler in strict mode reports no errors for it

#### Scenario: Client library loads in k6
- **WHEN** a k6 script consists of a generated client library followed by a default function that constructs its client
- **THEN** `k6 run` on that script exits with exit code 0

### Requirement: Method result
A method SHALL return a `Promise` that resolves, once the response has been received, to an object with the fields `status`, `data`, and `response`.
If `http.asyncRequest` rejects, for example because the k6 `throw` option is set and the request fails, the returned promise SHALL reject with the same error.
`response` SHALL be the k6 `Response`.
`status` SHALL be `response.status`, which is 0 when k6 received no response.
`data` SHALL be `undefined` if the response body is empty or absent.
Otherwise `data` SHALL be decoded according to the media type in the response's `Content-Type` header, compared case-insensitively and without parameters: parsed as JSON for `application/json` and media types ending in `+json`, and the unparsed response body for any other media type or a missing `Content-Type` header.
Whether the body is parsed as JSON SHALL depend only on the `Content-Type` header, not on whether parsing succeeds.
If a body declared as JSON cannot be parsed, the returned promise SHALL reject with the parse error.

A client library with at least one operation SHALL export the status code types `HttpStatus1XX`, `HttpStatus2XX`, `HttpStatus3XX`, `HttpStatus4XX`, and `HttpStatus5XX`, each the union of the number literal types of the 100 status codes of its range, such as `400 | 401 | ... | 499`, and `HttpStatusAll`, the union of these five types.
The names of the status code types SHALL be reserved, as the `Schema types` requirement specifies.
A client library without operations SHALL NOT export the status code types.

The method's return type SHALL be `Promise<R>`, where `R` is an exported type named after the method in PascalCase followed by `Result`, defined as a union of variants of the form `{ data: D; response: Response; status: S }`.
A documented response is a key of the operation's `responses` that is a status code of three digits from 100 to 599, a range from `1XX` to `5XX` compared case-insensitively, or `default`.
`R` SHALL have one variant for each documented response:
- An explicit status code SHALL give a variant whose `S` is the number literal type of the code.
- A range SHALL give a variant whose `S` is the range's status code type if the operation documents no explicit status code in that range, and otherwise an exported type named after the method in PascalCase followed by `Status` and the range in uppercase, such as `GetPetStatus4XX`, defined as the range's status code type without the explicitly documented codes.
- `default` SHALL give a variant whose `S` is `HttpStatusAll` if the operation documents no other response, and otherwise an exported type named after the method in PascalCase followed by `StatusDefault`, defined as `HttpStatusAll` without the explicitly documented codes and without the documented ranges.

`D` SHALL be the data type of the documented response.
The data type of a single response SHALL be `undefined` if it has no content, and otherwise the union of the data types of its media types, each type included once: the type of the schema for a JSON media type (`application/json` or a media type ending in `+json`) with a schema, `unknown` for a JSON media type without a schema, and `Response["body"]` for any other media type.
`R` SHALL also have the variant `{ data: undefined; response: Response; status: 0 }` for a request that got no response.
The variants SHALL be ordered by their `responses` key in lexicographic order, followed by the status 0 variant.
A `responses` key that is not a documented response by the definition above SHALL be skipped, and a warning that names the key and the operation SHALL be reported.

These types describe documented responses only, and SHALL NOT contain variants for status codes that the descriptor does not document.
A response whose status code no variant covers, such as `503` for an operation that documents only `200` and `404`, SHALL still resolve the method's promise as specified above, with `status` and `data` outside the result type; the same holds for `data` of a response whose `Content-Type` is not documented for its status.

#### Scenario: Narrowing by status
- **WHEN** `GET /pets/{petId}` with `operationId: getPet` documents a `200` response with schema `Pet`, a `404` response with schema `NotFound`, and a `default` response with schema `Error`
- **THEN** the client library exports a type `GetPetResult` that is the union of the variants `{ data: Pet; response: Response; status: 200 }`, `{ data: NotFound; response: Response; status: 404 }`, `{ data: Error; response: Response; status: GetPetStatusDefault }`, and `{ data: undefined; response: Response; status: 0 }`, regardless of how the type is formatted
- **AND** it exports a type `GetPetStatusDefault` whose members are the codes from 100 to 599 except 200 and 404
- **AND** `getPet` returns `Promise<GetPetResult>`
- **AND** after `const res = await client.getPet(1)`, checking `res.status === 200` narrows `res.data` to `Pet`, `res.status === 404` to `NotFound`, `res.status === 201` and `res.status === 503` to `Error`, and `res.status === 0` to `undefined`

#### Scenario: Narrowing by success
- **WHEN** an operation documents a `200` response with schema `Pet`, a `202` response with schema `Job`, and a `404` response with schema `NotFound`
- **THEN** after `const res = await client.updatePet(1)`, checking `res.status === 200 || res.status === 202` narrows `res.data` to `Pet | Job`

#### Scenario: Status code types
- **WHEN** the descriptor has at least one operation
- **THEN** the client library exports `HttpStatus1XX` to `HttpStatus5XX`, each with the 100 codes of its range, and `HttpStatusAll` as the union of the five

#### Scenario: Status code type name in use
- **WHEN** the descriptor has a component schema named `HttpStatusAll` and at least one operation
- **THEN** the union of all status code types is named `HttpStatusAll`, the type of the schema is named `HttpStatusAll2`, and a warning is reported

#### Scenario: Several success responses
- **WHEN** an operation documents a `200` response with schema `Pet` and a `202` response with schema `Job`
- **THEN** its result type has a variant with `status: 200` and `data: Pet` and a variant with `status: 202` and `data: Job`
- **AND** checking `status === 202` narrows `data` to `Job`

#### Scenario: Range with explicit codes
- **WHEN** `updatePet` documents a `404` response with schema `NotFound` and a `4XX` response with schema `Problem`
- **THEN** its result type has a variant with `status: UpdatePetStatus4XX` and `data: Problem`, and `UpdatePetStatus4XX` has the codes from 400 to 499 except 404
- **AND** checking `status === 418` narrows `data` to `Problem`, and checking `status === 404` narrows it to `NotFound`

#### Scenario: Range without explicit codes
- **WHEN** an operation documents a `200` response and a `5XX` response, and no other code from 500 to 599
- **THEN** the variant of the `5XX` response has `status: HttpStatus5XX`

#### Scenario: Response without content
- **WHEN** an operation documents a `200` response with schema `Pet` and a `204` response without content
- **THEN** its result type has a variant with `status: 204` and `data: undefined`
- **AND** when the server responds with status 204 and an empty body, the method's promise resolves with `data` set to `undefined`

#### Scenario: Several media types
- **WHEN** an operation documents a `200` response with the media type `application/json` with schema `Pet` and the media type `text/plain`
- **THEN** the variant with `status: 200` has the `data` type `Pet | Response["body"]`

#### Scenario: Only a default response
- **WHEN** `listPets` documents only a `default` response with schema `Pet`
- **THEN** its result type consists of a variant with `status: HttpStatusAll` and `data: Pet` and the status 0 variant

#### Scenario: No documented failure
- **WHEN** `createPet` documents only a `201` response with schema `Pet`
- **THEN** its result type consists of a variant with `status: 201` and `data: Pet` and the status 0 variant

#### Scenario: No documented responses
- **WHEN** `ping` has an empty `responses` object
- **THEN** its result type consists of the status 0 variant only

#### Scenario: Invalid response key
- **WHEN** `GET /pets` documents the responses `200` and `20X`
- **THEN** the client library is generated, its result type has no variant for `20X`, and a warning that names `20X` and `GET /pets` is reported

#### Scenario: Runtime failure result
- **WHEN** a method is called and the server responds with status 404, the header `Content-Type: application/json`, and the body `{"message":"not found"}`
- **THEN** the method's promise resolves to an object whose `status` is `404`, whose `data` is the parsed object, and whose `response.status` is `404`

#### Scenario: Undocumented status
- **WHEN** `getPet` documents only `200` and `404`, and the server responds with status 503, the header `Content-Type: text/html`, and the body `<html>`
- **THEN** the method's promise resolves to an object whose `status` is `503` and whose `data` is the string `<html>`

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
- **THEN** the method's promise resolves to an object whose `status` is `0`, whose `data` is `undefined`, and whose `response.status` is `0`

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
The reserved type names SHALL NOT be the name of a type other than the one they are reserved for: the identifiers that the client library imports, `FileData`, `FormData`, `Params`, `Response`, `URL`, and `URLSearchParams`, the global types that it uses, `ArrayBuffer`, `Exclude`, `Promise`, and `Record`, the client class name, and the status code types of the `Method result` requirement.
They SHALL be reserved even if the client library does not use them, and a type that would get a reserved name SHALL get a numeric suffix, and a warning SHALL be reported.
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
- **THEN** the client library exports one type `NotFoundResponse`, the `404` variants of both `GetPetResult` and `DeletePetResult` have the data type `NotFoundResponse`, and no warning is reported

#### Scenario: Reserved type name
- **WHEN** the descriptor has the title `Pet Store` and component schemas named `Response`, `Exclude`, and `PetStoreClient` that operations reference
- **THEN** their types are named `Response2`, `Exclude2`, and `PetStoreClient2`, and a warning is reported for each

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
