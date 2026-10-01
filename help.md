Generate a k6 client library from an OpenAPI descriptor.

The <openapi-spec> argument is the path to the OpenAPI descriptor.
This version does not read the descriptor yet and generates an empty client library.

With --example, a hello-world k6 test script is appended to the client library, so the output is a runnable k6 script.

The output is written to stdout unless --out is given.
With --out, the output is written to that file instead, and an existing file is overwritten.
The --out file must have a .ts extension, and its parent directory must exist.

When redirecting stdout to a file, use a .ts file, because k6 selects TypeScript handling by file extension.
Prefer --out when writing files: shell redirection empties the target file even if the command fails, while --out writes nothing on error.
