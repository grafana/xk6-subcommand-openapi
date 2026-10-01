# Proposal

## Why

The repository has no Go code, so there is nothing to build, run, or test yet.
Before implementing any OpenAPI parsing or code generation, we need a working k6 subcommand extension skeleton with a proven build, test, and CI loop, so that later changes only add behavior instead of also inventing infrastructure.

## What Changes

- Add a Go module `github.com/grafana-experimental/xk6-subcommand-openapi` that depends on `go.k6.io/k6/v2` at v2.3.0 and registers a k6 subcommand extension named `openapi`, matching the repository name.
- The command is invoked as `k6 x openapi <openapi-spec> [--example] [--out <file>]`.
- The `<openapi-spec>` argument is accepted but not read in this change.
- Each run produces a single output: by default only the client library, which is an empty placeholder in this change.
- `--example` appends a hello-world k6 script to the same output, so the output becomes a runnable k6 script.
- The output goes to stdout by default; `--out <file>` writes it to a `.ts` file instead, overwriting an existing file.
- Add in-process Go tests that execute the subcommand's command with arguments and verify its stdout and the file written with `--out`, following the pattern used by `grafana/xk6-subcommand-httpbin`.
- Add a hand-written `Makefile` with `build`, `test`, `it`, `lint`, `format`, `clean`, and `help` targets.
- `make it` builds a custom k6 binary with the local extension via `xk6`, writes the subcommand's `--example` output to a `.ts` file with `--out`, and runs that file with the same binary.
- Add GitHub Actions workflows: a thin caller of the shared `grafana/k6-ci` workflow, and an integration workflow that runs `make it`, modeled on `grafana/xk6-kafka`.
- The k6 version used by `xk6 build` is not pinned, so builds use the latest k6 release.

## Capabilities

### New Capabilities
- `openapi-command`: The `k6 x openapi` subcommand, its argument and flags, the output it generates, and where that output is written.

### Modified Capabilities

None.

## Impact

- New files: `go.mod`, `go.sum`, Go sources and tests at the repository root, a `help.md` for command help, `.github/workflows/`.
- `Makefile` is rewritten; the existing `lint` behavior is kept, but the k6-ci ref is read from the CI workflow instead of being hardcoded.
- `README.md` and `AGENTS.md` gain developer instructions (build, test, integration test, running the subcommand locally).
- New dependencies: `go.k6.io/k6/v2` v2.3.0 and `github.com/spf13/cobra`, plus `github.com/stretchr/testify` for tests.
- Developer prerequisites: Go, `xk6` (for `make build`, `make it`, and `make lint`), and `gosec` and `govulncheck` (for `xk6 lint` in `make lint`).
