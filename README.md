# xk6-subcommand-openapi

**Generate a k6 client library from an OpenAPI descriptor, as a native `k6` subcommand.**

This is a `k6` extension that will add a subcommand for generating a k6 test client library from an OpenAPI descriptor. It is implemented in Go, so it runs as part of a `k6` binary with no Node.js dependency. The goal is similar to [`grafana/openapi-to-k6`](https://github.com/grafana/openapi-to-k6), but this is a new, independent implementation, not a port. It does not aim for output or API compatibility with openapi-to-k6's generated code.

## Status

Early stage.
The subcommand generates a typed k6 client library from an OpenAPI 3.x descriptor.
Development follows a spec-first process: a change is proposed, reviewed, and only then implemented.
Proposals and specs live under [`openspec/`](openspec/).
See [AGENTS.md](AGENTS.md) for the guardrails this implies.

## Usage

```sh
k6 x openapi <openapi-spec> [--example] [--out <file>]
```

`<openapi-spec>` is the location of the OpenAPI descriptor: a local file path, or an `http` or `https` URL.
A relative path is resolved against the current directory.
The descriptor can be JSON or YAML.
External `$ref`s are resolved relative to the document that contains them, whether they point to a local file or a URL.

OpenAPI 3.0, 3.1, and 3.2 descriptors are supported.
Swagger 2.0 descriptors are not supported and fail with an error; convert them to OpenAPI 3 first.
Descriptor features that the client does not represent are ignored.

| Flag | Description |
|------|-------------|
| `--example` | Append a k6 test script that constructs the client to the client library, so the output is a runnable k6 script. |
| `-o`, `--out <file>` | Write the output to `<file>` instead of stdout. |

The output is written to stdout by default.
With `--out`, nothing is written to stdout, and an existing file is overwritten.
The `--out` file must have a `.ts` extension, because k6 selects TypeScript handling by file extension.
Its parent directory must exist; missing directories are not created.

When redirecting stdout, also use a `.ts` file.
Prefer `--out` for writing files: shell redirection empties the target file even if the command fails, while `--out` writes nothing on error.

### Warnings

A descriptor that loads but fails OpenAPI validation still generates a client.
Each validation error is logged as a warning through k6's logger, and so is each descriptor construct that the client skips or approximates, such as a cookie parameter or a renamed type.
Warnings never reach the `--out` file, and reach stdout only with `--log-output=stdout`.

### Example script

With `--example`, the output ends with an `async` default function that constructs the client and calls no operation:

```ts
export default async function (): Promise<void> {
  const client = new PetStoreClient({ baseUrl: "https://petstore.example.test/v1" });
  // Call the client's methods with await, for example: const result = await client.<method>(...);
}
```

The base URL is the descriptor's first server URL, with server variables replaced by their defaults, if it is an absolute `http` or `https` URL, and `http://localhost` otherwise.

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

## The generated client

The client library is a single TypeScript module that k6 runs without a build step.
It exports one type for each schema the operations use, the options and result types of each method, and one client class named after `info.title`, such as `PetStoreClient`.

For this descriptor excerpt:

```yaml
openapi: 3.0.3
info: { title: Pet Store, version: 1.0.0 }
paths:
  /pets:
    get:
      operationId: listPets
      parameters:
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: The pets.
          content:
            application/json:
              schema: { type: array, items: { $ref: "#/components/schemas/Pet" } }
    post:
      operationId: createPet
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/Pet" }
      responses:
        "201":
          description: Created.
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Pet" }
  /pets/{petId}:
    get:
      operationId: getPet
      parameters:
        - { name: petId, in: path, required: true, schema: { type: integer } }
      responses:
        "200":
          description: The pet.
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Pet" }
        "404":
          description: Not found.
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Error" }
components:
  schemas:
    Pet:
      type: object
      required: [name]
      properties:
        id: { type: integer }
        name: { type: string }
    Error:
      type: object
      properties:
        message: { type: string }
```

the client has these methods:

```ts
listPets(options?: ListPetsOptions): Promise<ListPetsResult>
createPet(options: CreatePetOptions): Promise<CreatePetResult>
getPet(petId: number, options?: GetPetOptions): Promise<GetPetResult>
```

Generate the client into `client.ts` with `k6 x openapi api.yaml --out client.ts` and use it from a test script:

```ts
import { check } from "k6";
import { PetStoreClient } from "./client.ts";

const client = new PetStoreClient({
  baseUrl: "https://petstore.example.test/v1",
  commonK6Params: { headers: { Authorization: "Bearer my-token" } },
});

export default async function () {
  const created = await client.createPet({ body: { name: "Rex" } });
  check(created, { "pet created": (res) => res.ok });

  const res = await client.getPet(1, { k6Params: { tags: { name: "getPet" } } });
  if (res.ok) {
    console.log(res.data.name); // res.data is a Pet
  } else {
    console.log(res.response.status, res.data?.message); // res.data is an Error, or undefined without a response
  }

  const [first, second] = await Promise.all([client.getPet(1), client.getPet(2)]);
  check(first, { "first pet found": (r) => r.ok });
  check(second, { "second pet found": (r) => r.ok });

  const pets = await client.listPets({ query: { limit: 10 } });
  check(pets, { "pets listed": (r) => r.ok });
}
```

### Constructor

The constructor takes the required `baseUrl` and optional `commonK6Params`, the k6 [`Params`](https://grafana.com/docs/k6/latest/javascript-api/k6-http/params/) for every request.
Trailing slashes are removed from `baseUrl`.

### Methods

There is one method per operation, named after its `operationId` in camelCase, or after the HTTP method and the path, such as `getPetsPetId`.
Every method is `async`: it sends its request with k6's non-blocking `http.asyncRequest` and returns a promise, so always `await` it.
Without `await`, the result is a `Promise`, whose `ok` is `undefined`.
Several calls can run concurrently with `Promise.all`.

A method takes the path parameters as positional arguments, in path order, followed by an `options` object with these members:

- `query`: the query parameters, sent as a query string, with one `name=value` pair per array element.
- `headers`: the header parameters.
- `body`: the request body, encoded according to its media type: JSON with `JSON.stringify`, `application/x-www-form-urlencoded` like the query string, `multipart/form-data` as a multipart body whose file fields take `http.file(...)` or an `ArrayBuffer`, and any other media type sent unchanged as a `string` or `ArrayBuffer`.
- `k6Params`: k6 `Params` for this request, merged over `commonK6Params`; `headers`, `cookies`, and `tags` are merged key by key.

A member is required only if the descriptor marks a parameter or the request body as required, and `options` itself is optional if all its members are.

### Result

A method's promise resolves to `{ ok, data, response }`:

- `ok` is `true` if the status is from 200 to 299, and `false` otherwise, including status 0 when k6 received no response.
- `data` is the response body decoded according to the response's `Content-Type` header: parsed as JSON for `application/json` and media types ending in `+json`, and the unparsed body for any other or a missing `Content-Type`; it is `undefined` for an empty body.
- `response` is the k6 [`Response`](https://grafana.com/docs/k6/latest/javascript-api/k6-http/response/).

Checking `ok` narrows the type of `data` to the union of the documented success types or of the documented error types.
If a body whose `Content-Type` says JSON is not valid JSON, the promise rejects with the parse error.
If a request fails and the k6 `throw` option is set, the promise rejects with k6's error.

The type of `data` describes documented responses only.
For an undocumented response, such as a gateway error page, or a response with another `Content-Type` than documented, `data` holds whatever the decoding above produces, which may not match its type.

### Credentials and security schemes

The descriptor's security schemes and requirements are ignored, and the client sends no credentials on its own.
Send credentials through `commonK6Params` for every request, or through `k6Params` for one request:

- `http` `basic` and `bearer`, and API keys in a header: set the header, such as `headers: { Authorization: "Bearer my-token" }`.
- API keys in a cookie: set `cookies`.
- `oauth2` and `openIdConnect`: obtain a token yourself and send it as a header.
- `http` `digest`: put the percent-encoded credentials into `baseUrl`, as in `http://user:pass@host`, and set `auth: "digest"` in `commonK6Params`; k6 supports only the MD5 algorithm.
- `mutualTLS`: use k6's [`tlsAuth`](https://grafana.com/docs/k6/latest/using-k6/k6-options/reference/#tls-auth) option.

API keys in the query and `http` schemes other than `basic`, `bearer`, and `digest` cannot be sent yet.
The subcommand logs a warning for each such scheme that an operation uses.

### Known limitations

- The client imports `URL`, `URLSearchParams`, and `FormData` from [jslib.k6.io](https://jslib.k6.io), so running a script that contains it needs network access to jslib.k6.io. A later change will remove this requirement.
- Generating credential options from security schemes is not supported yet.
- Generated names can collide with identifiers the client library uses, such as a schema named `Response` or `Promise`, or an operation named `constructor`, and the client library may then be invalid.

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
It checks that `k6 x --help` lists `openapi`, then generates every descriptor in [`test/testdata`](test/testdata/) with `--example --out` and runs each script with `k6 run`.
A run fails if k6 exits with an error or logs one.
The generated scripts import from jslib.k6.io, so `make it` needs network access.
In CI, the `integration` workflow ([`.github/workflows/integration.yml`](.github/workflows/integration.yml)) runs `make it` with the latest xk6 and k6 releases.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how a change gets proposed, reviewed, and implemented.

## License

AGPL-3.0. See [LICENSE.md](LICENSE.md).
