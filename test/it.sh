#!/bin/sh
# Integration test: builds a real k6 binary with this extension and checks the openapi subcommand end to end.
# Run it through `make it`, which sets MODULE.
set -eu

: "${MODULE:?MODULE must be set to the extension's Go module path}"

cd "$(dirname "$0")/.."

dir=build/it
k6="$dir/k6"
fixtures=test/testdata

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
rm -rf "$dir"
mkdir -p "$dir"
xk6 build --with "$MODULE=." --output "$k6"

step "k6 x --help lists openapi"
out=$("$k6" x --help 2>&1) || fail "k6 x --help exited with $?" "$out"
printf '%s\n' "$out" | grep -qw openapi || fail "k6 x --help does not list openapi" "$out"

# Generate each fixture, including the corpus, with an example script, and run it,
# which proves that every generated client library loads in k6.
cases=$(find "$fixtures" -type f \( -name '*.yaml' -o -name '*.json' \) | sort)
[ -n "$cases" ] || fail "no fixtures found in $fixtures"

for fixture in $cases; do
	name=${fixture#"$fixtures"/}
	script="$dir/${name%.*}.ts"
	mkdir -p "$(dirname "$script")"

	step "k6 x openapi $fixture --example --out $script"
	out=$("$k6" x openapi "$fixture" --example --out "$script" 2>&1) ||
		fail "k6 x openapi $fixture --example --out $script exited with $?" "$out"
	[ -s "$script" ] || fail "$script was not written" "$out"

	step "k6 run $script"
	out=$("$k6" run --quiet --no-usage-report "$script" 2>&1) || fail "k6 run $script exited with $?" "$out"
	# k6 exits with 0 if the default function throws, and only logs the exception as an error.
	if printf '%s\n' "$out" | grep -q 'level=error'; then
		fail "k6 run $script logged an error" "$out"
	fi
done

step "PASS"
