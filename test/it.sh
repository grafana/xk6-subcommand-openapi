#!/bin/sh
# Integration test: builds a real k6 binary with this extension and checks the openapi subcommand end to end.
# Run it through `make it`, which sets MODULE.
set -eu

: "${MODULE:?MODULE must be set to the extension's Go module path}"

cd "$(dirname "$0")/.."

dir=build/it
k6="$dir/k6"
script="$dir/script.ts"
want='Hello, World!'

# Do not send k6 usage reports from test runs.
export K6_NO_USAGE_REPORT=true

fail() {
	printf 'FAIL: %s\n' "$1" >&2
	if [ $# -gt 1 ]; then
		printf -- '--- output ---\n%s\n--------------\n' "$2" >&2
	fi
	exit 1
}

step() {
	printf '==> %s\n' "$1"
}

step "build $k6 with $MODULE"
mkdir -p "$dir"
rm -f "$k6" "$script"
xk6 build --with "$MODULE=." --output "$k6"

step "k6 x --help lists openapi"
out=$("$k6" x --help 2>&1) || fail "k6 x --help exited with $?" "$out"
printf '%s\n' "$out" | grep -qw openapi || fail "k6 x --help does not list openapi" "$out"

step "k6 x openapi unused.yaml prints nothing"
out=$("$k6" x openapi unused.yaml) || fail "k6 x openapi unused.yaml exited with $?" "$out"
[ -z "$out" ] || fail "k6 x openapi unused.yaml wrote to stdout" "$out"

step "k6 x openapi unused.yaml --example --out $script"
out=$("$k6" x openapi unused.yaml --example --out "$script" 2>&1) ||
	fail "k6 x openapi unused.yaml --example --out $script exited with $?" "$out"
[ -f "$script" ] || fail "$script was not written" "$out"

step "k6 run $script logs $want"
out=$("$k6" run "$script" 2>&1) || fail "k6 run $script exited with $?" "$out"
printf '%s\n' "$out" | grep -qF "$want" || fail "k6 run $script did not log $want" "$out"

step "PASS"
