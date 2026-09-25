# xk6-subcommand-openapi

**Generate a k6 client library from an OpenAPI descriptor, as a native `k6` subcommand.**

This is a `k6` extension that will add a subcommand for generating a k6 test client library from an OpenAPI descriptor. It is implemented in Go, so it runs as part of a `k6` binary with no Node.js dependency. The goal is similar to [`grafana/openapi-to-k6`](https://github.com/grafana/openapi-to-k6), but this is a new, independent implementation, not a port. It does not aim for output or API compatibility with openapi-to-k6's generated code.

## Status

Early stage. This repository currently has no functional code. Development follows a spec-first process: a change is proposed, reviewed, and only then implemented. Proposals and specs live under [`openspec/`](openspec/). See [AGENTS.md](AGENTS.md) for the guardrails this implies.

## License

AGPL-3.0. See [LICENSE.md](LICENSE.md).
