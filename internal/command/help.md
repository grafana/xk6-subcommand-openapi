Generate a typed k6 client library from an OpenAPI descriptor.

The descriptor is a local file or an http or https URL, in JSON or YAML, and must be OpenAPI 3.x; Swagger 2.0 is not supported.
The client library is a TypeScript module with one async method per operation.
Validation errors and unsupported descriptor constructs are logged as warnings.
See https://github.com/grafana/xk6-subcommand-openapi for the client's API and its limitations.
