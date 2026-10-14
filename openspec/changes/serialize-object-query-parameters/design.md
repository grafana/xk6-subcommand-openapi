# Design

## Context

The generated client builds the query string and URL-encoded bodies with one private helper, `_encodeQuery` in `internal/tsgen/render/templates/client.ts.tmpl`.
It iterates the fields of the passed object, expands arrays into one pair per element, skips `undefined` and `null`, and converts every remaining value with `String(item)`.
The generator already warns about query serializations other than `style: form` with `explode: true` in `checkQueryStyle` in `internal/tsgen/operations.go`, and keeps serializing them as that style.
No test runs a generated client in k6 against a server: `make it` only checks that each fixture's client library loads, and the golden tests compare generated text.
This change keeps that test setup.
See proposal.md for why object values need a defined serialization.

## Goals / Non-Goals

**Goals:**
- Object values are sent as OpenAPI's default `style: form` with `explode: true`, in both the query string and URL-encoded bodies, with one shared helper.
- No value is ever sent as `[object Object]`.
- Nested values, which OpenAPI does not define, are never sent in a made-up format, and the generator tells the user when a schema allows them.

**Non-Goals:**
- Implementing `deepObject`, `spaceDelimited`, `pipeDelimited`, or `explode: false`; they keep the existing warning and fallback.
- Query parameters described with `content` instead of `schema`.
- Honoring the `encoding` object of URL-encoded bodies.
- Detecting name clashes between a flattened property and another query parameter or body field.
- Removing the jslib.k6.io network requirement.
- Verifying the runtime serialization by running a generated client against a server; how to test the generated client end to end is established in a separate change.

## Decisions

### 1. Flatten one level, omit deeper values
The helper flattens a top-level object into one pair per property, expands a top-level array or an array property value into one pair per element, and omits any object or array found below that.
Sketch:

```ts
private _encodeQuery(values: unknown): string {
  const search = new URLSearchParams();
  const append = (name: string, value: unknown) => {
    for (const item of Array.isArray(value) ? value : [value]) {
      // Nested objects and arrays are not supported; the generator warns about them.
      if (item !== undefined && item !== null && typeof item !== "object") {
        search.append(name, String(item));
      }
    }
  };
  for (const [name, value] of Object.entries((values ?? {}) as Record<string, unknown>)) {
    if (value !== null && typeof value === "object" && !Array.isArray(value)) {
      for (const [key, item] of Object.entries(value)) append(key, item);
    } else {
      append(name, value);
    }
  }
  return search.toString();
}
```

Alternatives:
- Recursive flattening, such as `owner.name=Ann` or `owner[name]=Ann`: these are conventions of particular server frameworks, not OpenAPI, so picking one would be a guess.
- `JSON.stringify` text for nested values, as multipart parts use for objects: also a guess, because a server receives it as a plain string and would only parse it if it was written to.
- `String(item)` for nested values, as today: keeps `[object Object]`, the bug this change fixes.

Omitting sends no value in a made-up format, and the generation-time warning tells the user that nested values are not supported and are omitted.
The generated types still accept nested values, because they describe the schema; narrowing them would make the types disagree with the `Schema types` requirement.

### 2. URL-encoded bodies follow the same rules
OpenAPI defines the default serialization of a URL-encoded body field without an `encoding` entry as `style: form` with `explode: true`, the same as for query parameters, so the existing shared helper stays shared and the `Request building` requirement keeps saying "by the same rules as the query string".
The alternative, `JSON.stringify` for object fields as in multipart bodies, contradicts that default.

### 3. The nested-value check works on the mapped model types, where they are mapped
The check runs right after a query parameter's or a URL-encoded body's schema is mapped: in `paramsMember` for each query parameter, and in `requestBody` for a URL-encoded body, where the operation for the message is at hand.
For a query parameter it checks the parameter's type, and for a body it checks each property of the body's type, which is one body field.
For a body whose type is an intersection or union, the properties of all members are checked, and a field that several members declare is reported once.
The additional fields of a body, which its `additionalProperties` schema describes, are checked by the same rule as a field and reported once per body.
A property type allows a nested value if it is an array whose element allows an object or an array, or an object whose property or additional property types allow an object or are arrays whose element allows an object or an array; unions and intersections are checked member by member.
"Allows an object or an array" means `Object`, `Record`, or `Array`, directly or through `Named`, `Union`, or `Intersection`; `Unknown` does not count.
A set of visited `Named` references bounds the walk on recursive types.
`model.Object` keeps the mapped `additionalProperties` type in `Additional`, next to `Index`, which also includes the types of the named properties because a TypeScript index signature covers them; checking `Index` for the additional fields of a body would report a nested named property as nested additional fields.

A `Named` reference carries only the type name, so the check resolves it through the declarations in `typeDeclarations`, which are kept in a map keyed by the claimed name instead of a slice; `buildFile` already sorts them by name, so the slice's order was never used.
Every declaration that the check reaches is complete: `declare` records a declaration once its type is built, and `typeOf` has built every schema it reaches, including recursive ones, by the time it returns to `paramsMember` or `requestBody`.
The only declarations under construction at that point are the method's Options and Query types, which no schema can reference.

Working on model types reuses every mapping rule the `Schema types` requirement already defines, such as type inference, `allOf`, nullable types, and `$ref` resolution, instead of reimplementing them on `openapi3.Schema`.
The alternatives:
- Checking the `openapi3.Schema` in `checkQueryStyle` would duplicate the schema mapping and could disagree with the generated types.
- A pass over the finished `model.File` would need the operation of each method for the message, which `model.Method` does not keep, because its path segments hold argument names instead of placeholders.
- Checking only types that `typeOf` returns as `model.Object` would miss every `$ref`, and every URL-encoded body, which is always declared as a named type.

Warnings are reported in method order and, within a method, for query parameters before body fields, each in property order, and the warning about a body's additional fields after the fields of the member that declares them, so the output stays deterministic.
Each warning follows the warnings of mapping the schema it is about; the golden warning files record the order.
Messages:
- `the query parameter "filter" of GET /pets allows nested objects or arrays, which are not supported and are omitted`
- `the field "address" of the request body of POST /pets allows nested objects or arrays, which are not supported and are omitted`
- `the additional fields of the request body of POST /pets allow nested objects or arrays, which are not supported and are omitted`

## Risks / Trade-offs

- [Flattening drops the parameter name, so a property can clash with another parameter or field, for example `filter.limit` and `limit`] → Both pairs are sent, as `form` with `explode: true` defines; detecting clashes is a non-goal and can be added as a warning later.
- [Servers built with frameworks that expect `filter[color]=red` or `filter.color=red` reject the flattened form] → That is a `deepObject` or framework convention the descriptor should declare; declared `deepObject` keeps the existing warning, so supporting it is a natural next change.
- [A top-level value that is an object but not a plain object, such as an `ArrayBuffer`, is flattened into no pairs] → The generated types never allow such values in the query or a URL-encoded body.
- [A nested value that the user passes is silently missing from the request at runtime] → The generator warns for every query parameter and form body field whose schema allows one, so the omission is visible when the client is generated.
- [Nested arrays at the top level, such as `[[1, 2]]`, were sent as `1,2` and are now omitted] → This is the documented **BREAKING** change; there is no release yet, and the generator warns about it.
- [The runtime serialization is checked only by reviewing the generated helper in the golden files, not by running it] → The same gap as for the rest of the `Request building` requirement; the separate end-to-end testing change closes it.

## Migration Plan

There is no release yet, so the changed serialization needs no migration.
