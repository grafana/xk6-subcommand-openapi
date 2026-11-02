package test

import (
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/cmd/tests"
	"go.k6.io/k6/v2/lib/fsext"

	"github.com/grafana/xk6-subcommand-openapi/internal/command"
)

// apiYAML is the name of the descriptor file most tests generate from.
const apiYAML = "api.yaml"

// minimalDescriptor is the smallest valid OpenAPI descriptor, for tests that do not care about its content.
const minimalDescriptor = `openapi: 3.0.3
info: { title: Minimal, version: 1.0.0 }
paths: {}
`

// minimalFiles returns files for newTestState that put minimalDescriptor at apiYAML.
func minimalFiles() map[string]string {
	return map[string]string{apiYAML: minimalDescriptor}
}

// execute runs the openapi subcommand in-process with args and the default configuration.
// It returns what the command wrote to stdout, the messages of the warnings it logged, and its error.
func execute(t *testing.T, ts *tests.GlobalTestState, args ...string) (string, []string, error) {
	t.Helper()

	return executeWithConfig(t, ts, command.DefaultConfig(), args...)
}

// executeWithConfig is execute with cfg instead of the default configuration.
func executeWithConfig(
	t *testing.T, ts *tests.GlobalTestState, cfg command.Config, args ...string,
) (string, []string, error) {
	t.Helper()

	cmd := command.NewSubcommandWithConfig(ts.GlobalState, cfg)

	// Mirror the k6 root command, because in tests this command is the root:
	// it writes to the global state's writers, silences usage and errors, and prints the returned error itself.
	cmd.SetOut(ts.Stdout)
	cmd.SetErr(ts.Stderr)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(args)

	err := cmd.Execute()

	var warnings []string

	for _, entry := range ts.LoggerHook.Drain() {
		if entry.Level == logrus.WarnLevel {
			warnings = append(warnings, entry.Message)
		}
	}

	return ts.Stdout.String(), warnings, err
}

// writeFile writes content to name, relative to the working directory, creating parent directories.
func writeFile(t *testing.T, ts *tests.GlobalTestState, name, content string) {
	t.Helper()

	path := cwdPath(ts, name)
	require.NoError(t, ts.FS.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, fsext.WriteFile(ts.FS, path, []byte(content), 0o600))
}

// newTestState returns a k6 test state whose working directory contains files, keyed by relative path.
func newTestState(t *testing.T, files map[string]string) *tests.GlobalTestState {
	t.Helper()

	ts := tests.NewGlobalTestState(t)
	for name, content := range files {
		writeFile(t, ts, name, content)
	}

	return ts
}

func cwdPath(ts *tests.GlobalTestState, name string) string {
	return filepath.Join(ts.Cwd, name)
}

func readFile(t *testing.T, ts *tests.GlobalTestState, name string) string {
	t.Helper()

	data, err := fsext.ReadFile(ts.FS, cwdPath(ts, name))
	require.NoError(t, err)

	return string(data)
}

func statErr(ts *tests.GlobalTestState, name string) error {
	_, err := ts.FS.Stat(cwdPath(ts, name))

	return err
}

func TestHelp(t *testing.T) {
	t.Parallel()

	stdout, _, err := execute(t, newTestState(t, nil), "--help")
	require.NoError(t, err)

	assert.Contains(t, stdout, "openapi <openapi-spec>")
	assert.Contains(t, stdout, "--example")
	assert.Contains(t, stdout, "-o, --out")
}

func TestArguments(t *testing.T) {
	t.Parallel()

	for name, args := range map[string][]string{
		"missing":  {},
		"too many": {"a.yaml", "b.yaml"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stdout, _, err := execute(t, newTestState(t, nil), args...)

			require.ErrorContains(t, err, "accepts 1 arg(s)")
			assert.Empty(t, stdout)
		})
	}
}

func TestDescriptorPathDoesNotExist(t *testing.T) {
	t.Parallel()

	stdout, _, err := execute(t, newTestState(t, nil), "does-not-exist.yaml")

	require.ErrorContains(t, err, "does-not-exist.yaml")
	assert.Empty(t, stdout)
}

func TestDefaultOutput(t *testing.T) {
	t.Parallel()

	stdout, _, err := execute(t, newTestState(t, minimalFiles()), apiYAML)
	require.NoError(t, err)

	assert.Contains(t, stdout, "export class MinimalClient {")
	assert.NotContains(t, stdout, "export default")
}

func TestOutputWithExample(t *testing.T) {
	t.Parallel()

	client, _, err := execute(t, newTestState(t, minimalFiles()), apiYAML)
	require.NoError(t, err)

	stdout, _, err := execute(t, newTestState(t, minimalFiles()), apiYAML, "--example")
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(stdout, client), "the output must start with the client library")
	assert.Equal(t, `
export default async function (): Promise<void> {
  const client = new MinimalClient({ baseUrl: "http://localhost" });
  // Call the client's methods with await, for example: const result = await client.<method>(...);
}
`, strings.TrimPrefix(stdout, client))
}

func TestExampleBaseURL(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		servers string
		want    string
	}{
		"first server with variables": {
			servers: `
  - url: "https://{region}.example.test/{version}"
    variables:
      region: { default: eu, enum: [eu, us] }
      version: { default: v1 }
  - url: https://second.example.test`,
			want: "https://eu.example.test/v1",
		},
		"no servers":       {servers: " []", want: "http://localhost"},
		"relative URL":     {servers: "\n  - url: /v1", want: "http://localhost"},
		"non-http scheme":  {servers: "\n  - url: ftp://files.example.test", want: "http://localhost"},
		"upper-case HTTPS": {servers: "\n  - url: HTTPS://api.example.test", want: "HTTPS://api.example.test"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			descriptor := minimalDescriptor + "servers:" + tc.servers + "\n"

			stdout, _ := generate(t, newTestState(t, map[string]string{apiYAML: descriptor}), apiYAML, "--example")

			assert.Contains(t, stdout, "new MinimalClient({ baseUrl: "+strconv.Quote(tc.want)+" });")
		})
	}
}

func TestWriteToFile(t *testing.T) {
	t.Parallel()

	want, _, err := execute(t, newTestState(t, minimalFiles()), apiYAML, "--example")
	require.NoError(t, err)

	for _, flag := range []string{"--out", "-o"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()

			ts := newTestState(t, minimalFiles())

			stdout, _, err := execute(t, ts, apiYAML, "--example", flag, "script.ts")
			require.NoError(t, err)

			assert.Empty(t, stdout)
			assert.Equal(t, want, readFile(t, ts, "script.ts"), "script.ts must equal the stdout output")
		})
	}
}

func TestExistingFileIsOverwritten(t *testing.T) {
	t.Parallel()

	want, _, err := execute(t, newTestState(t, minimalFiles()), apiYAML)
	require.NoError(t, err)

	ts := newTestState(t, minimalFiles())
	writeFile(t, ts, "script.ts", "other content")

	_, _, err = execute(t, ts, apiYAML, "--out", "script.ts")
	require.NoError(t, err)

	assert.Equal(t, want, readFile(t, ts, "script.ts"), "script.ts must contain only the stdout output")
}

func TestOutPathWithoutTSExtension(t *testing.T) {
	t.Parallel()

	ts := newTestState(t, minimalFiles())

	stdout, _, err := execute(t, ts, apiYAML, "--out", "script.js")

	require.ErrorContains(t, err, "must have a .ts extension")
	assert.Empty(t, stdout)
	assert.ErrorIs(t, statErr(ts, "script.js"), fs.ErrNotExist, "script.js must not be created")
}

func TestParentDirectoryDoesNotExist(t *testing.T) {
	t.Parallel()

	ts := newTestState(t, minimalFiles())

	stdout, _, err := execute(t, ts, apiYAML, "--out", "missing/script.ts")

	require.ErrorContains(t, err, "parent directory")
	assert.Empty(t, stdout)
	assert.ErrorIs(t, statErr(ts, "missing"), fs.ErrNotExist, "missing must not be created")
}

func TestOutPathIsDirectory(t *testing.T) {
	t.Parallel()

	ts := newTestState(t, minimalFiles())
	require.NoError(t, ts.FS.MkdirAll(cwdPath(ts, "out.ts"), 0o750))

	stdout, _, err := execute(t, ts, apiYAML, "--out", "out.ts")

	require.ErrorContains(t, err, "is a directory")
	assert.Empty(t, stdout)
}
