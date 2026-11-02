SHELL       := bash
.SHELLFLAGS := -e -o pipefail -c

MODULE := github.com/grafana/xk6-subcommand-openapi

# The grafana/k6-ci ref is read from the CI workflow, so CI and local lint use the same config.
WORKFLOW  ?= .github/workflows/all.yml
K6_CI_REF := $(shell grep -oE 'grafana/k6-ci/[^@[:space:]]+@[A-Za-z0-9._/-]+' $(WORKFLOW) | head -n1 | cut -d@ -f2)

LINT_BASE_URL := https://raw.githubusercontent.com/grafana/k6-ci/$(K6_CI_REF)/.golangci.yml
LINT_DIR      ?= build/lint
LINT_BASE     := $(LINT_DIR)/.golangci-base.yml
LINT_FINAL    := $(LINT_DIR)/.golangci.yml
LINT_PATCH    ?= .golangci.patch

.DEFAULT_GOAL := help

## help: Show this list of targets.
.PHONY: help
help:
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | awk -F': ' '{ printf "  %-18s %s\n", $$1, $$2 }'

## build: Build ./k6 with this extension using xk6.
.PHONY: build
build:
	xk6 build --with $(MODULE)=.

## test: Run the Go tests.
.PHONY: test
test:
	go test -count 1 -race -timeout 60s ./...

## it: Run the integration test against a real k6 binary built with xk6.
.PHONY: it
it:
	MODULE=$(MODULE) ./test/it.sh

## typecheck: Type-check the generated client libraries in test/testdata with TypeScript; needs Node.js.
.PHONY: typecheck
typecheck:
	cd test/typecheck && npm ci --no-audit --no-fund && npx tsc -p .

# xk6 lint's versions check requires a vX.Y.Z git tag; re-enable it once the first release is tagged.
## lint: Run golangci-lint with the k6-ci config plus .golangci.patch, then xk6 lint.
.PHONY: lint
lint: $(LINT_FINAL)
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$$(head -n 1 $(LINT_BASE) | tr -d '# ') \
	  run --config=$(LINT_FINAL) ./...
	xk6 lint --disable versions

## format: Format the Go sources.
.PHONY: format
format:
	go fmt ./...

## update-lint-patch: Regenerate .golangci.patch from a locally edited build/lint/.golangci.yml.
.PHONY: update-lint-patch
update-lint-patch: $(LINT_BASE)
	@if [ ! -f $(LINT_FINAL) ]; then \
	  echo "Run 'make lint' first to materialize $(LINT_FINAL), edit it, then re-run."; \
	  exit 1; \
	fi
	-diff -u --label a/.golangci.yml --label b/.golangci.yml $(LINT_BASE) $(LINT_FINAL) > $(LINT_PATCH)

## clean: Remove build output.
.PHONY: clean
clean:
	rm -rf ./k6 ./build

$(LINT_DIR):
	mkdir -p $(LINT_DIR)

# Re-download the base config whenever the pinned ref in $(WORKFLOW) changes.
$(LINT_BASE): $(WORKFLOW) | $(LINT_DIR)
	curl -fsSL $(LINT_BASE_URL) -o $@

$(LINT_FINAL): $(LINT_BASE) $(wildcard $(LINT_PATCH))
	cp $(LINT_BASE) $@
	@if [ -f $(LINT_PATCH) ]; then \
	  echo "Applying $(LINT_PATCH)"; \
	  git apply --directory=$(LINT_DIR) $(LINT_PATCH); \
	fi
