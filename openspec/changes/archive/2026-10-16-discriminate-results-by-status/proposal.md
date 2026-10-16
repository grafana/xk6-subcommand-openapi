# Proposal

## Why

A method's result is a union of two variants told apart only by `ok`, so the type of `data` can only be narrowed by success or failure, not by status code.
Test scripts usually assert an exact status, such as `201` after a create or `404` after a delete, so today they check the status and then check `ok` a second time just to narrow `data`.
A user who moved an e2e suite to the new generator reported exactly this: every call needed an extra type guard on `ok` next to the status assertion.
Within one side, such as `Pet | Job` for `200` and `202`, there is no typed way to tell the documented responses apart at all.
Once `status` narrows `data`, `ok` is redundant: it can be derived from `status`, and keeping it as a second discriminant would force `default` to be split into a success and a failure part.

## What Changes

- A result gains a top-level `status` field, the HTTP status code of the response, and becomes a union of one variant per documented response instead of one success and one failure variant.
  Each variant has a `status`, its own `data` type, and `response`.
  `if (res.status === 404)` and `switch (res.status)` narrow `data` to the type documented for that status, without any helper function.
- **BREAKING**: The `ok` field is removed; a success check is written as a check for the expected status, such as `res.status === 201`.
- A variant for an explicit status code has that code as a number literal type.
  A variant for a range or for `default` has a status type that lists the codes it covers, named per method if it excludes codes, such as `GetPetStatus4XX` for `4XX` without the explicitly documented 4xx codes.
  This keeps narrowing exact when `default` or ranges are documented, which a `number` catch-all would break.
- `default` covers every status code from 100 to 599 that no other documented response covers, including 2xx codes, as OpenAPI defines it.
- Status codes that the descriptor does not document are not part of the result type, as today; comparing `status` with such a code is reported by editors as a comparison without overlap, and `res.response.status` can be compared instead.
- Every client library with operations exports the status code types `HttpStatus1XX` to `HttpStatus5XX`, each the union of the 100 codes of its range, and `HttpStatusAll`, the union of all of them.
- Every result has a variant for status `0`, with `data: undefined`, for a request that got no response.
- A type name that the client library imports or uses, such as `Response`, `Params`, `Record`, or `Exclude`, the client class name, and the names of the status code types are reserved: a schema with such a name gets a numeric suffix and a warning, instead of producing an invalid client library.
- Every generated client library must type-check in strict mode; `make typecheck` checks this for the golden files with the TypeScript compiler and asserts the narrowing of the result types, and a CI workflow runs it.
  Node.js becomes a development and CI dependency for this check; users still need only the k6 binary.
- A `responses` key that is neither a status code from 100 to 599, a range from `1XX` to `5XX`, nor `default` is skipped with a warning.

## Capabilities

### New Capabilities

### Modified Capabilities
- `generated-client`: The `Method result` requirement replaces `ok` with `status`, defines one variant per documented response, the treatment of `default`, the exported status code types, the status `0` variant, and the warning for invalid `responses` keys.
  The `Client library file` requirement adds that the client library type-checks in strict mode.
  The `Schema types` requirement reserves the type names that the client library imports or uses, the client class name, and the status code type names.

## Impact

- Code: `internal/tsgen/results.go` builds the per-response variants and the per-method status types; `internal/tsgen/status.go` declares the shared status code types; `internal/tsgen/tsgen.go` starts the type name registry with the reserved names; `internal/tsgen/model` gains a type node for `Exclude<T, U>`, which `internal/tsgen/render` prints; the method template in `internal/tsgen/render/templates/client.ts.tmpl` returns `status` instead of `ok`.
- Generated output: every client library with operations changes its result types and gains about 3 KB of status code types, so most golden files in `test/testdata/` change; a schema with a reserved name is renamed with a numeric suffix.
- Verification: `test/typecheck/` holds the TypeScript toolchain and the narrowing assertions, the `Makefile` gains `make typecheck`, and `.github/workflows/typecheck.yml` runs it; `openspec/config.yaml` allows Node.js in development and CI, and `AGENTS.md`, `CONTRIBUTING.md`, and the README's development section list the new check.
- `README.md` describes `status`, the per-status narrowing, and the status code types, and its example checks exact status codes instead of `ok`.
