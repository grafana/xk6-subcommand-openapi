package tsgen

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// words splits s into words at every character that is not a letter or digit,
// at every transition from a lowercase letter or digit to an uppercase letter,
// and before the last uppercase letter of an uppercase run that is followed by a lowercase letter,
// so "get-pet_by id", "getPetById", and "HTTPError" give [get pet by id], [get Pet By Id], and [HTTP Error].
func words(s string) []string {
	var (
		result []string
		word   []rune
	)

	runes := []rune(s)
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			result = appendWord(result, word)
			word = nil

			continue
		}

		if i > 0 && unicode.IsUpper(r) && len(word) > 0 {
			prev := runes[i-1]
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])

			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextIsLower) {
				result = appendWord(result, word)
				word = nil
			}
		}

		word = append(word, r)
	}

	return appendWord(result, word)
}

func appendWord(result []string, word []rune) []string {
	if len(word) == 0 {
		return result
	}

	return append(result, string(word))
}

// pascalCase joins the words of s with the first letter of each word in uppercase,
// keeping the case of the other letters, so "pet store api" gives "PetStoreApi".
// It returns "" if s has no letters or digits, and prefixes a leading digit with "_".
func pascalCase(s string) string {
	var b strings.Builder

	for _, w := range words(s) {
		b.WriteString(upperFirst(w))
	}

	return identifier(b.String())
}

// camelCase is pascalCase with the first word in lowercase, so "get-pet-by-id" and "HTTPError"
// give "getPetById" and "httpError".
func camelCase(s string) string {
	ws := words(s)
	if len(ws) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString(strings.ToLower(ws[0]))

	for _, w := range ws[1:] {
		b.WriteString(upperFirst(w))
	}

	return identifier(b.String())
}

func upperFirst(w string) string {
	r := []rune(w)
	r[0] = unicode.ToUpper(r[0])

	return string(r)
}

// identifier prefixes name with "_" if it starts with a digit.
func identifier(name string) string {
	if name != "" && unicode.IsDigit([]rune(name)[0]) {
		return "_" + name
	}

	return name
}

// reservedArgNames are the names a path argument cannot have: the reserved words of strict-mode JavaScript,
// in which class bodies run, the names strict mode forbids for parameters, the method's own options argument,
// and the identifiers the generated method body uses: the http import and its local variables.
var reservedArgNames = map[string]bool{ //nolint:gochecknoglobals // constant lookup table
	"arguments": true, "await": true, "break": true, "case": true, "catch": true, "class": true,
	"const": true, "continue": true, "debugger": true, "default": true, "delete": true, "do": true,
	"else": true, "enum": true, "eval": true, "export": true, "extends": true, "false": true,
	"finally": true, "for": true, "function": true, "if": true, "implements": true, "import": true,
	"in": true, "instanceof": true, "interface": true, "let": true, "new": true, "null": true,
	"package": true, "private": true, "protected": true, "public": true, "return": true,
	"static": true, "super": true, "switch": true, "this": true, "throw": true, "true": true,
	"try": true, "typeof": true, "var": true, "void": true, "while": true, "with": true,
	"yield": true, "options": true,
	"http": true, "headers": true, "query": true, "url": true, "formData": true, "body": true, "response": true,
}

// argName returns the camelCase name of a path argument, with a "_" suffix if it is reserved.
func argName(s string) string {
	name := camelCase(s)
	if reservedArgNames[name] {
		return name + "_"
	}

	return name
}

// nameRegistry makes names unique within one namespace, such as the client methods or the exported types.
type nameRegistry struct {
	kind string // the kind of name, for warnings, such as "method" or "type"
	used map[string]bool
	warn func(string)
}

func newNameRegistry(kind string, warn func(string)) *nameRegistry {
	return &nameRegistry{kind: kind, used: make(map[string]bool), warn: warn}
}

// claim reserves name and returns it, or, if it is already taken, reserves and returns name followed by
// the lowest free numeric suffix from 2 on, and reports a warning that names what the name is for.
func (r *nameRegistry) claim(name, what string) string {
	if !r.used[name] {
		r.used[name] = true

		return name
	}

	for i := 2; ; i++ {
		candidate := name + strconv.Itoa(i)
		if r.used[candidate] {
			continue
		}

		r.used[candidate] = true
		r.warn(fmt.Sprintf("%s name %q is already used, so %s is named %q", r.kind, name, what, candidate))

		return candidate
	}
}
