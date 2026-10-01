# Tasks

## 1. Module and tooling setup

- [x] 1.1 Create `go.mod` for `github.com/grafana-experimental/xk6-subcommand-openapi` requiring `go.k6.io/k6/v2 v2.3.0` and `github.com/spf13/cobra`, and verify `go mod tidy` leaves no diff once the Go sources from group 2 exist
- [x] 1.2 Add `.github/workflows/all.yml` as a thin caller of `grafana/k6-ci/.github/workflows/all.yml` pinned to v0.5.2 by SHA with matching `k6-ci-ref`, copied from `grafana/xk6-subcommand-example`, and verify the file is valid YAML
- [x] 1.3 Rewrite `Makefile` by hand with `help` (default), `build`, `test`, `it`, `lint`, `format`, `clean`, reading the k6-ci ref from `all.yml` and keeping `.golangci.patch` support, and verify `make` prints the target list and `make lint` downloads the v0.5.2 config
- [x] 1.4 Document prerequisites (Go, xk6) and the `make` targets in `README.md`, and update `AGENTS.md` to require `make test` and `make it` to pass before a Go change is done and to drop the note that the repository intentionally has no Go code, and verify the documented targets match `make help`

## 2. Subcommand implementation

- [x] 2.1 Add `register.go` registering `openapi` and `cmd.go` with `newSubcommand`, `Use: "openapi <openapi-spec>"`, `cobra.ExactArgs(1)`, a boolean `--example` flag, a string `--out`/`-o` flag, and `Long` from embedded `help.md` (including the advice to redirect to a `.ts` file and to prefer `--out` when writing files), and verify `go build ./...` succeeds
- [x] 2.2 Add `templates/example.ts` (hello-world default function, no imports) and `generate.go` that builds the output (empty client, plus the embedded example with `--example`) in full, then writes it to stdout or, with `--out`, via `gs.FS` after checking the `.ts` suffix, resolving relative paths with `gs.Getwd`, and rejecting a directory or a missing parent directory, overwriting an existing file, and verify with the tests in 2.3
- [x] 2.3 Add `cmd_test.go` using `go.k6.io/k6/v2/cmd/tests.NewGlobalTestState` and testify's `assert` (and `require` where a failure must stop the test) that runs the command in-process with captured stdout for every scenario in `specs/openapi-command/spec.md` except the `k6 x --help` listing and the `k6 run` scenario, and verify `make test` passes
- [x] 2.4 Document usage (`k6 x openapi <openapi-spec> [--example] [--out <file>]`, stdout as the default, the `.ts` requirement, overwrite behavior, and `xk6 x -- openapi ...` for quick local runs) in `README.md`, and verify the `xk6 x` command runs as written
- [x] 2.5 Verify `make lint` passes, including `xk6 lint` (with its `versions` check disabled until the first release tag), without adding `.golangci.patch` entries

## 3. Integration test

- [x] 3.1 Add `test/it.sh` that builds `build/it/k6` with `xk6 build --with <module>=.`, checks `k6 x --help` lists `openapi`, checks `k6 x openapi unused.yaml` prints nothing, writes the script with `k6 x openapi unused.yaml --example --out build/it/script.ts` (no stdout redirection), and runs `k6 run build/it/script.ts` checking for `Hello, World!`, and verify `make it` passes locally
- [x] 3.2 Verify `make it` fails with a clear message when one of its checks is broken, by temporarily changing the expected log text
- [x] 3.3 Add `.github/workflows/integration.yml` modeled on `grafana/xk6-kafka`'s integration workflow (a `configure` job using `grafana/k6-ci/.github/actions/go-versions` pinned to the same SHA as `all.yml`, then checkout, `setup-go` with `needs.configure.outputs.current`, `grafana/setup-xk6` pinned by SHA with the latest xk6 version, `make it`) with `contents: read` permissions, document it next to `make it` in `README.md`, and verify the workflow passes on the pull request
- [x] 3.4 Verify `git status` shows no untracked build output after `make build`, `make test`, and `make it`, updating `.gitignore` if needed
