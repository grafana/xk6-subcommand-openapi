package tsgen

import (
	"fmt"

	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen/model"
)

// statusTypes references the shared status code types of the client library.
type statusTypes struct {
	ranges [5]model.Named // HttpStatus1XX to HttpStatus5XX, the 100 codes of each range
	all    model.Named    // HttpStatusAll, the union of the five ranges
}

// declareStatusTypes declares HttpStatus1XX to HttpStatus5XX and HttpStatusAll.
func declareStatusTypes(types *typeDeclarations) *statusTypes {
	s := &statusTypes{}
	all := make(model.Union, 0, len(s.ranges))

	for i := range s.ranges {
		first := (i + 1) * 100
		codes := fmt.Sprintf("status codes from %d to %d", first, first+99)

		s.ranges[i] = types.declare(fmt.Sprintf("HttpStatus%dXX", i+1), "the type of the "+codes, "The "+codes+".",
			func(string) model.Type {
				literals := make(model.Union, 0, 100)
				for code := first; code < first+100; code++ {
					literals = append(literals, model.Literal{Value: float64(code)})
				}

				return literals
			})
		all = append(all, s.ranges[i])
	}

	s.all = types.declare("HttpStatusAll", "the type of all status codes", "The status codes from 100 to 599.",
		func(string) model.Type { return all })

	return s
}
