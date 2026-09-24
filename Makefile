# K6_CI_REF pins the grafana/k6-ci ref to fetch the base lint config from (currently v0.5.0).
K6_CI_REF     ?= 03469f9ae7aad25369827832c456e8f8b7b8c1c2
LINT_BASE_URL := https://raw.githubusercontent.com/grafana/k6-ci/$(K6_CI_REF)/.golangci.yml
LINT_DIR      ?= build/lint
LINT_BASE     := $(LINT_DIR)/.golangci-base.yml
LINT_FINAL    := $(LINT_DIR)/.golangci.yml
LINT_PATCH    ?= .golangci.patch

$(LINT_DIR):
	mkdir -p $(LINT_DIR)

$(LINT_BASE): | $(LINT_DIR)
	curl -fsSL $(LINT_BASE_URL) -o $@

$(LINT_FINAL): $(LINT_BASE) $(wildcard $(LINT_PATCH))
	cp $(LINT_BASE) $@
	@if [ -f $(LINT_PATCH) ]; then \
	  echo "Applying $(LINT_PATCH)"; \
	  git apply --directory=$(LINT_DIR) $(LINT_PATCH); \
	fi

## lint: Run golangci-lint with the k6-ci config + $(LINT_PATCH).
lint: $(LINT_FINAL)
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$$(head -n 1 $(LINT_BASE) | tr -d '# ') \
	  run --config=$(LINT_FINAL) ./...

## update-lint-patch: Regenerate $(LINT_PATCH) from the locally edited $(LINT_FINAL).
update-lint-patch: $(LINT_BASE)
	@if [ ! -f $(LINT_FINAL) ]; then \
	  echo "Run 'make lint' first to materialize $(LINT_FINAL), edit it, then re-run."; \
	  exit 1; \
	fi
	-diff -u --label a/.golangci.yml --label b/.golangci.yml $(LINT_BASE) $(LINT_FINAL) > $(LINT_PATCH)

## clean-lint: Remove $(LINT_DIR).
clean-lint:
	rm -rf $(LINT_DIR)
