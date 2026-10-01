# openapi-command Specification

## Purpose
Provides the `k6 x openapi` subcommand, which turns an OpenAPI descriptor into a k6 client library, optionally combined with an example k6 script.

## Requirements

### Requirement: Subcommand registration
A k6 binary built with this extension SHALL expose a subcommand named `openapi` under `k6 x`.

#### Scenario: Subcommand is listed
- **WHEN** the user runs `k6 x --help` with a k6 binary that includes this extension
- **THEN** the output lists `openapi`

#### Scenario: Subcommand help
- **WHEN** the user runs `k6 x openapi --help`
- **THEN** the output shows the usage `openapi <openapi-spec>` and describes the `--example` and `--out` flags

### Requirement: Command arguments
The subcommand SHALL require exactly one positional argument: the path to an OpenAPI descriptor.
The descriptor SHALL NOT be read, and its existence SHALL NOT be checked.

#### Scenario: Missing argument
- **WHEN** the user runs `k6 x openapi` without arguments
- **THEN** the command exits with a non-zero exit code, reports that one argument is required, and writes nothing to stdout

#### Scenario: Too many arguments
- **WHEN** the user runs `k6 x openapi a.yaml b.yaml`
- **THEN** the command exits with a non-zero exit code, reports that one argument is required, and writes nothing to stdout

#### Scenario: Descriptor path does not exist
- **WHEN** the user runs `k6 x openapi does-not-exist.yaml`
- **THEN** the command succeeds

### Requirement: Generated output
The subcommand SHALL produce a single output per run.
By default the output SHALL contain only the client library, which is empty in this version.
With `--example`, the output SHALL contain the client library followed by a k6 test script whose default function logs `Hello, World!` and that imports no module.

#### Scenario: Default output
- **WHEN** the user runs `k6 x openapi api.yaml`
- **THEN** the command succeeds and writes nothing to stdout

#### Scenario: Output with example
- **WHEN** the user runs `k6 x openapi api.yaml --example`
- **THEN** stdout contains a k6 script with a default function that logs `Hello, World!`

#### Scenario: Example output runs in k6
- **WHEN** the user saves the output of `k6 x openapi api.yaml --example` to `script.ts`
- **AND** runs `k6 run script.ts` with the same k6 binary
- **THEN** k6 exits with exit code 0 and logs `Hello, World!`

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
- **THEN** `script.ts` is empty

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
