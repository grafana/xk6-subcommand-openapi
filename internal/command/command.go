// Package command implements the "openapi" k6 subcommand,
// which generates a k6 client library from an OpenAPI descriptor.
package command

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"go.k6.io/k6/v2/cmd/state"

	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/descriptor"
	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen"
)

//go:embed help.md
var help string

// exampleText is the Examples section of the help, in the style of k6's own commands.
const exampleText = `  # Generate a client library from a local descriptor and print it.
  k6 x openapi api.yaml

  # Generate a client library from a remote descriptor into client.ts.
  k6 x openapi https://example.test/api.yaml --out client.ts

  # Generate a runnable script that constructs the client, and run it.
  k6 x openapi api.yaml --example --out script.ts
  k6 run script.ts`

// Config holds the settings of the subcommand that are not exposed as flags.
type Config struct {
	// FetchTimeout bounds each HTTP request made to load a remote descriptor or its refs.
	FetchTimeout time.Duration
}

// DefaultConfig returns the production configuration.
// It is the only place that sets the production timeout of 30 seconds.
func DefaultConfig() Config {
	return Config{FetchTimeout: 30 * time.Second}
}

// NewSubcommand creates the "openapi" subcommand with DefaultConfig.
// It is the function registered with k6.
func NewSubcommand(gs *state.GlobalState) *cobra.Command {
	return NewSubcommandWithConfig(gs, DefaultConfig())
}

// NewSubcommandWithConfig creates the "openapi" subcommand with cfg.
// Tests use it, for example to set a short FetchTimeout.
func NewSubcommandWithConfig(gs *state.GlobalState, cfg Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "openapi <openapi-spec>",
		Short:   "Generate a k6 client library from an OpenAPI descriptor",
		Long:    help,
		Example: exampleText,
		Args:    cobra.ExactArgs(1),
	}

	flags := cmd.Flags()
	example := flags.Bool("example", false, "append a k6 test script that constructs the client")
	out := flags.StringP("out", "o", "",
		"write the output to `file` instead of stdout; it must end in .ts and is overwritten if it exists")

	cmd.RunE = func(_ *cobra.Command, args []string) error {
		return run(gs, cfg, options{spec: args[0], example: *example, out: *out})
	}

	return cmd
}

// options are the command's argument and flags.
type options struct {
	spec    string // location of the OpenAPI descriptor: a file path or an http or https URL
	example bool
	out     string
}

// run validates the --out path, loads the descriptor, generates the complete output,
// then writes it to gs.Stdout, or to the file at opts.out if it is set.
// Generating before writing ensures an error never leaves partial output.
// Warnings are logged through gs.Logger as they are found.
func run(gs *state.GlobalState, cfg Config, opts options) error {
	var outPath string

	if opts.out != "" {
		var err error
		if outPath, err = resolveOutPath(gs, opts.out); err != nil {
			return err
		}
	}

	cwd, err := gs.Getwd()
	if err != nil {
		return fmt.Errorf("resolving descriptor %q: %w", opts.spec, err)
	}

	warn := func(msg string) { gs.Logger.Warn(msg) }

	doc, err := descriptor.Load(gs.Ctx, opts.spec, descriptor.Options{
		FS:           gs.FS,
		Cwd:          cwd,
		FetchTimeout: cfg.FetchTimeout,
		Warn:         warn,
	})
	if err != nil {
		return err
	}

	output, err := tsgen.Generate(doc, tsgen.Options{Example: opts.example}, warn)
	if err != nil {
		return fmt.Errorf("generating the client library from %q: %w", opts.spec, err)
	}

	return writeOutput(gs, opts.out, outPath, output)
}
