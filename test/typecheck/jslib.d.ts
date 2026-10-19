// Declarations of the jslib.k6.io modules that generated client libraries import, which ship no types.
// They declare only the members the generated code uses.

declare module "https://jslib.k6.io/url/1.0.0/index.js" {
  export class URL {
    constructor(url: string);
    toString(): string;
  }

  export class URLSearchParams {
    append(name: string, value: string): void;
    toString(): string;
  }
}

declare module "https://jslib.k6.io/formdata/0.0.2/index.js" {
  export class FormData {
    boundary: string;
    append(name: string, value: unknown): void;
    body(): ArrayBuffer;
  }
}
