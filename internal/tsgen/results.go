package tsgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen/model"
)

// result declares the Result type of op: the union of one variant per documented response,
// each with its data type, the k6 response, and its status type, followed by the variant for status 0,
// which k6 reports when it received no response. A responses key that is not a status code, a range,
// or default is skipped with a warning.
// The Result name is claimed first, then the names of the per-method status types,
// then the names of the response types, in discovery order.
func (o *operationMapper) result(op operation, typeName string) model.Named {
	return o.types.declare(typeName+"Result", "the result of "+op.String(), "", func(string) model.Type {
		responses := op.op.Responses.Map()
		documented := make([]responseKey, 0, len(responses))

		for _, key := range sortedKeys(responses) {
			k, ok := parseResponseKey(key)
			if !ok {
				o.warn(fmt.Sprintf("the response %q of %s has a key that is not a status code, a range, or default, "+
					"and is skipped", key, op))

				continue
			}

			documented = append(documented, k)
		}

		statuses := make([]model.Type, len(documented))
		for i, k := range documented {
			statuses[i] = o.statusType(op, typeName, k, documented)
		}

		variants := make(model.Union, 0, len(documented)+1)
		for i, k := range documented {
			variants = append(variants, variant(o.responseData(op, typeName, k.key, responses[k.key]), statuses[i]))
		}

		return append(variants, variant(model.Undefined, model.Literal{Value: float64(0)}))
	})
}

// responseKind is the kind of a documented responses key.
type responseKind int

const (
	statusCodeKey responseKind = iota + 1 // a status code from 100 to 599, such as 404
	rangeKey                              // a range from 1XX to 5XX, such as 4XX
	defaultKey                            // default
)

// responseKey is a documented responses key.
type responseKey struct {
	key   string
	kind  responseKind
	code  int // the status code, for a statusCodeKey
	class int // the first digit of the range, for a rangeKey
}

// parseResponseKey parses a responses key: three digits from 100 to 599, a range from 1XX to 5XX compared
// case-insensitively, or default. It reports false for any other key.
func parseResponseKey(key string) (responseKey, bool) {
	if key == "default" {
		return responseKey{key: key, kind: defaultKey}, true
	}

	if len(key) != 3 || key[0] < '1' || key[0] > '5' {
		return responseKey{}, false
	}

	if strings.EqualFold(key[1:], "XX") {
		return responseKey{key: key, kind: rangeKey, class: int(key[0] - '0')}, true
	}

	// The first character is a digit, so Atoi accepts no sign and succeeds only for three digits.
	code, err := strconv.Atoi(key)

	return responseKey{key: key, kind: statusCodeKey, code: code}, err == nil
}

// statusType returns the status type of the documented response k of op: the number literal of a status code,
// and for a range or default the shared status code type if the operation documents nothing that it excludes,
// and otherwise a per-method type without the explicit codes of the range, or, for default, without all
// other documented responses. documented are all documented responses of op in key order.
func (o *operationMapper) statusType(
	op operation, typeName string, k responseKey, documented []responseKey,
) model.Type {
	if k.kind == statusCodeKey {
		return model.Literal{Value: float64(k.code)}
	}

	from, suffix := o.statusTypes.all, "StatusDefault"
	if k.kind == rangeKey {
		from, suffix = o.statusTypes.ranges[k.class-1], "Status"+strconv.Itoa(k.class)+"XX"
	}

	// A range excludes the status codes in it, and default excludes all other documented responses.
	var excluded []model.Type

	for _, other := range documented {
		switch {
		case other.kind == statusCodeKey && (k.kind == defaultKey || other.code/100 == k.class):
			excluded = append(excluded, model.Literal{Value: float64(other.code)})
		case other.kind == rangeKey && k.kind == defaultKey:
			excluded = append(excluded, o.statusTypes.ranges[other.class-1])
		}
	}

	if len(excluded) == 0 {
		return from
	}

	what := fmt.Sprintf("the status type of the response %s of %s", k.key, op)

	return o.types.declare(typeName+suffix, what, "", func(string) model.Type {
		return model.Exclude{From: from, Excluded: union(excluded...)}
	})
}

// variant returns a variant of a Result type with the given data and status types.
func variant(data, status model.Type) model.Object {
	return model.Object{Properties: []model.Property{
		{Name: "data", Type: data},
		{Name: "response", Type: model.Response},
		{Name: "status", Type: status},
	}}
}

// responseData returns the data type of the response documented for status: undefined if it has no content,
// and otherwise the union over its media types of the schema type for JSON, unknown for JSON without
// a schema, and the k6 response body type for any other media type.
// Inline objects and compositions are named after the method and the status, such as GetPet200,
// or, in a response referenced from components, after the response, such as NotFoundResponse.
func (o *operationMapper) responseData(
	op operation, typeName, status string, ref *openapi3.ResponseRef,
) model.Type {
	if ref == nil || ref.Value == nil || len(ref.Value.Content) == 0 {
		return model.Undefined
	}

	content := ref.Value.Content
	types := make([]model.Type, 0, len(content))
	sharedName, location := sharedComponent(ref.Ref, ref.RefPath(), "Response")

	for _, mediaType := range sortedKeys(content) {
		media := content[mediaType]

		switch {
		case !isJSON(mediaType):
			types = append(types, model.ResponseBody)
		case media == nil || media.Schema == nil:
			types = append(types, model.Unknown)
		default:
			c := schemaCtx{hint: typeName + upperFirst(status), where: fmt.Sprintf("the response %s of %s", status, op)}
			if location != "" {
				// A response has one type per media type, so the media type is part of the key.
				c.hint, c.location = sharedName, location+"/content/"+mediaType
			}

			types = append(types, o.schemas.typeOf(media.Schema, c))
		}
	}

	return union(types...)
}

// isJSON reports whether mediaType is application/json or ends with +json, ignoring case and parameters.
func isJSON(mediaType string) bool {
	base := mediaBase(mediaType)

	return base == jsonMediaType || strings.HasSuffix(base, "+json")
}
