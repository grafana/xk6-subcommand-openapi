package test

import (
	"flag"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/lib/fsext"
)

// update makes TestGolden rewrite the golden files instead of comparing with them:
//
//	go test ./test -run TestGolden -update
var update = flag.Bool("update", false, "rewrite the golden files in testdata instead of comparing with them") //nolint:gochecknoglobals // test flag

// goldenDir holds the fixtures <case>.yaml and their golden files <case>_client.ts and <case>_warnings.txt,
// and, in its subdirectory corpus, real-world fixtures in YAML and JSON with their golden files.
const goldenDir = "testdata"

// generations is how often each fixture is generated to check that the output is deterministic.
// Go randomizes map iteration order, so an ordering bug shows up as a difference between runs.
const generations = 10

// TestGolden generates the client library from each fixture in goldenDir and its subdirectories
// and compares the output with <case>_client.ts and the logged warnings, one per line, with <case>_warnings.txt.
func TestGolden(t *testing.T) {
	t.Parallel()

	disk := fsext.NewOsFs()

	fixtures := goldenFixtures(t, disk)
	require.NotEmpty(t, fixtures, "no fixtures found in %s", goldenDir)

	for _, fixture := range fixtures {
		name := strings.TrimSuffix(fixture, filepath.Ext(fixture))

		t.Run(filepath.ToSlash(name), func(t *testing.T) {
			t.Parallel()

			file := filepath.Base(fixture)
			descriptor := string(readDisk(t, disk, fixture))
			client, warnings := generateCase(t, file, descriptor)

			compareGolden(t, disk, name+"_client.ts", client)
			compareGolden(t, disk, name+"_warnings.txt", warnings)

			for range generations - 1 {
				again, againWarnings := generateCase(t, file, descriptor)
				require.Equal(t, client, again, "repeated generation must produce identical output")
				require.Equal(t, warnings, againWarnings, "repeated generation must report identical warnings")
			}
		})
	}
}

// goldenFixtures returns the paths, relative to goldenDir, of the .yaml and .json fixtures in goldenDir
// and its subdirectories.
func goldenFixtures(t *testing.T, disk fsext.Fs) []string {
	t.Helper()

	var fixtures []string

	err := fsext.Walk(disk, goldenDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		if ext := filepath.Ext(path); ext == ".yaml" || ext == ".json" {
			rel, err := filepath.Rel(goldenDir, path)
			if err != nil {
				return err
			}

			fixtures = append(fixtures, rel)
		}

		return nil
	})
	require.NoError(t, err)

	return fixtures
}

// generateCase runs the command on descriptor, stored in the working directory as file,
// and returns the output and the warnings, one per line.
func generateCase(t *testing.T, file, descriptor string) (string, string) {
	t.Helper()

	stdout, warnings := generate(t, newTestState(t, map[string]string{file: descriptor}), file)
	if len(warnings) == 0 {
		return stdout, ""
	}

	return stdout, strings.Join(warnings, "\n") + "\n"
}

// compareGolden compares got with the golden file name in goldenDir, or rewrites the file with -update.
func compareGolden(t *testing.T, disk fsext.Fs, name, got string) {
	t.Helper()

	path := filepath.Join(goldenDir, name)

	if *update {
		require.NoError(t, fsext.WriteFile(disk, path, []byte(got), 0o600))
		return
	}

	assert.Equal(t, string(readDisk(t, disk, name)), got,
		"%s differs; if the change is intended, run go test ./test -run TestGolden -update and review git diff", path)
}

func readDisk(t *testing.T, disk fsext.Fs, name string) []byte {
	t.Helper()

	data, err := fsext.ReadFile(disk, filepath.Join(goldenDir, name))
	require.NoError(t, err, "if a golden file is missing, run go test ./test -run TestGolden -update")

	return data
}
