# Spec Delta

## ADDED Requirements

### Requirement: Descriptor loading
The subcommand SHALL treat an `<openapi-spec>` argument that starts with `http://` or `https://` as a remote URL and any other argument as a local file path.
A relative file path SHALL be resolved against the current working directory.
A remote descriptor SHALL be fetched with an HTTP GET request.
The descriptor SHALL be accepted in JSON or YAML format.
External `$ref`s SHALL be resolved relative to the document that contains them, whether they point to a local file or a remote URL.

#### Scenario: Local JSON descriptor
- **WHEN** `api.json` in the current directory is a valid OpenAPI 3.0 descriptor in JSON
- **AND** the user runs `k6 x openapi api.json`
- **THEN** the command succeeds and writes the client library generated from `api.json` to stdout

#### Scenario: Local YAML descriptor
- **WHEN** `api.yaml` contains the same descriptor as `api.json`, written in YAML
- **AND** the user runs `k6 x openapi api.yaml`
- **THEN** the command writes the same output as `k6 x openapi api.json`

#### Scenario: Remote descriptor
- **WHEN** `https://example.test/api.yaml` serves a valid OpenAPI descriptor
- **AND** the user runs `k6 x openapi https://example.test/api.yaml`
- **THEN** the command writes the same output as for the same descriptor loaded from a local file

#### Scenario: External reference to a local file
- **WHEN** `api.yaml` references a schema with `$ref: schemas.yaml#/Pet`, and `schemas.yaml` is in the same directory as `api.yaml`
- **AND** the user runs `k6 x openapi api.yaml` from a different directory, passing the path to `api.yaml`
- **THEN** the generated client contains the `Pet` type defined in `schemas.yaml`

#### Scenario: External reference in a remote descriptor
- **WHEN** `https://example.test/api.yaml` references `$ref: schemas.yaml#/Pet`
- **AND** the user runs `k6 x openapi https://example.test/api.yaml`
- **THEN** the subcommand fetches `https://example.test/schemas.yaml` and the generated client contains the `Pet` type

### Requirement: Descriptor load failures
The subcommand SHALL fail with a non-zero exit code and an error message that names the descriptor if the descriptor cannot be loaded.
A descriptor cannot be loaded if the file does not exist or cannot be read, the remote server responds with a 4xx or 5xx status, the remote request does not complete within 30 seconds, the content is neither valid JSON nor valid YAML, or a `$ref` cannot be resolved.
On a load failure the subcommand SHALL write nothing to stdout and SHALL NOT create or change the `--out` file.

#### Scenario: Remote descriptor not found
- **WHEN** `https://example.test/missing.yaml` responds with status 404
- **AND** the user runs `k6 x openapi https://example.test/missing.yaml`
- **THEN** the command exits with a non-zero exit code and the error names the URL and the status

#### Scenario: Remote descriptor times out
- **WHEN** `https://example.test/slow.yaml` does not respond within 30 seconds
- **AND** the user runs `k6 x openapi https://example.test/slow.yaml`
- **THEN** the command exits with a non-zero exit code after about 30 seconds

#### Scenario: Descriptor is not JSON or YAML
- **WHEN** `api.yaml` contains text that is neither valid JSON nor valid YAML
- **AND** the user runs `k6 x openapi api.yaml`
- **THEN** the command exits with a non-zero exit code and nothing is written to stdout

#### Scenario: Unresolvable reference
- **WHEN** `api.yaml` contains `$ref: '#/components/schemas/Missing'` and no such schema exists
- **AND** the user runs `k6 x openapi api.yaml`
- **THEN** the command exits with a non-zero exit code and the error names the missing component `Missing`

#### Scenario: Load failure leaves the out file unchanged
- **WHEN** `client.ts` exists with some content
- **AND** the user runs `k6 x openapi does-not-exist.yaml --out client.ts`
- **THEN** the command exits with a non-zero exit code and `client.ts` keeps its content

### Requirement: Supported OpenAPI versions
The subcommand SHALL accept descriptors whose `openapi` field has major version 3.
Descriptor features that the generated client does not represent SHALL be ignored.
Swagger 2.0 descriptors are not supported: the subcommand SHALL fail with a non-zero exit code and an error stating that Swagger 2.0 is not supported if the descriptor has a `swagger` field.
Any other descriptor without an `openapi` field of major version 3 SHALL fail with a non-zero exit code and an error stating the unsupported version.

#### Scenario: OpenAPI 3.1 descriptor
- **WHEN** `api.yaml` is a valid OpenAPI 3.1 descriptor
- **AND** the user runs `k6 x openapi api.yaml`
- **THEN** the command succeeds and writes the generated client library to stdout

#### Scenario: Swagger 2.0 descriptor
- **WHEN** `api.yaml` contains `swagger: "2.0"`
- **AND** the user runs `k6 x openapi api.yaml`
- **THEN** the command exits with a non-zero exit code, reports that Swagger 2.0 is not supported, and writes nothing to stdout

#### Scenario: Unsupported OpenAPI version
- **WHEN** `api.yaml` contains `openapi: "4.0.0"`
- **AND** the user runs `k6 x openapi api.yaml`
- **THEN** the command exits with a non-zero exit code and reports that version `4.0.0` is not supported

### Requirement: Warnings
The subcommand SHALL report each problem that does not prevent generation as one warning-level entry through k6's logger, and SHALL still succeed.
Such problems are OpenAPI validation errors of a descriptor that loads, and descriptor constructs that the generated client skips or approximates.
Warnings SHALL NOT be written to the `--out` file, and SHALL NOT be written to stdout unless the user directs k6's log output there with `--log-output=stdout`.

#### Scenario: Descriptor fails validation
- **WHEN** `api.yaml` loads but fails OpenAPI validation, for example because a schema's `example` does not match its `pattern`
- **AND** the user runs `k6 x openapi api.yaml`
- **THEN** the command succeeds, writes the generated client library to stdout, and logs a warning that describes the validation error

#### Scenario: Warnings do not reach the out file
- **WHEN** `api.yaml` produces a warning
- **AND** the user runs `k6 x openapi api.yaml --out client.ts`
- **THEN** the warning is logged, and `client.ts` contains the same content that `k6 x openapi api.yaml` writes to stdout

## MODIFIED Requirements

### Requirement: Command arguments
The subcommand SHALL require exactly one positional argument: the location of an OpenAPI descriptor, given as a local file path or an `http` or `https` URL.

#### Scenario: Missing argument
- **WHEN** the user runs `k6 x openapi` without arguments
- **THEN** the command exits with a non-zero exit code, reports that one argument is required, and writes nothing to stdout

#### Scenario: Too many arguments
- **WHEN** the user runs `k6 x openapi a.yaml b.yaml`
- **THEN** the command exits with a non-zero exit code, reports that one argument is required, and writes nothing to stdout

#### Scenario: Descriptor path does not exist
- **WHEN** the user runs `k6 x openapi does-not-exist.yaml`
- **THEN** the command exits with a non-zero exit code, the error names `does-not-exist.yaml`, and nothing is written to stdout

### Requirement: Generated output
The subcommand SHALL produce a single output per run.
By default the output SHALL contain only the client library generated from the descriptor, as specified by the `generated-client` capability.
With `--example`, the output SHALL contain the client library followed by a k6 test script whose default function is declared `async`, so that calls to the client's methods can be awaited in it, constructs the generated client, and calls no operation.
The example SHALL pass the descriptor's first server URL, with server variables replaced by their default values, as the client's base URL if that URL is an absolute `http` or `https` URL, and `http://localhost` otherwise.
The subcommand SHALL generate the complete output before writing any of it.

#### Scenario: Default output
- **WHEN** the user runs `k6 x openapi api.yaml`
- **THEN** the command succeeds and stdout contains the generated client library and no default function

#### Scenario: Output with example
- **WHEN** the user runs `k6 x openapi api.yaml --example`
- **THEN** stdout contains the generated client library followed by `export default async function` that constructs the generated client class

#### Scenario: Example uses the first server URL
- **WHEN** the first entry of `servers` in `api.yaml` is `https://{region}.example.test/v1` and the `region` variable has the default `eu`
- **AND** the user runs `k6 x openapi api.yaml --example`
- **THEN** the example constructs the client with base URL `https://eu.example.test/v1`

#### Scenario: Example without an absolute server URL
- **WHEN** `api.yaml` has no `servers`, or its first server URL is relative
- **AND** the user runs `k6 x openapi api.yaml --example`
- **THEN** the example constructs the client with base URL `http://localhost`

#### Scenario: Example output runs in k6
- **WHEN** the user saves the output of `k6 x openapi api.yaml --example` to `script.ts`
- **AND** runs `k6 run script.ts` with the same k6 binary
- **THEN** k6 exits with exit code 0

### Requirement: Output destination
The subcommand SHALL write the output to stdout unless `--out <file>` (short `-o`) is given.
With `--out`, the subcommand SHALL write the output to that file instead, write nothing to stdout, and overwrite the file if it exists.
A relative `--out` path SHALL be resolved against the current working directory.
The subcommand SHALL fail with a non-zero exit code, without creating or changing any file, if the `--out` path does not end in `.ts`, its parent directory does not exist, or the path is a directory.

#### Scenario: Write to file
- **WHEN** the user runs `k6 x openapi api.yaml --example --out script.ts`
- **THEN** `script.ts` contains the same content that `k6 x openapi api.yaml --example` writes to stdout
- **AND** nothing is written to stdout

#### Scenario: Existing file is overwritten
- **WHEN** `script.ts` exists with other content
- **AND** the user runs `k6 x openapi api.yaml --out script.ts`
- **THEN** `script.ts` contains only the content that `k6 x openapi api.yaml` writes to stdout

#### Scenario: Out path without .ts extension
- **WHEN** the user runs `k6 x openapi api.yaml --out script.js`
- **THEN** the command exits with a non-zero exit code, reports that the output file must have a `.ts` extension, and `script.js` is not created

#### Scenario: Parent directory does not exist
- **WHEN** the user runs `k6 x openapi api.yaml --out missing/script.ts` and `missing` does not exist
- **THEN** the command exits with a non-zero exit code and `missing` is not created

#### Scenario: Out path is a directory
- **WHEN** `out.ts` is an existing directory
- **AND** the user runs `k6 x openapi api.yaml --out out.ts`
- **THEN** the command exits with a non-zero exit code and reports that the path is a directory
