package openapi

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/lib/fsext"
)

//go:embed templates/example.ts
var exampleScript []byte

// outFilePerm is the permission of a newly created --out file; generated sources are not secret.
const outFilePerm fs.FileMode = 0o644

// options are the command's argument and flags.
type options struct {
	spec    string // path to the OpenAPI descriptor; not read in this version
	example bool
	out     string
}

// run generates the complete output, then writes it to stdout, or to the file at opts.out if it is set.
// Generating before writing ensures an error never leaves partial output.
func run(gs *state.GlobalState, stdout io.Writer, opts options) error {
	data := generate(opts.example)

	if opts.out == "" {
		_, err := stdout.Write(data)
		return err
	}

	path, err := resolveOutPath(gs, opts.out)
	if err != nil {
		return err
	}

	if err := fsext.WriteFile(gs.FS, path, data, outFilePerm); err != nil {
		return fmt.Errorf("writing output file %q: %w", opts.out, err)
	}

	return nil
}

// generate builds the complete output: the client library, followed by the example script if requested.
// The client library is empty in this version.
func generate(example bool) []byte {
	var out bytes.Buffer

	if example {
		out.Write(exampleScript)
	}

	return out.Bytes()
}

// resolveOutPath validates the --out path and returns it as an absolute path.
// It fails if the path does not end in .ts, is a directory, or its parent directory does not exist.
func resolveOutPath(gs *state.GlobalState, out string) (string, error) {
	if !strings.HasSuffix(out, ".ts") {
		return "", fmt.Errorf("output file %q must have a .ts extension", out)
	}

	path := out
	if !filepath.IsAbs(path) {
		cwd, err := gs.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolving output file %q: %w", out, err)
		}

		path = filepath.Join(cwd, path)
	}

	info, err := gs.FS.Stat(path)

	switch {
	case err == nil && info.IsDir():
		return "", fmt.Errorf("output path %q is a directory", out)
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("checking output file %q: %w", out, err)
	}

	parent, err := gs.FS.Stat(filepath.Dir(path))

	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("parent directory of output file %q does not exist", out)
	case err != nil:
		return "", fmt.Errorf("checking parent directory of output file %q: %w", out, err)
	case !parent.IsDir():
		return "", fmt.Errorf("parent of output file %q is not a directory", out)
	}

	return path, nil
}
