# xk6-subcommand-openapi

A `k6` extension that adds a subcommand for generating a k6 client library from an OpenAPI descriptor. It is implemented in Go and ships as part of a `k6` binary, with no Node.js dependency.

## Relationship to grafana/openapi-to-k6

This project aims for the same overall goal as [`grafana/openapi-to-k6`](https://github.com/grafana/openapi-to-k6): turning an OpenAPI descriptor into a k6 test client. It is a new, independent implementation, not a port or a rewrite of that project.

**Non-goal:** JS-API compatibility with openapi-to-k6's generated code. Do not treat openapi-to-k6's output shape as a compatibility target when designing this extension.

## Process gate: spec before code

This project follows spec-driven development. Do not add implementation Go code until a spec exists and is approved for that piece of work.

## OpenSpec workflow

Specs and changes live under `openspec/`. Start a change with `/opsx:propose` (Claude Code), `$openspec-propose` (Codex), or `/opsx-propose` (GitHub Copilot). Once a proposal is approved, implement it with the matching apply command. Project context shown to these tools lives in `openspec/config.yaml`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full PR structure and status tracking.

## Checks before a Go change is done

A Go code change is not done until all of these pass:

- `make test`: the in-process Go tests.
- `make it`: the integration test against a real k6 binary built with xk6.
- `make lint`: golangci-lint, configured the same way k6 configures it, plus `xk6 lint`.
