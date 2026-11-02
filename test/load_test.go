package test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/cmd/tests"

	"github.com/grafana/xk6-subcommand-openapi/internal/command"
)

const petsYAML = `openapi: 3.0.3
info: { title: Pets, version: 1.0.0 }
paths:
  /pets:
    get:
      responses:
        "200": { description: ok }
`

const petsJSON = `{
  "openapi": "3.0.3",
  "info": { "title": "Pets", "version": "1.0.0" },
  "paths": {
    "/pets": { "get": { "responses": { "200": { "description": "ok" } } } }
  }
}
`

// petsWithRefYAML references the Pet schema in schemas.yaml, relative to its own location.
const petsWithRefYAML = `openapi: 3.0.3
info: { title: Pets, version: 1.0.0 }
paths:
  /pets:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "schemas.yaml#/Pet" }
`

const schemasYAML = `Pet:
  type: object
  properties:
    name: { type: string }
`

// petsMethod is the start of the method generated for the operation in petsYAML and petsJSON.
const petsMethod = "async getPets("

// petType is the declaration generated for the Pet schema in schemasYAML.
const petType = "export type Pet = {\n  name?: string;\n};"

// serve starts an HTTP server that serves files by path and records the requested paths.
// Paths not in files get a 404.
func serve(t *testing.T, files map[string]string) (*httptest.Server, func() []string) {
	t.Helper()

	var (
		mu        sync.Mutex
		requested []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requested = append(requested, r.URL.Path)
		mu.Unlock()

		content, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}

		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(srv.Close)

	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()

		return append([]string(nil), requested...)
	}
}

// generate runs the command in ts with args, requires it to succeed, and returns its stdout and warnings.
func generate(t *testing.T, ts *tests.GlobalTestState, args ...string) (string, []string) {
	t.Helper()

	stdout, warnings, err := execute(t, ts, args...)
	require.NoError(t, err)

	return stdout, warnings
}

// generateFails runs the command in ts with args, requires it to fail without writing to stdout,
// and returns the error message.
func generateFails(t *testing.T, ts *tests.GlobalTestState, args ...string) string {
	t.Helper()

	stdout, _, err := execute(t, ts, args...)
	require.Error(t, err)
	assert.Empty(t, stdout)

	return err.Error()
}

func TestLocalDescriptor(t *testing.T) {
	t.Parallel()

	fromJSON, _ := generate(t, newTestState(t, map[string]string{"api.json": petsJSON}), "api.json")
	fromYAML, _ := generate(t, newTestState(t, map[string]string{apiYAML: petsYAML}), apiYAML)

	assert.Contains(t, fromYAML, petsMethod)
	assert.Equal(t, fromJSON, fromYAML, "JSON and YAML must generate the same output")
}

func TestRemoteDescriptor(t *testing.T) {
	t.Parallel()

	srv, _ := serve(t, map[string]string{"/api.yaml": petsYAML})

	remote, _ := generate(t, newTestState(t, nil), srv.URL+"/api.yaml")
	local, _ := generate(t, newTestState(t, map[string]string{apiYAML: petsYAML}), apiYAML)

	assert.Contains(t, local, petsMethod)
	assert.Equal(t, local, remote, "remote and local descriptors must generate the same output")
}

func TestExternalRefToLocalFile(t *testing.T) {
	t.Parallel()

	// Run from cwd, passing a path into a sibling directory, so the ref only resolves
	// relative to the descriptor's directory, not to the working directory.
	ts := newTestState(t, map[string]string{
		"specs/api.yaml":     petsWithRefYAML,
		"specs/schemas.yaml": schemasYAML,
	})
	ts.Cwd = cwdPath(ts, "work")
	require.NoError(t, ts.FS.MkdirAll(ts.Cwd, 0o750))

	stdout, _ := generate(t, ts, "../specs/api.yaml")

	assert.Contains(t, stdout, petType)
}

func TestExternalRefInRemoteDescriptor(t *testing.T) {
	t.Parallel()

	srv, requested := serve(t, map[string]string{
		"/specs/api.yaml":     petsWithRefYAML,
		"/specs/schemas.yaml": schemasYAML,
	})

	stdout, _ := generate(t, newTestState(t, nil), srv.URL+"/specs/api.yaml")

	assert.Contains(t, requested(), "/specs/schemas.yaml")
	assert.Contains(t, stdout, petType)
}

func TestExternalRefWithSiblingKeywords(t *testing.T) {
	t.Parallel()

	// In OpenAPI 3.1, keywords next to a $ref give each use its own copy of the schema;
	// both uses must still map to one Pet type, built from the first use.
	descriptor := `openapi: 3.1.0
info: { title: Pets, version: 1.0.0 }
paths:
  /pets:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  first: { $ref: "schemas.yaml#/Pet", description: The first pet. }
                  second: { $ref: "schemas.yaml#/Pet" }
`

	stdout, warnings := generate(t, newTestState(t, map[string]string{apiYAML: descriptor, "schemas.yaml": schemasYAML}),
		apiYAML)

	assert.Empty(t, warnings)
	assert.Contains(t, stdout, petType)
	assert.Contains(t, stdout, "first?: Pet;\n  second?: Pet;")
	assert.NotContains(t, stdout, "Pet2")
}

func TestExternalRefWithoutFragment(t *testing.T) {
	t.Parallel()

	// A $ref to a whole file names the schema after the file, without its extension.
	descriptor := strings.Replace(petsWithRefYAML, "schemas.yaml#/Pet", "pet.yaml", 1)
	petYAML := "type: object\nproperties:\n  name: { type: string }\n"

	stdout, warnings := generate(t, newTestState(t, map[string]string{apiYAML: descriptor, "pet.yaml": petYAML}),
		apiYAML)

	assert.Empty(t, warnings)
	assert.Contains(t, stdout, petType)
	assert.Contains(t, stdout, "data: Pet;")
}

func TestSameSchemaNameInDifferentDocuments(t *testing.T) {
	t.Parallel()

	// Two different schemas named Pet, one local and one in schemas.yaml, become Pet and Pet2,
	// in discovery order, with a warning.
	descriptor := `openapi: 3.0.3
info: { title: Pets, version: 1.0.0 }
paths:
  /pets:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  local: { $ref: "#/components/schemas/Pet" }
                  remote: { $ref: "schemas.yaml#/Pet" }
components:
  schemas:
    Pet: { type: string }
`

	stdout, warnings := generate(t, newTestState(t, map[string]string{apiYAML: descriptor, "schemas.yaml": schemasYAML}),
		apiYAML)

	assert.Equal(t, []string{
		`type name "Pet" is already used, so the schema schemas.yaml#/Pet is named "Pet2"`,
	}, warnings)
	assert.Contains(t, stdout, "export type Pet = string;")
	assert.Contains(t, stdout, "export type Pet2 = {\n  name?: string;\n};")
	assert.Contains(t, stdout, "local?: Pet;\n  remote?: Pet2;")
}

func TestRemoteDescriptorNotFound(t *testing.T) {
	t.Parallel()

	srv, _ := serve(t, nil)
	url := srv.URL + "/missing.yaml"

	msg := generateFails(t, newTestState(t, nil), url)

	assert.Contains(t, msg, url)
	assert.Contains(t, msg, "404")
}

func TestRemoteDescriptorTimesOut(t *testing.T) {
	t.Parallel()

	// The handler returns when the client gives up and closes the connection.
	// If the client does not give up, it responds after 3 seconds with a valid descriptor,
	// so that the command succeeds and the test fails instead of blocking.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
			_, _ = w.Write([]byte(petsYAML))
		}
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := executeWithConfig(t, newTestState(t, nil),
		command.Config{FetchTimeout: 100 * time.Millisecond}, srv.URL+"/slow.yaml")

	require.ErrorContains(t, err, srv.URL+"/slow.yaml")
	require.ErrorContains(t, err, "Client.Timeout exceeded")
	assert.Empty(t, stdout)
}

func TestDescriptorIsNotJSONOrYAML(t *testing.T) {
	t.Parallel()

	generateFails(t, newTestState(t, map[string]string{apiYAML: "{ this is: [ not valid"}), apiYAML)
}

func TestUnresolvableRef(t *testing.T) {
	t.Parallel()

	descriptor := strings.Replace(petsWithRefYAML, "schemas.yaml#/Pet", "#/components/schemas/Missing", 1) +
		"components:\n  schemas:\n    Pet: { type: string }\n"

	msg := generateFails(t, newTestState(t, map[string]string{apiYAML: descriptor}), apiYAML)

	// Only the missing component, not the full reference: a bug in kin-openapi v0.149.0 drops the reference
	// from the error. See the Risks section of the generate-client-from-descriptor design.
	assert.Contains(t, msg, `"Missing"`)
}

func TestLoadFailureLeavesOutFileUnchanged(t *testing.T) {
	t.Parallel()

	ts := newTestState(t, map[string]string{"client.ts": "some content"})

	_, _, err := execute(t, ts, "does-not-exist.yaml", "--out", "client.ts")

	require.Error(t, err)
	assert.Equal(t, "some content", readFile(t, ts, "client.ts"))
}

func TestOpenAPI31Descriptor(t *testing.T) {
	t.Parallel()

	descriptor := `openapi: 3.1.0
info: { title: Pets, version: 1.0.0 }
paths:
  /pets:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { type: [string, "null"] }
`

	stdout, _ := generate(t, newTestState(t, map[string]string{apiYAML: descriptor}), apiYAML)

	assert.Contains(t, stdout, "data: string | null;")
}

func TestSwagger20Descriptor(t *testing.T) {
	t.Parallel()

	descriptor := `swagger: "2.0"
info: { title: Pets, version: 1.0.0 }
paths:
  /pets:
    get:
      parameters:
        - $ref: "#/parameters/limit"
      responses:
        "200":
          description: ok
          schema: { $ref: "#/definitions/Pet" }
parameters:
  limit: { name: limit, in: query, type: integer }
definitions:
  Pet:
    type: object
    properties:
      name: { type: string }
`

	msg := generateFails(t, newTestState(t, map[string]string{apiYAML: descriptor}), apiYAML)

	assert.Contains(t, msg, "Swagger 2.0")
	assert.Contains(t, msg, "not supported")
}

func TestUnsupportedOpenAPIVersion(t *testing.T) {
	t.Parallel()

	descriptor := strings.Replace(petsYAML, "openapi: 3.0.3", `openapi: "4.0.0"`, 1)

	msg := generateFails(t, newTestState(t, map[string]string{apiYAML: descriptor}), apiYAML)

	assert.Contains(t, msg, "4.0.0")
	assert.Contains(t, msg, "not supported")
}

// invalidExampleYAML loads but fails validation: the example does not match the pattern.
const invalidExampleYAML = `openapi: 3.0.3
info: { title: Pets, version: 1.0.0 }
paths:
  /pets:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { type: string, pattern: "^[a-z]+$", example: "123" }
`

func TestDescriptorFailsValidation(t *testing.T) {
	t.Parallel()

	stdout, warnings := generate(t, newTestState(t, map[string]string{apiYAML: invalidExampleYAML}), apiYAML)

	assert.Contains(t, stdout, petsMethod)

	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "^[a-z]+$")
	assert.NotContains(t, warnings[0], "\n", "a warning must be a single line")
}

func TestWarningsDoNotReachOutFile(t *testing.T) {
	t.Parallel()

	files := map[string]string{apiYAML: invalidExampleYAML}
	want, _ := generate(t, newTestState(t, files), apiYAML)

	ts := newTestState(t, files)

	stdout, warnings, err := execute(t, ts, apiYAML, "--out", "client.ts")
	require.NoError(t, err)

	assert.Empty(t, stdout)
	assert.NotEmpty(t, warnings)
	assert.Equal(t, want, readFile(t, ts, "client.ts"))
}
