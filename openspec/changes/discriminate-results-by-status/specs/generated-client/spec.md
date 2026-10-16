## MODIFIED Requirements

### Requirement: Method result
A method SHALL return a `Promise` that resolves, once the response has been received, to an object with the fields `status`, `data`, and `response`.
If `http.asyncRequest` rejects, for example because the k6 `throw` option is set and the request fails, the returned promise SHALL reject with the same error.
`response` SHALL be the k6 `Response`.
`status` SHALL be `response.status`, which is 0 when k6 received no response.
`data` SHALL be `undefined` if the response body is empty or absent.
Otherwise `data` SHALL be decoded according to the media type in the response's `Content-Type` header, compared case-insensitively and without parameters: parsed as JSON for `application/json` and media types ending in `+json`, and the unparsed response body for any other media type or a missing `Content-Type` header.
Whether the body is parsed as JSON SHALL depend only on the `Content-Type` header, not on whether parsing succeeds.
If a body declared as JSON cannot be parsed, the returned promise SHALL reject with the parse error.

A client library with at least one operation SHALL export the status code types `Status1XX`, `Status2XX`, `Status3XX`, `Status4XX`, and `Status5XX`, each the union of the number literal types of the 100 status codes of its range, such as `400 | 401 | ... | 499`, and `HttpStatus`, the union of these five types.
If a status code type would get the same name as another generated type, the status code type SHALL be the later one and get the numeric suffix.
A client library without operations SHALL NOT export the status code types.

The method's return type SHALL be `Promise<R>`, where `R` is an exported type named after the method in PascalCase followed by `Result`, defined as a union of variants of the form `{ data: D; response: Response; status: S }`.
A documented response is a key of the operation's `responses` that is a status code of three digits from 100 to 599, a range from `1XX` to `5XX` compared case-insensitively, or `default`.
`R` SHALL have one variant for each documented response:
- An explicit status code SHALL give a variant whose `S` is the number literal type of the code.
- A range SHALL give a variant whose `S` is the range's status code type if the operation documents no explicit status code in that range, and otherwise an exported type named after the method in PascalCase followed by `Status` and the range in uppercase, such as `GetPetStatus4XX`, defined as the range's status code type without the explicitly documented codes.
- `default` SHALL give a variant whose `S` is `HttpStatus` if the operation documents no other response, and otherwise an exported type named after the method in PascalCase followed by `StatusDefault`, defined as `HttpStatus` without the explicitly documented codes and without the documented ranges.

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
- **THEN** the client library exports `Status1XX` to `Status5XX`, each with the 100 codes of its range, and `HttpStatus` as the union of the five

#### Scenario: Status code type name in use
- **WHEN** the descriptor has a component schema named `HttpStatus` and at least one operation
- **THEN** the type of the schema is named `HttpStatus`, the union of all status code types is named `HttpStatus2`, and a warning is reported

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
- **THEN** the variant of the `5XX` response has `status: Status5XX`

#### Scenario: Response without content
- **WHEN** an operation documents a `200` response with schema `Pet` and a `204` response without content
- **THEN** its result type has a variant with `status: 204` and `data: undefined`
- **AND** when the server responds with status 204 and an empty body, the method's promise resolves with `data` set to `undefined`

#### Scenario: Several media types
- **WHEN** an operation documents a `200` response with the media type `application/json` with schema `Pet` and the media type `text/plain`
- **THEN** the variant with `status: 200` has the `data` type `Pet | Response["body"]`

#### Scenario: Only a default response
- **WHEN** `listPets` documents only a `default` response with schema `Pet`
- **THEN** its result type consists of a variant with `status: HttpStatus` and `data: Pet` and the status 0 variant

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
