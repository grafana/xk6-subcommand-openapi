# Tasks

## 1. Fixture

- [ ] 1.1 Extend `test/testdata/results.yaml` so it covers every scenario of the `Method result` requirement: add a `404` response to `updatePet` for `Narrowing by success` and `Range with explicit codes`, an operation with `200` and `5XX` for `Range without explicit codes`, an operation with `200` and the invalid key `20X` for `Invalid response key`, and an operation whose response uses a component schema named `HttpStatus` for `Status code type name in use`, keeping `createPet` with only `201` for `No documented failure`; update the fixture's header comment, regenerate its golden files with `go test ./test -run TestGolden -update`, and verify `make test` passes and `results_client.ts` still has the two-variant Result types with `ok`

## 2. Status code types and per-response variants

- [ ] 2.1 Add the `Exclude` node to `internal/tsgen/model` and print it as `Exclude<From, Excluded>` in `internal/tsgen/render/types.go`, as in design decision 8, and verify `make test` passes with unchanged golden files
- [ ] 2.2 Declare `Status1XX` to `Status5XX` and `HttpStatus` in `internal/tsgen` for client libraries with at least one operation, claimed after all operations are mapped as in design decision 7, regenerate the golden files, and verify that `results_client.ts` exports them, that the `HttpStatus` schema keeps its name while the status code type is `HttpStatus2` with a warning in `results_warnings.txt`, and that `no_operations_client.ts` does not change
- [ ] 2.3 Rewrite `result` in `internal/tsgen/results.go` to build one variant per documented response with `data`, `response`, and `status` and without `ok`, the per-method `Status<N>XX` and `StatusDefault` types of design decision 3, the `default` rule of design decision 4, without variants for undocumented codes, the status 0 variant, and the warning for invalid keys of design decision 6; change the returned object in `internal/tsgen/render/templates/client.ts.tmpl` to `{ status: response.status, data, response }`; regenerate the golden files, review `results_client.ts` and `results_warnings.txt` by hand against each scenario of the `Method result` requirement, and verify `make test` and `make it` pass
- [ ] 2.4 Review the golden diffs of the other fixtures and the corpus for result types that contradict the `Method result` requirement, such as a leftover `ok`, a variant for an undocumented code, a missing status 0 variant, or a catch-all typed `number`, and verify `make test` passes
- [ ] 2.5 Check by hand, with a TypeScript compiler outside the repository, that `results_client.ts` narrows as the scenarios `Narrowing by status`, `Narrowing by success`, `Several success responses`, and `Range with explicit codes` say, that comparing `status` with an undocumented code is reported as TS2367 while `response.status` is not, and that hovers and errors show the per-method status type names, as in design decision 9; record the compiler version and the outcome in the PR description

## 3. Documentation

- [ ] 3.1 Update the `Result` section and the example in `README.md`: replace `ok` with `status`, show narrowing with `if` and `switch` on `status` and exact status checks in `check`, describe the status code types and the per-method status types, that undocumented codes are not in the type, so a comparison with one is flagged by editors and `res.response.status` can be compared instead, the status 0 variant for requests without a response, and that `expect(...).toBe(...)` does not narrow; remove the sentence about a missing `await` giving `ok` as `undefined`; verify every type name and field in the README matches `results_client.ts`

## 4. Final checks

- [ ] 4.1 Verify `openspec validate discriminate-results-by-status --strict`, `make test`, `make it`, and `make lint` pass without new `.golangci.patch` entries, and `git status` shows no untracked build output
