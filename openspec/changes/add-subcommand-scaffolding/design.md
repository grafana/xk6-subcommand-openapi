# Design

## Context

The repository only contains docs, OpenSpec setup, `.editorconfig`, `.gitignore`, and a `Makefile` with lint targets that hardcode the `grafana/k6-ci` ref (v0.5.0).
The reference layout is `grafana/xk6-subcommand-example` (the `xk6 new` template) and `grafana/xk6-subcommand-httpbin`.
The integration test and CI approach are modeled on `grafana/xk6-kafka`.
The k6-ci golangci config forbids `os.*` and `fmt.Print*`, so file and console I/O must go through k6's `state.GlobalState` (`FS`, `Getwd`, command output writers).

## Goals / Non-Goals

**Goals:**
- A layout that matches the upstream template, so k6 maintainers recognize it and `xk6 lint` passes.
- Two test levels: fast in-process Go tests of the command, and `make it` that proves the real k6 binary integration works.
- Local and CI runs use the same `make` targets.

**Non-Goals:**
- Release workflow (`release.yml`), devcontainer, Renovate, `cdo`-generated Makefile, and `make security` (gosec, govulncheck). These can be added in later changes.
- Any OpenAPI parsing or real client generation.

## Decisions

### Layout: single root package, split like the template
Files at the repository root in package `openapi`:
- `register.go`: `init()` calls `subcommand.RegisterExtension("openapi", newSubcommand)`.
- `cmd.go`: `newSubcommand(gs *state.GlobalState) *cobra.Command` with `Use: "openapi <openapi-spec>"`, `Args: cobra.ExactArgs(1)`, a boolean `--example` flag, a string `--out`/`-o` flag, and `Long` from an embedded `help.md`.
- `generate.go`: builds the output as bytes, which is the empty client followed by the embedded example script when `--example` is set.
- `templates/example.ts`: embedded with `//go:embed`, so the example script is a real `.ts` file that editors can treat as TypeScript.

Alternative considered: an `internal/` package from the start.
Rejected for now because the template uses a flat root package and there is too little code to justify the split; it can move when real generation lands.

### k6 dependency: `go.k6.io/k6/v2` v2.3.0, unpinned k6 in `xk6 build`
`go.mod` requires `go.k6.io/k6/v2 v2.3.0`, matching the template's move to k6 v2.
`make build` and `make it` call `xk6 build` without a k6 version, so xk6 uses the latest k6 release.
Because Go uses minimal version selection, the binary never uses a k6 older than the one in `go.mod`.
Alternative considered: `xk6 build master` as in the template.
Rejected because building against an unreleased k6 makes local and CI results depend on k6's main branch state.

### Output: stdout by default, `--out` through `gs.FS`
The output is generated in full before anything is written, so an error never leaves partial output.
Without `--out`, the output is written to `cmd.OutOrStdout()`.
With `--out`, the command validates the path before writing, in this order:
1. The path must end in `.ts` (case-sensitive), because k6 selects TypeScript handling by file extension.
2. A relative path is resolved against `gs.Getwd()`.
3. The path must not be an existing directory, and its parent directory must exist (checked with `gs.FS.Stat`); missing directories are not created.
4. The output is written with `fsext.WriteFile`, which truncates an existing file.

Going through `gs.FS` and the command's output writer satisfies the k6-ci `forbidigo` rules and lets tests use k6's in-memory filesystem and a captured stdout.
Alternative considered: writing both a client file and an example file into an output directory.
Rejected in review: a single output per run is enough while there is no real generation, and it keeps the command pipeable.

### In-process tests: execute the cobra command directly
Following `xk6-subcommand-httpbin`, tests in `cmd_test.go` build the command with `newSubcommand(ts.GlobalState)`, where `ts` comes from the public `go.k6.io/k6/v2/cmd/tests.NewGlobalTestState(t)`.
Each test sets arguments with `cmd.SetArgs(...)`, captures stdout with `cmd.SetOut(buf)`, runs `cmd.ExecuteContext(ctx)`, and asserts on the captured stdout, the in-memory `ts.FS` for `--out` scenarios, and the returned error.
This covers every scenario in the spec except those that need the real k6 root command (`k6 x --help`) or `k6 run`; those are covered by `make it`.
Alternatives considered: running the full k6 root command in-process via a `go:linkname` to k6's internal `ExecuteWithGlobalState`, or re-executing the test binary through the public `cmd.Execute()`.
Rejected because the upstream examples do not do this, and `make it` already verifies the k6 integration with a real binary.

### `make it`: real binary, real k6 run
`make it` runs these steps, failing on the first error:
1. `xk6 build --with github.com/grafana-experimental/xk6-subcommand-openapi=. --output build/it/k6`
2. `build/it/k6 x --help` and check that the output lists `openapi`.
3. `build/it/k6 x openapi unused.yaml` and check that stdout is empty; `unused.yaml` does not exist, which also covers the "descriptor not read" scenario.
4. `build/it/k6 x openapi unused.yaml --example --out build/it/script.ts`.
5. `build/it/k6 run build/it/script.ts` and check that the output contains `Hello, World!`.

`make it` writes the script with `--out`, not stdout redirection, so the file the real binary writes is the one k6 runs.
Stdout output with `--example` is covered by the in-process Go tests.

The steps live in a POSIX shell script `test/it.sh` called by the Makefile, because multi-step checks with clear failure messages are hard to read inside a Makefile recipe.
The `build/` directory is already gitignored.
Alternative considered: `xk6 test`, as in xk6-kafka.
Rejected because `xk6 test` runs JS test scripts against a module extension and cannot invoke a subcommand before running the script.

### Makefile: hand-written, template target names
Targets: `help` (default), `build`, `test`, `it`, `lint`, `format`, `clean`.
- `build`: `xk6 build --with <module>=.`, producing `./k6`.
- `test`: `go test -count 1 -race -timeout 60s ./...`.
- `lint`: the existing golangci flow with the k6-ci config and optional `.golangci.patch`, followed by `xk6 lint`.
The k6-ci ref is read from `.github/workflows/all.yml` with the same `grep` the template uses, so Renovate or a manual bump updates CI and local lint together.

### CI: template caller plus xk6-kafka-style integration workflow
- `.github/workflows/all.yml`: copy of the template's thin caller of `grafana/k6-ci/.github/workflows/all.yml`, pinned to v0.5.2 by SHA, with `k6-ci-ref` set to the same SHA.
- `.github/workflows/integration.yml`: modeled on xk6-kafka's `integration.yml`, with the Go version resolved the same way as k6-ci.
A `configure` job runs `grafana/k6-ci/.github/actions/go-versions` pinned to the same SHA as `all.yml`, as the template's `release.yml` does.
The `integration` job then runs checkout, `actions/setup-go` with `go-version: ${{ needs.configure.outputs.current }}`, `grafana/setup-xk6` pinned by SHA with a pinned xk6 version, and `make it`.
This keeps the integration job on the same Go version as the k6-ci jobs.
Alternative considered: `go-version-file: go.mod`, as in xk6-kafka.
Rejected because it can diverge from the Go version k6-ci tests with.
It runs on pull requests, pushes to `main`, tags, and manual dispatch, with `contents: read` permissions.
Only the xk6 tool version is pinned, not the k6 version.

## Risks / Trade-offs

- [Unpinned k6 means a new k6 release can break `make it` without any change in this repo] → This is intended, since it surfaces incompatibilities early; the failure shows up in the integration workflow.
- [`k6 x --help` output format could change between k6 versions] → The check only greps for the subcommand name.
- [`fsext.WriteFile` and `cmd/tests` are k6 APIs that can change across major versions] → Both are public packages in k6 v2, and a breaking change fails at compile time.
- [The k6-ci SHA is pinned in both `all.yml` and `integration.yml`] → Renovate updates both; a manual bump must change both to the same SHA.
- [The output file must end in `.ts` for k6 to treat it as TypeScript] → `--out` fails with an error for any other extension; for stdout the target cannot be checked, so the help text and README say to redirect to a `.ts` file.
- [Shell redirection empties the target file even if the command fails] → The help text and README recommend `--out` when writing files, because `--out` writes nothing on error.
- [The ignored `<openapi-spec>` argument means users get no error for a wrong path] → Accepted for this placeholder change; a later change will read and validate it.
