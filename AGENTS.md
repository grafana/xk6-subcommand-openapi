# xk6-subcommand-openapi

A `k6` extension that adds a subcommand for generating a k6 client library from an OpenAPI descriptor. It is implemented in Go and ships as part of a `k6` binary, with no Node.js dependency.

## Relationship to grafana/openapi-to-k6

This project aims for the same overall goal as [`grafana/openapi-to-k6`](https://github.com/grafana/openapi-to-k6): turning an OpenAPI descriptor into a k6 test client. It is a new, independent implementation, not a port or a rewrite of that project.

**Non-goal:** JS-API compatibility with openapi-to-k6's generated code. Do not treat openapi-to-k6's output shape as a compatibility target when designing this extension.

## Process gate: spec before code

This project follows spec-driven development. Do not add implementation Go code until a spec exists and is approved for that piece of work. There is no Go code in this repository yet, and that is intentional, not an oversight.

## OpenSpec workflow

Specs and changes live under `openspec/`. Start a change with `/opsx:propose` (Claude Code), `$openspec-propose` (Codex), or `/opsx-propose` (GitHub Copilot). Once a proposal is approved, implement it with the matching apply command. Project context shown to these tools lives in `openspec/config.yaml`.

## Lint

All generated Go code must pass `make lint` (golangci-lint, configured the same way k6 configures it). Run it before considering any Go code change done.
