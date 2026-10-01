# xk6-subcommand-openapi

**Generate a k6 client library from an OpenAPI descriptor, as a native `k6` subcommand.**

This is a `k6` extension that will add a subcommand for generating a k6 test client library from an OpenAPI descriptor. It is implemented in Go, so it runs as part of a `k6` binary with no Node.js dependency. The goal is similar to [`grafana/openapi-to-k6`](https://github.com/grafana/openapi-to-k6), but this is a new, independent implementation, not a port. It does not aim for output or API compatibility with openapi-to-k6's generated code.

## Status

Early stage.
The subcommand exists, but it does not read the OpenAPI descriptor yet and generates an empty client library.
Development follows a spec-first process: a change is proposed, reviewed, and only then implemented.
Proposals and specs live under [`openspec/`](openspec/).
See [AGENTS.md](AGENTS.md) for the guardrails this implies.

## Usage

```sh
k6 x openapi <openapi-spec> [--example] [--out <file>]
```

`<openapi-spec>` is the path to the OpenAPI descriptor.
It is not read yet, so the generated client library is empty.

| Flag | Description |
|------|-------------|
| `--example` | Append a hello-world k6 test script to the client library, so the output is a runnable k6 script. |
| `-o`, `--out <file>` | Write the output to `<file>` instead of stdout. |

The output is written to stdout by default.
With `--out`, nothing is written to stdout, and an existing file is overwritten.
The `--out` file must have a `.ts` extension, because k6 selects TypeScript handling by file extension.
Its parent directory must exist; missing directories are not created.

When redirecting stdout, also use a `.ts` file.
Prefer `--out` for writing files: shell redirection empties the target file even if the command fails, while `--out` writes nothing on error.

For example, to generate the example script and run it:

```sh
k6 x openapi api.yaml --example --out script.ts
k6 run script.ts
```

### Running locally without a build

During development, [`xk6 x`](https://github.com/grafana/xk6) builds a temporary k6 binary with the extension in the current directory and runs the subcommand with it.
Use `--` to separate xk6 flags from the subcommand's arguments:

```sh
xk6 x -- openapi api.yaml --example
```

## Development

### Prerequisites

- [Go](https://go.dev/doc/install), in the version required by `go.mod` or newer.
- [xk6](https://github.com/grafana/xk6), used by `make build`, `make it`, and `make lint`.
- [gosec](https://github.com/securego/gosec) and [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck), used by `xk6 lint` in `make lint`:

  ```sh
  go install github.com/securego/gosec/v2/cmd/gosec@latest
  go install golang.org/x/vuln/cmd/govulncheck@latest
  ```

### Make targets

Run `make` (or `make help`) to list the targets.

| Target | Description |
|--------|-------------|
| `make help` | Show the list of targets. This is the default target. |
| `make build` | Build `./k6` with this extension using xk6. |
| `make test` | Run the Go tests. |
| `make it` | Run the integration test against a real k6 binary built with xk6. |
| `make lint` | Run golangci-lint with the [grafana/k6-ci](https://github.com/grafana/k6-ci) config plus `.golangci.patch`, then `xk6 lint` without its `versions` check. |
| `make format` | Format the Go sources. |
| `make update-lint-patch` | Regenerate `.golangci.patch` from a locally edited `build/lint/.golangci.yml`. |
| `make clean` | Remove build output. |

The k6-ci ref used by `make lint` is read from `.github/workflows/all.yml`, so local lint and CI use the same config.

### Integration test

`make it` runs [`test/it.sh`](test/it.sh), which builds `build/it/k6` with the extension using xk6 and checks the subcommand end to end.
It checks that `k6 x --help` lists `openapi`, that the default output is empty, and that the script written with `--example --out` runs with `k6 run` and logs `Hello, World!`.
In CI, the `integration` workflow ([`.github/workflows/integration.yml`](.github/workflows/integration.yml)) runs `make it` with the latest xk6 and k6 releases.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how a change gets proposed, reviewed, and implemented.

## License

AGPL-3.0. See [LICENSE.md](LICENSE.md).
