## MODIFIED Requirements

### Requirement: Request building
A method SHALL send exactly one request with k6's `http.asyncRequest`, using the operation's HTTP method, and SHALL NOT use the blocking `http.request`.
The request URL SHALL be the base URL followed by the path, with each path placeholder replaced by the path argument value encoded with `encodeURIComponent`.
Query parameters SHALL be appended as a query string, which SHALL be left out if no query parameter has a value.
The query string SHALL contain, for each field of `query` in property order, the `name=value` pairs of its value, where `undefined` and `null` SHALL give no pair wherever they occur:
- An array SHALL give one pair per element, named after the parameter.
- An object that is not an array SHALL give, for each of its properties in property order, one pair per element if the property value is an array and one pair otherwise, named after the property and not after the parameter.
  This is OpenAPI's `style: form` with `explode: true` for objects, so `{ filter: { color: "red", tags: ["a", "b"] } }` gives `color=red&tags=a&tags=b`.
- Any other value SHALL give one pair, named after the parameter.

An element or property value that is itself an object or an array is a nested value, which is not supported and SHALL give no pair.
The value of any other pair SHALL be the string conversion of the element, property value, or value.
The generator SHALL report a warning that nested values are not supported and are omitted for each query parameter whose schema allows one, which is the case if the schema is:
- an array whose items allow an object or an array, or
- an object with a property, or `additionalProperties`, that allows an object, or that is an array whose items allow an object or an array.

A schema allows an object or an array if it maps to an object type, a `Record`, or an array type, or if one of its `allOf`, `oneOf`, or `anyOf` members does; a schema that maps to `unknown` SHALL NOT give this warning.
A query parameter that declares a `style` other than `form` or `explode: false` SHALL be serialized the same way, and a warning SHALL be reported.
Header parameters whose value is not `undefined` or `null` SHALL be sent as request headers, converted to strings.
If the request body offers an `application/json` media type, the generator SHALL use it; otherwise it SHALL use the request body's first media type in lexicographic order.
The method SHALL encode `body` according to the chosen media type:
- `application/json`: `JSON.stringify` of `body`, with `Content-Type: application/json`.
- `application/x-www-form-urlencoded`: a URL-encoded string that contains the pairs of the fields of `body` by the same rules as the query string, with `Content-Type: application/x-www-form-urlencoded`.
  The generator SHALL report a warning for each field of the body's schema that allows a nested value, by the same rule as for query parameters.
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

#### Scenario: Object query parameter
- **WHEN** the operation `GET /pets` has a query parameter `filter` with an object schema whose properties `color` and `size` are strings and whose property `tags` is an array of strings
- **AND** the method is called with `query` set to `{ filter: { color: "red", tags: ["a", "b"], size: undefined } }`
- **THEN** the request URL ends with `/pets?color=red&tags=a&tags=b`
- **AND** no warning is reported for `filter`

#### Scenario: Nested query parameter value
- **WHEN** the operation `GET /pets` has a query parameter `filter` with an object schema whose property `color` is a string and whose property `owner` is an object with a string property `name`
- **AND** the method is called with `query` set to `{ filter: { color: "red", owner: { name: "Ann" } } }`
- **THEN** the request URL ends with `/pets?color=red`
- **AND** a warning is reported that names the query parameter `filter` and says that nested values are not supported and are omitted

#### Scenario: Array of objects in the query
- **WHEN** the operation `GET /pets` has a query parameter `owners` with an array schema whose items are objects
- **THEN** a warning naming the query parameter `owners` is reported

#### Scenario: JSON request body
- **WHEN** an operation's request body has the media type `application/json`
- **AND** the method is called with the body `{ name: "Rex" }`
- **THEN** the request body is `{"name":"Rex"}` and the `Content-Type` header is `application/json`

#### Scenario: URL-encoded request body
- **WHEN** an operation's only request body media type is `application/x-www-form-urlencoded`
- **AND** the method is called with the body `{ name: "Rex", tags: ["a", "b"], age: 3, note: undefined }`
- **THEN** the request body is `name=Rex&tags=a&tags=b&age=3` and the `Content-Type` header is `application/x-www-form-urlencoded`

#### Scenario: Object field of a URL-encoded request body
- **WHEN** an operation's only request body media type is `application/x-www-form-urlencoded`, and its schema has a string property `name` and an object property `address` with a string property `city`
- **AND** the method is called with the body `{ name: "Rex", address: { city: "Berlin" } }`
- **THEN** the request body is `name=Rex&city=Berlin`
- **AND** no warning is reported for `address`

#### Scenario: Nested field of a URL-encoded request body
- **WHEN** an operation's only request body media type is `application/x-www-form-urlencoded`, and its schema has an object property `address` with a string property `city` and an object property `geo`
- **AND** the method is called with the body `{ address: { city: "Berlin", geo: { lat: 52.5 } } }`
- **THEN** the request body is `city=Berlin`
- **AND** a warning is reported that names the field `address` and says that nested values are not supported and are omitted

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
