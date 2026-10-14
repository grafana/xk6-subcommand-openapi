# Tasks

## 1. Fixture

- [x] 1.1 Add the fixture `test/testdata/serialization.yaml` with the operations of the `Array query parameter`, `Object query parameter`, `Nested query parameter value`, `Array of objects in the query`, `URL-encoded request body`, `Object field of a URL-encoded request body`, and `Nested field of a URL-encoded request body` scenarios, generate its golden files with `go test ./test -run TestGolden -update`, and verify `make test` passes and `serialization_client.ts` still contains the old `String(item)` helper, which sends objects as `[object Object]`

## 2. Serialize object values

- [x] 2.1 Replace `_encodeQuery` in `internal/tsgen/render/templates/client.ts.tmpl` with the helper from design decision 1 and update its doc comment, regenerate the golden files with `go test ./test -run TestGolden -update`, review the diffs by hand so that only the helper changes and the helper matches each request scenario of the `Request building` requirement, and verify `make test` and `make it` pass

## 3. Warn about nested values

- [x] 3.1 Keep the declarations of `typeDeclarations` in a map keyed by name, and add the nested-value check from design decision 3 to `internal/tsgen`, run in `paramsMember` and `requestBody` right after the schema is mapped, warning for query parameters and for the fields of URL-encoded bodies that nested values are not supported and are omitted, and verify with the golden warnings of `serialization.yaml` that `filter` in the nested case, `owners`, and `address` in the nested body case get a warning and the non-nested `filter` and `address` cases do not
- [x] 3.2 Extend `serialization.yaml` with the edge cases of design decision 3: a recursive schema, a body with `allOf`, a nullable object property, a property typed `unknown`, and a `$ref` to a component schema, regenerate and review the golden files, and verify `make test` passes and each warning in `serialization_warnings.txt` is expected
- [x] 3.3 Review the golden warning files of the other fixtures and the corpus for new nested-value warnings, confirm each is correct against the `Request building` requirement, and verify `make test` passes

## 4. Documentation and spec

- [ ] 4.1 Update the `query` and `body` items of the method section in `README.md` to describe the flattening of objects and that nested values are not supported, are omitted, and give a warning, and verify the described serialization matches the `Request building` scenarios
- [ ] 4.2 Remove the object query parameter item from the `Open problems` paragraph in the Purpose of `openspec/specs/generated-client/spec.md`, which the archive step does not sync, and verify `openspec validate serialize-object-query-parameters --strict` passes

## 5. Final checks

- [ ] 5.1 Verify `make test`, `make it`, and `make lint` pass without new `.golangci.patch` entries, and `git status` shows no untracked build output
