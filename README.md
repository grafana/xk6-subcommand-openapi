# xk6-subcommand-openapi

**Generate a typed k6 client library from an OpenAPI descriptor, with a native `k6` subcommand.**

`k6 x openapi` turns an OpenAPI 3.x descriptor into a TypeScript module with one method per operation, which k6 test scripts import and run without a build step.
It runs inside the `k6` binary, with no Node.js dependency.
It is independent of [`grafana/openapi-to-k6`](https://github.com/grafana/openapi-to-k6) and generates a different client.

## Installation

The extension requires k6 v2.3.0 or newer.
Download a k6 binary with the extension from the [releases](https://github.com/grafana/xk6-subcommand-openapi/releases), or build one with [xk6](https://github.com/grafana/xk6):

```sh
xk6 build --with github.com/grafana/xk6-subcommand-openapi@latest
```

## Quick start

Generate a runnable script from a descriptor and run it:

```sh
k6 x openapi api.yaml --example --out script.ts
k6 run script.ts
```

The script constructs the client and calls no operation yet.
Add calls to its default function, or generate only the client library and import it from your own script, as described in [Using the client](#using-the-client).
The [`examples`](examples/) directory contains a complete example against the QuickPizza demo application.

## Command

```sh
k6 x openapi <openapi-spec> [--example] [--out <file>]
```

`<openapi-spec>` is a local file path, relative to the current directory, or an `http` or `https` URL.
The descriptor can be JSON or YAML, in OpenAPI 3.0, 3.1, or 3.2.
Swagger 2.0 is not supported; convert it to OpenAPI 3 first.
External `$ref`s are resolved relative to the document that contains them.

| Flag | Description |
|------|-------------|
| `--example` | Append a k6 default function that constructs the client, so the output is a runnable script. |
| `-o`, `--out <file>` | Write the output to `<file>` instead of stdout. The file must have a `.ts` extension, and its directory must exist. |

With `--example`, the client's base URL is the descriptor's first server URL, with server variables set to their defaults, if that is an absolute `http` or `https` URL, and `http://localhost` otherwise.

Prefer `--out` over shell redirection: it overwrites the file only if generation succeeds, while redirection empties it even on failure.
When redirecting stdout, also use a `.ts` file, because k6 handles a file as TypeScript by its extension.

### Warnings

A descriptor that fails OpenAPI validation still generates a client, and each validation error is logged as a warning.
So is each descriptor construct that the client skips or approximates, such as a cookie parameter, or a schema renamed because its name clashes with one the client uses, such as `Response` becoming `Response2`.
Warnings go to k6's log on stderr and never into the `--out` file.

## Using the client

The client library exports a client class named after the descriptor's `info.title`, such as `PetStoreClient`, and types for the schemas, options, and results of the operations.
For a descriptor with the operations `listPets`, `createPet`, and `getPet`, where `getPet` documents a `200` response with a `Pet` and a `404` response with an `Error`, generate the client with `k6 x openapi api.yaml --out client.ts` and use it like this:

```ts
import { check } from "k6";
import { PetStoreClient } from "./client.ts";

const client = new PetStoreClient({
  baseUrl: "https://petstore.example.test/v1",
  commonK6Params: { headers: { Authorization: "Bearer my-token" } },
});

export default async function () {
  const created = await client.createPet({ body: { name: "Rex" } });
  check(created, { "pet created": (res) => res.status === 201 });

  const res = await client.getPet(1, { k6Params: { tags: { name: "getPet" } } });
  switch (res.status) {
    case 200:
      console.log(res.data.name); // res.data is a Pet
      break;
    case 404:
      console.log(res.data.message); // res.data is an Error
      break;
    case 0:
      console.log("no response"); // res.data is undefined
      break;
  }

  const [first, second] = await Promise.all([client.getPet(1), client.getPet(2)]);
  check(first, { "first pet found": (r) => r.status === 200 });
  check(second, { "second pet found": (r) => r.status === 200 });

  const pets = await client.listPets({ query: { limit: 10 } });
  check(pets, { "pets listed": (r) => r.status === 200 });
}
```

### Constructor

The constructor takes the required `baseUrl`, whose trailing slashes are removed, and optional `commonK6Params`, the k6 [`Params`](https://grafana.com/docs/k6/latest/javascript-api/k6-http/params/) for every request.

### Methods

There is one method per operation, named after its `operationId` in camelCase, or after the HTTP method and path, such as `getPetsPetId`.
Every method is `async` and sends its request with k6's non-blocking `http.asyncRequest`, so always `await` it; run calls concurrently with `Promise.all`.

A method takes the path parameters as positional arguments, in path order, followed by an `options` object:

- `query`: the query parameters.
  An array becomes one `name=value` pair per element, and an object one pair per property, so `{ filter: { color: "red", tags: ["a", "b"] } }` is sent as `color=red&tags=a&tags=b`.
  Objects and arrays nested inside an array or an object are omitted, with a warning at generation time.
- `headers`: the header parameters.
- `body`: the request body, encoded by its media type:
  - `application/json` and other JSON media types: with `JSON.stringify`.
  - `application/x-www-form-urlencoded`: by the same rules as `query`.
  - `multipart/form-data`: as a multipart body; file fields take `http.file(...)` or an `ArrayBuffer`.
  - any other media type: sent unchanged, as a `string` or an `ArrayBuffer`.
- `k6Params`: k6 `Params` for this request, merged over `commonK6Params`, with `headers`, `cookies`, and `tags` merged key by key.

A member is required only if the descriptor marks the parameter or request body as required, and `options` is optional if all its members are.

### Results

A method resolves to `{ status, data, response }`:

- `status`: the HTTP status code, or `0` if k6 received no response, such as on a timeout or a connection, DNS, or TLS failure.
- `data`: the body, parsed for a JSON `Content-Type` and unparsed otherwise, or `undefined` if it is empty.
- `response`: the k6 [`Response`](https://grafana.com/docs/k6/latest/javascript-api/k6-http/response/).

The promise rejects if a JSON body does not parse, or if a request fails while the k6 `throw` option is set.

The result type has one variant per documented response, plus one for status `0`.
Checking `status` with `if` or `switch` narrows `data` to the type documented for that status.
`expect(res.status).toBe(200)` does not narrow, because `expect` is not a TypeScript assertion function.
A range such as `4XX` has the status type `HttpStatus4XX`, and `default` has `HttpStatusAll`.
If other documented responses overlap them, the type is named after the method instead, such as `GetPetStatus4XX` or `GetPetStatusDefault`, and excludes the codes those responses cover.

The types describe documented responses only.
An undocumented response, such as a gateway error page, still arrives, with its actual `status` and `data`.
To check for one, compare `res.response.status`, a plain `number`, because editors flag `res.status === 503` as never true if the operation documents neither `503`, `5XX`, nor `default`.
For the same reason, the `default` branch of a `switch` over the documented codes has the type `never`.
A response with another `Content-Type` than documented gets `data` decoded as above, which may not match its type.

### Credentials

The client ignores the descriptor's security schemes and sends no credentials on its own.
Send them with `commonK6Params` for every request, or with `k6Params` for one:

- `basic` and `bearer` auth, and API keys in a header: set the header, such as `headers: { Authorization: "Bearer my-token" }`.
- API keys in a cookie: set `cookies`.
- `oauth2` and `openIdConnect`: obtain a token yourself and send it as a header.
- `digest` auth: put the percent-encoded credentials into `baseUrl`, as in `http://user:pass@host`, and set `auth: "digest"`; k6 supports only MD5.
- `mutualTLS`: use k6's [`tlsAuth`](https://grafana.com/docs/k6/latest/using-k6/k6-options/reference/#tls-auth) option.

API keys in the query and `http` schemes other than `basic`, `bearer`, and `digest` cannot be sent yet; the subcommand logs a warning for each such scheme that an operation uses.

## Limitations

- The client imports `URL`, `URLSearchParams`, and `FormData` from [jslib.k6.io](https://jslib.k6.io), so running it needs network access to jslib.k6.io.
- Security schemes do not generate credential options.
- An operation whose method name clashes with a member of the client class, such as `constructor`, makes the client library invalid.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to propose a change and for the development setup.

## License

AGPL-3.0. See [LICENSE.md](LICENSE.md).
