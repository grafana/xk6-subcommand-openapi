# Contributing

Participation in this project is governed by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Proposing a change

For a bug fix or a small improvement, open a pull request directly.
For a new feature or a change to the generated client's API, open an issue first, so the design can be agreed on before you write code.

## Before opening a pull request

Run `make test`, `make it`, `make lint`, and `make typecheck`, as described in [Development](#development), and make sure they pass.
Update [README.md](README.md) if the change affects how the command or the generated client is used.

## Development

### Prerequisites

- [Go](https://go.dev/doc/install), in the version required by `go.mod` or newer.
- [xk6](https://github.com/grafana/xk6), used by `make build`, `make it`, and `make lint`.
- [Node.js](https://nodejs.org/), used only by `make typecheck`.
- [gosec](https://github.com/securego/gosec) and [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck), used by `xk6 lint` in `make lint`:

  ```sh
  go install github.com/securego/gosec/v2/cmd/gosec@latest
  go install golang.org/x/vuln/cmd/govulncheck@latest
  ```

### Running the subcommand without a build

[`xk6 x`](https://github.com/grafana/xk6) builds a temporary k6 binary with the extension in the current directory and runs the subcommand with it.
Use `--` to separate xk6 flags from the subcommand's arguments:

```sh
xk6 x -- openapi api.yaml --example
```

### Make targets

Run `make` (or `make help`) to list the targets.

| Target | Description |
|--------|-------------|
| `make help` | Show the list of targets. This is the default target. |
| `make build` | Build `./k6` with this extension using xk6. |
| `make test` | Run the Go tests. |
| `make it` | Run the integration test against a real k6 binary built with xk6. |
| `make typecheck` | Type-check the generated client libraries and the example with TypeScript, including the narrowing assertions in `test/typecheck`; needs Node.js. |
| `make lint` | Run golangci-lint with the [grafana/k6-ci](https://github.com/grafana/k6-ci) config plus `.golangci.patch`, then `xk6 lint`. |
| `make format` | Format the Go sources. |
| `make update-lint-patch` | Regenerate `.golangci.patch` from a locally edited `build/lint/.golangci.yml`. |
| `make clean` | Remove build output. |

The k6-ci ref used by `make lint` is read from `.github/workflows/all.yml`, so local lint and CI use the same config.

### Integration test

`make it` runs [`test/it.sh`](test/it.sh), which builds `build/it/k6` with the extension using xk6 and checks the subcommand end to end.
It checks that `k6 x --help` lists `openapi`, then generates every descriptor in [`test/testdata`](test/testdata/) with `--example --out` and runs each script with `k6 run`.
A run fails if k6 exits with an error or logs one.
It then checks that [`examples/quickpizza-client.ts`](examples/quickpizza-client.ts) matches a fresh generation.
The generated scripts import from jslib.k6.io, so `make it` needs network access.
In CI, the `integration` workflow ([`.github/workflows/integration.yml`](.github/workflows/integration.yml)) runs `make it` with the latest xk6 and k6 releases.

### Type check

`make typecheck` installs the pinned TypeScript compiler and the k6 type definitions from [`test/typecheck/package.json`](test/typecheck/package.json) and type-checks every golden client library in [`test/testdata`](test/testdata/) in strict mode.
It also type-checks the [example](examples/) and the `*.check.ts` files in [`test/typecheck`](test/typecheck/), which assert that the result types narrow as specified: the check fails if a narrowed type differs from the expected one, or if a line marked with `@ts-expect-error` compiles.
In CI, the `typecheck` workflow ([`.github/workflows/typecheck.yml`](.github/workflows/typecheck.yml)) runs `make typecheck`.
