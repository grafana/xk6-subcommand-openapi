package command

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/lib/fsext"
)

// outFilePerm is the permission of a newly created --out file; generated sources are not secret.
const outFilePerm fs.FileMode = 0o644

// writeOutput writes data to gs.Stdout if out is empty, and otherwise to path,
// the --out file out as resolved by resolveOutPath.
func writeOutput(gs *state.GlobalState, out, path string, data []byte) error {
	if out == "" {
		_, err := gs.Stdout.Write(data)
		return err
	}

	if err := fsext.WriteFile(gs.FS, path, data, outFilePerm); err != nil {
		return fmt.Errorf("writing output file %q: %w", out, err)
	}

	return nil
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
