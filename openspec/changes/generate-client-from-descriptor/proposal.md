# Proposal

## Why

The `k6 x openapi` subcommand exists, but it ignores its `<openapi-spec>` argument and generates an empty client library.
This change makes the subcommand do its actual job: load an OpenAPI descriptor from a local file or a remote URL and generate a typed k6 client library from it, using the output process the subcommand already has (stdout or `--out`, optionally followed by an example script).

## What Changes

- `<openapi-spec>` is now read: it is either a local file path or an `http`/`https` URL, in JSON or YAML.
- **BREAKING**: A descriptor that does not exist, cannot be fetched, or cannot be parsed now fails the command, where before the argument was ignored.
- Local and remote external `$ref`s in the descriptor are resolved.
- OpenAPI 3.x descriptors are accepted; Swagger 2.0 descriptors are rejected as unsupported.
- A descriptor that loads but fails OpenAPI validation still generates a client; validation problems are reported as warnings through k6's logger.
- **BREAKING**: The default output is now a generated TypeScript client library instead of empty output.
  It contains exported types for the schemas used by the operations and one client class with one method per operation.
- Each client method is `async`: it sends its request with k6's non-blocking `http.asyncRequest` and returns a promise, so scripts `await` calls and can run several requests concurrently with `Promise.all`.
- The promise resolves to a result with an `ok` flag, the response body decoded according to its `Content-Type` header (JSON parsed, anything else unchanged), and the k6 response; checking `ok` narrows the body's type to the documented success types or the documented error types.
- **BREAKING**: `--example` now appends a k6 script whose `async` default function constructs the generated client, instead of a hello-world script that logs `Hello, World!`.
  Calling an operation from the example is left to a later change.
- Constructs the generator cannot represent are skipped or approximated with a warning through k6's logger; warnings never reach the `--out` file, and reach stdout only with `--log-output=stdout`.
- Security schemes and security requirements are not used for generation: the client sends no credentials on its own, and callers supply them through `commonK6Params` or `k6Params`.
  A scheme that cannot be supplied that way, an `apiKey` in the query or an `http` scheme other than `basic`, `bearer`, or `digest`, produces a warning.
  Generating support for security schemes is postponed to a later change, not dropped.

## Capabilities

### New Capabilities
- `generated-client`: The shape and runtime behavior of the generated TypeScript client library: types, client class, methods, request building, and the method result.

### Modified Capabilities
- `openapi-command`: The `<openapi-spec>` argument is now loaded from a file or URL, load failures and validation warnings are reported, the default output is the generated client, and `--example` constructs the client.

## Impact

- Code: the command moves from the root package (`cmd.go`, `generate.go`, `help.md`) into `internal/command`, and only the registration stays in `register.go`; descriptor loading is added as `internal/descriptor` and client generation as `internal/tsgen`, and `templates/example.ts` is replaced by an embedded template under `internal/tsgen/render/templates/`.
- New dependency: `github.com/getkin/kin-openapi`.
- Tests: the in-process tests move from `cmd_test.go` into package `test` under `test/` and gain descriptor fixtures with golden client and warning files in `test/testdata/`; `make it` generates a client with an example from each fixture and runs it with `k6 run`.
  Verifying the runtime behavior of the generated client against a test server is left to a later change.
- The generated client imports `URL`, and when needed `URLSearchParams` and `FormData`, from `https://jslib.k6.io`, so running a script that contains it needs network access to jslib.k6.io, including in `make it` and CI.
  The hardcoded jslib imports are an acknowledged limitation accepted for this change; removing the network requirement is postponed to a later change, not dropped.
- `internal/command/help.md` and `README.md` are updated to describe the new behavior.
