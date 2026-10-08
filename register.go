// Package openapi registers the "openapi" k6 subcommand,
// which generates a k6 client library from an OpenAPI descriptor.
package openapi

import (
	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/command"
	"go.k6.io/k6/v2/subcommand"
)

func init() {
	subcommand.RegisterExtension("openapi", command.NewSubcommand)
}
