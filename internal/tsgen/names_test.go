package tsgen //nolint:testpackage // the naming helpers are unexported.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// These tests cover the edge cases of the naming rules.
// The golden tests cover the naming scenarios of the spec through the command.

func TestPascalCase(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"HTTPError":      "HTTPError",
		"v2Pets":         "V2Pets",
		"2fa-codes":      "_2faCodes",
		"__pet__store__": "PetStore",
		"café menu":      "CaféMenu",
		"!!!":            "",
	} {
		assert.Equal(t, want, pascalCase(in), "pascalCase(%q)", in)
	}
}

func TestCamelCase(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"HTTPError":      "httpError",
		"XMLHttpRequest": "xmlHttpRequest",
		"PETS":           "pets",
		"_internal":      "internal",
		"2fa":            "_2fa",
		"":               "",
	} {
		assert.Equal(t, want, camelCase(in), "camelCase(%q)", in)
	}
}

func TestArgName(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"pet-id":  "petId",
		"class":   "class_",
		"Default": "default_",
		"options": "options_",
		"eval":    "eval_",
		"http":    "http_",
		"url":     "url_",
	} {
		assert.Equal(t, want, argName(in), "argName(%q)", in)
	}
}
