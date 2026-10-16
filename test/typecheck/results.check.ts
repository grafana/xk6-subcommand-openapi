// Asserts that the result types of results_client.ts narrow as the Method result requirement specifies.
// Each assertion only compiles if the narrowed type is exactly the expected one, so `make typecheck` fails
// if narrowing breaks. `@ts-expect-error` marks code that must not compile.

import type { Error, Job, NotFound, Pet, Problem, ResultsClient } from "../testdata/results_client.ts";

/** Equal is true if A and B are the same type, and false otherwise. */
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;

/** assertType compiles only if the type argument is true, as in assertType<Equal<typeof x, Pet>>(). */
function assertType<T extends true>(): void {}

declare const client: ResultsClient;

// Scenario: Narrowing by status.
export async function narrowingByStatus(): Promise<void> {
  const res = await client.getPet(1);

  if (res.status === 200) {
    assertType<Equal<typeof res.data, Pet>>();
  }

  if (res.status === 404) {
    assertType<Equal<typeof res.data, NotFound>>();
  }

  if (res.status === 201 || res.status === 503) {
    assertType<Equal<typeof res.data, Error>>();
  }

  if (res.status === 0) {
    assertType<Equal<typeof res.data, undefined>>();
  }

  switch (res.status) {
    case 200:
      assertType<Equal<typeof res.data, Pet>>();
      break;
    case 404:
      assertType<Equal<typeof res.data, NotFound>>();
      break;
    case 0:
      assertType<Equal<typeof res.data, undefined>>();
      break;
    default:
      assertType<Equal<typeof res.data, Error>>();
  }
}

// Scenarios: Narrowing by success, Several success responses, and Range with explicit codes.
export async function narrowingBySuccessAndRange(): Promise<void> {
  const res = await client.updatePet(1);

  if (res.status === 200 || res.status === 202) {
    assertType<Equal<typeof res.data, Pet | Job>>();
  }

  if (res.status === 202) {
    assertType<Equal<typeof res.data, Job>>();
  }

  if (res.status === 418) {
    assertType<Equal<typeof res.data, Problem>>();
  }

  switch (res.status) {
    case 200:
    case 202:
      assertType<Equal<typeof res.data, Pet | Job>>();
      break;
    case 204:
    case 0:
      assertType<Equal<typeof res.data, undefined>>();
      break;
    case 404:
      assertType<Equal<typeof res.data, NotFound>>();
      break;
    default:
      assertType<Equal<typeof res.data, Problem>>();
  }

  switch (res.status) {
    case 202:
      assertType<Equal<typeof res.data, Job>>();
      break;
    case 418:
      assertType<Equal<typeof res.data, Problem>>();
      break;
  }
}

// Scenario: Range without explicit codes, next to a default that excludes the range.
export async function rangeWithoutExplicitCodes(): Promise<void> {
  const res = await client.getJob();

  if (res.status === 503) {
    assertType<Equal<typeof res.data, Problem>>();
  }

  if (res.status === 201) {
    assertType<Equal<typeof res.data, Error>>();
  }
}

// Undocumented status codes are not part of the result type, but response.status can be compared with them.
export async function undocumentedStatus(): Promise<void> {
  const res = await client.updatePet(1);

  // @ts-expect-error updatePet documents neither 503, nor 5XX, nor default.
  if (res.status === 503) {
    return;
  }

  if (res.response.status === 503) {
    return;
  }
}
