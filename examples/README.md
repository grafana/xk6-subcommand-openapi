# Example: load testing QuickPizza

This example load tests [QuickPizza](https://github.com/grafana/quickpizza), Grafana's demo application, with a client library generated from its OpenAPI descriptor.

| File | Description |
|------|-------------|
| [`quickpizza.yaml`](quickpizza.yaml) | A copy of QuickPizza's OpenAPI descriptor. |
| [`quickpizza-client.ts`](quickpizza-client.ts) | The client library generated from `quickpizza.yaml`. |
| [`script.ts`](script.ts) | A k6 test script that asks for a pizza recommendation and then fetches the recommended pizza. |

## Run it

Build a k6 binary with this extension, as described in the [README](../README.md), and run these commands in this directory:

```sh
k6 x openapi quickpizza.yaml --out quickpizza-client.ts
k6 run script.ts
```

The first command regenerates the client library, which is already committed here.
It logs two warnings, because QuickPizza's descriptor contains two examples that do not match their schemas.

The script runs against the public server at https://quickpizza.grafana.com by default.
To test a QuickPizza instance of your own, such as one started with `docker run --rm -p 3333:3333 ghcr.io/grafana/quickpizza-local:latest`, set `BASE_URL`:

```sh
k6 run -e BASE_URL=http://localhost:3333 script.ts
```
