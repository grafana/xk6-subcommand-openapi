// Package openapi implements the "openapi" k6 subcommand,
// which generates a k6 client library from an OpenAPI descriptor.
package openapi

import (
	_ "embed"

	"github.com/spf13/cobra"
	"go.k6.io/k6/v2/cmd/state"
)

//go:embed help.md
var help string

// newSubcommand creates the "openapi" subcommand.
func newSubcommand(gs *state.GlobalState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openapi <openapi-spec>",
		Short: "Generate a k6 client library from an OpenAPI descriptor",
		Long:  help,
		Example: "  k6 x openapi api.yaml --out client.ts\n" +
			"  k6 x openapi api.yaml --example --out script.ts",
		Args: cobra.ExactArgs(1),
	}

	flags := cmd.Flags()
	example := flags.Bool("example", false, "append a hello-world k6 test script to the output")
	out := flags.StringP("out", "o", "",
		"write the output to this .ts file instead of stdout, overwriting an existing file")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return run(gs, cmd.OutOrStdout(), options{spec: args[0], example: *example, out: *out})
	}

	return cmd
}
