// Package descriptor loads an OpenAPI descriptor from a local file or a remote URL,
// resolves its refs, checks its version, and validates it.
package descriptor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"go.k6.io/k6/v2/lib/fsext"
)

// Options configure Load.
type Options struct {
	// FS is the filesystem local descriptors and refs are read from.
	FS fsext.Fs
	// Cwd is the directory relative descriptor paths are resolved against.
	Cwd string
	// FetchTimeout bounds each HTTP request for a remote descriptor or ref.
	// It has no default here; the command sets it.
	FetchTimeout time.Duration
	// Warn receives each problem that does not prevent loading, as a single line.
	Warn func(string)
}

// Load reads the descriptor at location (a URL, or a path resolved against opts.Cwd),
// resolves its refs, checks its version, validates it, and reports validation
// errors through opts.Warn.
// Every returned error names location as given.
func Load(ctx context.Context, location string, opts Options) (*openapi3.T, error) {
	doc, err := load(ctx, location, opts)
	if err != nil {
		return nil, fmt.Errorf("loading descriptor %q: %w", location, err)
	}

	validate(ctx, doc, opts.Warn)

	return doc, nil
}

func load(ctx context.Context, location string, opts Options) (*openapi3.T, error) {
	uri, err := resolveLocation(location, opts.Cwd)
	if err != nil {
		return nil, err
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.Context = ctx
	loader.ReadFromURIFunc = openapi3.URIMapCache(openapi3.ReadFromURIs(
		openapi3.ReadFromHTTP(&http.Client{Timeout: opts.FetchTimeout}),
		readFromFS(opts.FS),
	))

	doc, err := loader.LoadFromURI(uri)
	if err != nil {
		return nil, err
	}

	if err := checkVersion(doc); err != nil {
		return nil, err
	}

	return doc, nil
}

// readFromFS returns a reader for local URLs, those without a host and with an empty or file scheme,
// that reads them from fs, so local reads stay on k6's filesystem abstraction.
// It returns openapi3.ErrURINotSupported for every other URL.
func readFromFS(fs fsext.Fs) openapi3.ReadFromURIFunc {
	return func(_ *openapi3.Loader, location *url.URL) ([]byte, error) {
		if location.Host != "" || (location.Scheme != "" && location.Scheme != "file") {
			return nil, openapi3.ErrURINotSupported
		}

		return fsext.ReadFile(fs, filepath.FromSlash(location.Path))
	}
}

// resolveLocation turns the user's argument into the URL the loader reads the root document from.
// An http or https argument is parsed as a URL.
// Any other argument is a file path, made absolute against cwd and converted to a URL path
// with forward slashes, so relative refs resolve against the descriptor's directory.
// A Windows path such as C:\test\api.yaml becomes C:/test/api.yaml, as in kin-openapi's own LoadFromFile.
func resolveLocation(location, cwd string) (*url.URL, error) {
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		return url.Parse(location)
	}

	path := location
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	return &url.URL{Path: filepath.ToSlash(path)}, nil
}

// checkVersion rejects documents that are not OpenAPI 3.x.
// kin-openapi loads Swagger 2.0 and other versions without error, so the loaded document is checked.
func checkVersion(doc *openapi3.T) error {
	if _, ok := doc.Extensions["swagger"]; ok {
		return errors.New("descriptors in Swagger 2.0 format are not supported; only OpenAPI 3.x is supported")
	}

	if doc.OpenAPI == "" {
		return errors.New("the descriptor has no openapi field; only OpenAPI 3.x is supported")
	}

	if major, _, _ := strings.Cut(doc.OpenAPI, "."); major != "3" {
		return fmt.Errorf("OpenAPI version %q is not supported; only OpenAPI 3.x is supported", doc.OpenAPI)
	}

	return nil
}

// validate reports each OpenAPI validation error of doc as one single-line warning.
func validate(ctx context.Context, doc *openapi3.T, warn func(string)) {
	err := doc.Validate(ctx, openapi3.EnableMultiError())
	if err == nil {
		return
	}

	errs := []error{err}
	if multi, ok := errors.AsType[openapi3.MultiError](err); ok {
		errs = multi
	}

	for _, err := range errs {
		warn("invalid descriptor: " + strings.Join(strings.Fields(err.Error()), " "))
	}
}
