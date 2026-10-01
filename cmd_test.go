package openapi //nolint:testpackage // newSubcommand is unexported.

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/cmd/tests"
	"go.k6.io/k6/v2/lib/fsext"
)

const helloWorld = `console.log("Hello, World!");`

// execute runs the openapi subcommand in-process with args and returns what it wrote to stdout.
func execute(t *testing.T, ts *tests.GlobalTestState, args ...string) (string, error) {
	t.Helper()

	cmd := newSubcommand(ts.GlobalState)

	// The k6 root command silences usage and errors and prints the returned error itself.
	// Mirror that here, because in tests this command is the root.
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	var stdout, stderr bytes.Buffer

	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(t.Context())

	return stdout.String(), err
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

	stdout, err := execute(t, tests.NewGlobalTestState(t), "--help")
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

			stdout, err := execute(t, tests.NewGlobalTestState(t), args...)

			require.ErrorContains(t, err, "accepts 1 arg(s)")
			assert.Empty(t, stdout)
		})
	}
}

func TestDescriptorPathDoesNotExist(t *testing.T) {
	t.Parallel()

	_, err := execute(t, tests.NewGlobalTestState(t), "does-not-exist.yaml")
	require.NoError(t, err)
}

func TestDefaultOutput(t *testing.T) {
	t.Parallel()

	stdout, err := execute(t, tests.NewGlobalTestState(t), "api.yaml")
	require.NoError(t, err)

	assert.Empty(t, stdout)
}

func TestOutputWithExample(t *testing.T) {
	t.Parallel()

	stdout, err := execute(t, tests.NewGlobalTestState(t), "api.yaml", "--example")
	require.NoError(t, err)

	assert.Contains(t, stdout, "export default function")
	assert.Contains(t, stdout, helloWorld)
	assert.NotContains(t, stdout, "import ", "the example must import no module")
}

func TestWriteToFile(t *testing.T) {
	t.Parallel()

	want, err := execute(t, tests.NewGlobalTestState(t), "api.yaml", "--example")
	require.NoError(t, err)

	for _, flag := range []string{"--out", "-o"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)

			stdout, err := execute(t, ts, "api.yaml", "--example", flag, "script.ts")
			require.NoError(t, err)

			assert.Empty(t, stdout)
			assert.Equal(t, want, readFile(t, ts, "script.ts"), "script.ts must equal the stdout output")
		})
	}
}

func TestExistingFileIsOverwritten(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, cwdPath(ts, "script.ts"), []byte("other content"), 0o600))

	_, err := execute(t, ts, "api.yaml", "--out", "script.ts")
	require.NoError(t, err)

	assert.Empty(t, readFile(t, ts, "script.ts"))
}

func TestOutPathWithoutTSExtension(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	stdout, err := execute(t, ts, "api.yaml", "--out", "script.js")

	require.ErrorContains(t, err, "must have a .ts extension")
	assert.Empty(t, stdout)
	assert.ErrorIs(t, statErr(ts, "script.js"), fs.ErrNotExist, "script.js must not be created")
}

func TestParentDirectoryDoesNotExist(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	stdout, err := execute(t, ts, "api.yaml", "--out", "missing/script.ts")

	require.ErrorContains(t, err, "parent directory")
	assert.Empty(t, stdout)
	assert.ErrorIs(t, statErr(ts, "missing"), fs.ErrNotExist, "missing must not be created")
}

func TestOutPathIsDirectory(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	require.NoError(t, ts.FS.MkdirAll(cwdPath(ts, "out.ts"), 0o750))

	stdout, err := execute(t, ts, "api.yaml", "--out", "out.ts")

	require.ErrorContains(t, err, "is a directory")
	assert.Empty(t, stdout)
}
