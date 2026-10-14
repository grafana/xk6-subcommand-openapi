# Proposal

## Why

A query parameter with an object schema is typed as an object, but the generated client converts its value with `String(item)`, so `{ filter: { color: "red" } }` is sent as `?filter=%5Bobject+Object%5D`.
The call type-checks and no warning is reported, so a load test silently sends wrong requests; object fields of `application/x-www-form-urlencoded` bodies have the same problem, because they use the same encoding.
This is the first item under `Open problems` in the `generated-client` spec.

## What Changes

- An object value of a query parameter is serialized as OpenAPI's default for query parameters, `style: form` with `explode: true`: one `name=value` pair per property, named after the property, so `{ filter: { color: "red", size: "L" } }` gives `?color=red&size=L`.
  Property values follow the existing rules: an array gives one pair per element, and `undefined` and `null` are omitted.
- An object field of an `application/x-www-form-urlencoded` body is serialized the same way, which is also OpenAPI's default for form bodies without an `encoding` entry.
- **BREAKING**: Nested values are not supported: an object or array below that level, such as an object property value or an object array element, gives no pair, instead of being sent as `[object Object]` or as the comma-joined `String` of an array.
  OpenAPI does not define a serialization for nested values, and no choice would be understood by servers in general, so the generator reports a warning for each query parameter and form body field whose schema allows a nested value, saying that nested values are not supported and are omitted.
- Query parameters that declare `style: deepObject`, another style, or `explode: false` keep being serialized as `style: form` with `explode: true`, with the existing warning; supporting other styles is not part of this change.
- The object query parameter item is removed from the `Open problems` in the `generated-client` spec.

## Capabilities

### New Capabilities

### Modified Capabilities
- `generated-client`: The `Request building` requirement defines how object values and nested values of query parameters and URL-encoded body fields are serialized, and when a warning is reported.

## Impact

- Code: the `_encodeQuery` helper in `internal/tsgen/render/templates/client.ts.tmpl`, and a new generation-time check for nested values next to `checkQueryStyle` in `internal/tsgen/operations.go`, which also covers the fields of URL-encoded request bodies.
- Generated output: every client library that has query parameters or a URL-encoded body gets the new helper, so the golden files in `test/testdata/` change; fixtures gain object and nested query parameters and form body fields.
- Tests: the golden tests and `make it` cover the new helper and warnings; verifying the runtime serialization by running a generated client against a server is left to a separate change that establishes how to test the generated client end to end.
- `README.md` describes the serialization of object values and the warning for nested values.
