# Design

## Context

`internal/tsgen/results.go` builds each method's Result type as a union of exactly two variants, `{ data; ok: true; response }` and `{ data; ok: false; response }`, whose `data` types are the unions of the data types of the documented success and failure responses.
The method template in `internal/tsgen/render/templates/client.ts.tmpl` returns `{ ok, data, response } as <Result>`.
The model in `internal/tsgen/model` has number literal types and unions, but no way to express `Exclude<T, U>`.
Type names are made unique by one `nameRegistry`, where the first claimant keeps a name and later ones get a numeric suffix and a warning.
kin-openapi does not validate the keys of a `responses` object, so a key like `20X` reaches the generator unchanged.
No test type-checks the generated TypeScript: k6 strips types without checking them, and the project has no Node.js dependency, so the golden tests compare generated text and `make it` checks that each client loads in k6.
The original decision for a single `ok` discriminant is recorded in `openspec/changes/archive/2026-10-08-generate-client-from-descriptor/design.md`, section "Result: success flag instead of per-status variants"; see proposal.md for why it is revisited.

## Goals / Non-Goals

**Goals:**
- Plain `if` and `switch` on `res.status` narrow `data` exactly, for every descriptor, including ones that document `default` and ranges, without helper functions.
- The result type describes exactly the documented responses and the no-response case, nothing the descriptor does not say.
- Error messages and editor hovers stay readable.

**Non-Goals:**
- Typing status codes that the descriptor does not document.
- Making assertion libraries narrow types: `expect(res.status).toBe(200)` does not narrow, because `expect` is not a TypeScript assertion function.
- Type-checking the generated client in the project's tests.
- Verifying the runtime behavior by running a generated client against a server, which a separate change establishes.

## Decisions

### 1. `status` is the only discriminant
Each documented response gets its own variant with a literal `status` type, so TypeScript narrows `data` on `status`.
`status` must be a top-level property, because TypeScript does not narrow a union on a nested property like `res.response.status`.
Alternatives:
- Keeping `ok` as the only discriminant: forces a second check after every status assertion, the problem in proposal.md.
- Keeping `ok` next to `status` as a second literal discriminant: it is derivable from `status`, so it is a second way to write the same check, and a literal `ok` on every variant forbids a variant that mixes 2xx and other codes, which forces `default` to be split into a success and a failure part with its own rules.
  What it adds is narrowing to all success types at once, which matters only for operations with several success codes; in the descriptor of the user who asked for this change, all 51 operations document exactly one 2xx code.
- Deriving `ok` from the documented 2xx codes instead of the 200 to 299 range: an undocumented `202` would turn into a failure in load test checks, and `ok` would no longer mean what `fetch`'s `Response.ok` means.
- A discriminant keyed by the matched `responses` key, such as `matched: "4XX"`: needs runtime mapping and a second, less obvious field next to `status`.

### 2. Catch-alls as finite literal unions
`Status1XX` to `Status5XX` each list their 100 codes, and `HttpStatus` is their union, so `default` and ranges can be typed with `Exclude` as finite unions that do not contain the explicitly documented codes.
OpenAPI allows only the ranges `1XX` to `5XX`, so these five types cover every range key.
The five lists add about 3 KB per client library, and are declared once per file because the client library is a single module.
Status 0 is not part of `HttpStatus`, because it has its own variant (decision 5).

Alternatives, each checked with `tsc` 5.9:
- `status: number` as the catch-all: `200` is assignable to `number`, so `status === 200` keeps the catch-all variant and narrows `data` only to `Pet | Error`.
- A branded number, `number & { [brand]: true }`: a brand is not a literal type, so TypeScript stops treating `status` as a discriminant and no comparison narrows at all, not even `status === 200`.
- Generated `isStatus`/`assertStatus` guard functions over a `number` catch-all: a type predicate can pick the variant with the literal code and fall back to the catch-all, so they narrow exactly, but they duplicate what `if` does and have to be discovered.
- No catch-all at all, only explicit codes: narrows exactly, but `default` and ranges are documented responses whose data types must appear in the result type.
- `HttpStatus` with only the registered status codes: fewer literals, but an unregistered code such as `299` or `599` would fall outside every variant, even when `default` documents it.
- One list of all codes per file instead of five lists per range: ranges would then need their own lists anyway, so splitting by range costs nothing extra.

### 3. One named status type per catch-all that excludes codes
A range with explicitly documented codes and a `default` next to other responses get an exported type, such as `GetPetStatus4XX = Exclude<Status4XX, 404>` and `GetPetStatusDefault = Exclude<HttpStatus, 200 | 404 | Status4XX>`.
TypeScript keeps such a name in error messages and hovers, so `r.status` shows as `200 | 404 | GetPetStatusDefault` instead of a list of about 500 codes, which is what an inline `Exclude` prints, truncated after about 30 codes.
The name is lost only when narrowing removes a code from inside a catch-all, such as after `r.status !== 503`; TypeScript then builds a new anonymous union and prints the codes.
A range without explicitly documented codes uses the shared `Status<N>XX` directly, and a `default` without other responses uses `HttpStatus`, because nothing is excluded.
The names end in `Status4XX` and `StatusDefault`, which cannot clash with inline response types such as `GetPet4XX` and `GetPetDefault`, and are claimed right after the Result name, so a clash with a schema name follows the existing suffix rule.

### 4. `default` covers every code not documented otherwise; undocumented codes stay untyped
`default` covers every code from 100 to 599 that no explicit code or range of the operation covers, including 2xx codes, which is how OpenAPI defines it.
An operation without `default` has variants only for its explicit codes and ranges, and the status 0 variant; an operation with an empty `responses` object, which already gets a validation warning, has only the status 0 variant.
The result type then says what the descriptor says and nothing more, and editors flag a comparison with a code the descriptor does not document, such as a mistyped `if (res.status === 240)`.
Alternative: treating an operation without `default` as if it documented one with `data: unknown`, so every code from 100 to 599 is in a variant.
It makes a comparison with an undocumented code type-check, but adds a variant that the descriptor does not contain.

### 5. A variant for status 0
Every result has `{ data: undefined; response: Response; status: 0 }`.
k6 resolves `http.asyncRequest` with status 0 when it gets no response, such as on a timeout, a refused connection, or a DNS or TLS failure, unless the `throw` option is set, and `_decodeBody` returns `undefined` for its empty body.
These failures are common under load, and `res.status === 0` is how a script tells them apart from a server error, so the check must type-check and narrow `data` exactly.
Alternatives:
- No variant for 0: `res.status === 0` would be a TS2367 error, because `HttpStatus` has only HTTP codes.
- 0 as a member of the `default` catch-all: `res.status === 0` would narrow `data` to the `default` schema or to `unknown`, although it is always `undefined`.

### 6. Invalid `responses` keys are skipped with a warning
A key must be three digits from 100 to 599, a range from `1XX` to `5XX` compared case-insensitively as the generator does today, or `default`.
Any other key, such as `20X`, `600`, or `2xx0`, cannot be typed as a status and is skipped, with the warning `the response "20X" of GET /pets has a key that is not a status code, a range, or default, and is skipped`.
Its codes are then covered by `default` if the operation documents one, and by no variant otherwise.
Alternative: failing the generation; the generator reports other invalid constructs as warnings and keeps going, so this follows that.

### 7. The shared status code types are claimed last
The shared types are declared after all operations are mapped, so a schema named `HttpStatus`, which some APIs have, keeps its name and the status code type gets the suffix.
Result and per-method status types are built while operations are mapped and refer to the shared types before their names are known.
`buildFile` passes the result mapper a holder for the shared types; after mapping, it claims the shared names and sets them in the declarations that refer to them, which are only the per-method status types and the Result variants with a shared status type.
Alternative: claiming the shared names first, which renames a user's schema instead, a visible change to a type that the user wrote.

### 8. Model, rendering, and runtime
`model` gains `Exclude{From, Excluded Type}`, printed as `Exclude<From, Excluded>`; number literals and unions already exist.
The variant's properties are `data`, `response`, and `status`, in lexicographic order as the `Deterministic output` requirement says, and the variants are ordered by `responses` key, which `sortedKeys` already gives, then the status 0 variant.
The template returns `{ status: response.status, data, response }` and keeps the cast to the Result type, which is needed because `response.status` is a `number`.

### 9. Verification
The golden tests check the generated type text against the spec scenarios, and `make it` checks that every client still loads in k6.
That the text narrows as specified was verified with `tsc` 5.9 outside the repository during this design; the implementation repeats this check by hand on the `results` golden file, without adding Node.js to the project.

## Risks / Trade-offs

- [Narrowing away a code inside a catch-all prints about 500 codes in hovers and errors] → Only after comparing with a code that is not documented explicitly, such as `r.status !== 503`, which is rare; the named types keep every other message short.
- [A response with an undocumented status resolves with a `status` outside the result type, so TypeScript treats it as impossible, for example in the `default` branch of a `switch` over the documented codes, which is typed `never`] → The same gap as for undocumented responses before this change; README documents it, and documenting the response or a `default` in the descriptor closes it.
- [Editors report a comparison with an undocumented status, such as `res.status === 503`, as TS2367, a comparison without overlap] → `res.response.status` is a plain `number` and can be compared freely, and assertion libraries such as `expect(res.status).toBe(503)` take untyped arguments and are not affected.
- [Removing `ok` breaks scripts that check `res.ok`, and an operation with several success codes needs them listed, such as `res.status === 200 || res.status === 202`] → This is the **BREAKING** part; there is no release yet, and README shows the status checks.
- [The cast in the generated method hides a mismatch between the runtime value and the type] → The same cast exists today; the runtime value is built from `response.status` and the decoded body, as the requirement specifies.
- [`expect(response.status).toBe(200)` still does not narrow `data`] → A non-goal; `if (res.status !== 200) fail(...)` or `switch` narrows, and README shows it.

## Migration Plan

There is no release yet, so the changed result types need no migration.
