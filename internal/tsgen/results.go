package tsgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/grafana-experimental/xk6-subcommand-openapi/internal/tsgen/model"
)

// result declares the Result type of op: the union of a success and a failure variant,
// whose data types are the unions of the data types of the documented responses.
// The Result name is claimed before the response types, in discovery order.
func (o *operationMapper) result(op operation, typeName string) model.Named {
	return o.types.declare(typeName+"Result", "the result of "+op.String(), "", func(string) model.Type {
		responses := op.op.Responses.Map()

		var success, failure []model.Type

		var defaultData model.Type

		for _, status := range sortedKeys(responses) {
			data := o.responseData(op, typeName, status, responses[status])

			switch {
			case isSuccess(status):
				success = append(success, data)
			case status == "default":
				defaultData = data

				fallthrough
			default:
				failure = append(failure, data)
			}
		}

		if len(success) == 0 && defaultData != nil {
			success = []model.Type{defaultData}
		}

		return model.Union{variant(true, success), variant(false, failure)}
	})
}

// variant returns a variant of a Result type with the union of data as its data type, or unknown if data is empty.
func variant(ok bool, data []model.Type) model.Object {
	typ := model.Type(model.Unknown)
	if len(data) > 0 {
		typ = union(data...)
	}

	return model.Object{Properties: []model.Property{
		{Name: "data", Type: typ},
		{Name: "ok", Type: model.Literal{Value: ok}},
		{Name: "response", Type: model.Response},
	}}
}

// isSuccess reports whether the responses key status is the 2XX range or a status code from 200 to 299.
func isSuccess(status string) bool {
	if strings.EqualFold(status, "2XX") {
		return true
	}

	code, err := strconv.Atoi(status)

	return err == nil && code >= 200 && code <= 299
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
