SHELL := /bin/bash

ROOT := $(CURDIR)
BIN_DIR := $(ROOT)/.bin
COVERAGE_DIR := $(ROOT)/artifacts/coverage
export PATH := $(BIN_DIR):$(PATH)

GO ?= $(shell command -v go 2>/dev/null || { test -x /opt/homebrew/bin/go && echo /opt/homebrew/bin/go; } || { test -x /usr/local/go/bin/go && echo /usr/local/go/bin/go; })
GOFMT ?= $(dir $(GO))gofmt
GOFLAGS ?= -count=1
GOLANGCI_LINT_VERSION ?= 2.11.1
GOCYCLO_VERSION ?= 0.6.0
DEADCODE_VERSION ?= 0.43.0
DMT_VERSION ?= 0.1.69
MODULE_SDK_VERSION ?= 0.10.0
OPERATOR_SDK_VERSION ?= 1.42.2
HELM_DESIRED_VERSION ?= 3.20.1

GOLANGCI_LINT ?= $(BIN_DIR)/golangci-lint
GOCYCLO ?= $(BIN_DIR)/gocyclo
DEADCODE ?= $(BIN_DIR)/deadcode
DMT ?= $(BIN_DIR)/dmt
MODULE_SDK ?= $(BIN_DIR)/module-sdk
OPERATOR_SDK ?= $(BIN_DIR)/operator-sdk
WERF ?= $(shell command -v werf 2>/dev/null || { test -x /opt/homebrew/bin/werf && echo /opt/homebrew/bin/werf; } || { test -x /usr/local/bin/werf && echo /usr/local/bin/werf; })

.PHONY: ensure-bin-dir ensure-golangci-lint ensure-gocyclo ensure-deadcode ensure-dmt ensure-module-sdk ensure-operator-sdk ensure-tools ensure-ci-tools coverage-dir api-test controller-test hooks-test dmcr-test controller-coverage-artifacts fmt generate test lint-dmt lint-docs lint-shell lint-controller-complexity lint-controller-size lint-controller-test-size lint-codex-governance lint-thin-reconcilers test-controller-coverage deadcode deadcode-controller deadcode-hooks check-controller-test-evidence lint helper-shell-check helm-template kubeconform render-docs verify verify-ci werf-build werf-build-dev

ensure-bin-dir:
	@mkdir -p $(BIN_DIR)

ensure-golangci-lint: ensure-bin-dir
	@INSTALL_DIR=$(BIN_DIR) GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION) ./tools/install-golangci-lint.sh

ensure-gocyclo: ensure-bin-dir
	@INSTALL_DIR=$(BIN_DIR) GOCYCLO_VERSION=$(GOCYCLO_VERSION) ./tools/install-gocyclo.sh

ensure-deadcode: ensure-bin-dir
	@INSTALL_DIR=$(BIN_DIR) DEADCODE_VERSION=$(DEADCODE_VERSION) ./tools/install-deadcode.sh

ensure-dmt: ensure-bin-dir
	@INSTALL_DIR=$(BIN_DIR) DMT_VERSION=$(DMT_VERSION) ./tools/install-dmt.sh

ensure-module-sdk: ensure-bin-dir
	@INSTALL_DIR=$(BIN_DIR) MODULE_SDK_VERSION=$(MODULE_SDK_VERSION) ./tools/install-module-sdk.sh

ensure-operator-sdk: ensure-bin-dir
	@INSTALL_DIR=$(BIN_DIR) OPERATOR_SDK_VERSION=$(OPERATOR_SDK_VERSION) ./tools/install-operator-sdk.sh

ensure-tools: ensure-golangci-lint ensure-gocyclo ensure-deadcode ensure-dmt ensure-module-sdk ensure-operator-sdk

ensure-ci-tools: ensure-gocyclo ensure-deadcode ensure-dmt

coverage-dir:
	@mkdir -p $(COVERAGE_DIR)

fmt:
	@files="$$(find . -type f -name '*.go' -not -path '*/.cache/*' -not -path '*/vendor/*' | sort)"; \
	if [[ -z "$$files" ]]; then \
		echo "==> no Go files to format"; \
	else \
		echo "==> gofmt"; \
		$(GOFMT) -w $$files; \
	fi

generate:
	@mods="$$(find . -name go.mod -not -path '*/.cache/*' -not -path '*/vendor/*' -exec dirname {} \; | sort)"; \
	if [[ -z "$$mods" ]]; then \
		echo "==> no Go modules to generate"; \
	else \
		echo "==> go generate"; \
		while IFS= read -r dir; do \
			[[ -n "$$dir" ]] || continue; \
			( cd "$$dir" && $(GO) generate ./... ); \
		done <<< "$$mods"; \
	fi

api-test: coverage-dir
	@echo "==> go test (api)"
	@cd api && $(GO) test $(GOFLAGS) -coverprofile $(COVERAGE_DIR)/api.out ./...

controller-test: coverage-dir
	@echo "==> go test (images/controller)"
	@cd images/controller && $(GO) test $(GOFLAGS) -coverprofile $(COVERAGE_DIR)/controller.out ./...

hooks-test: coverage-dir
	@echo "==> go test (images/hooks)"
	@cd images/hooks && $(GO) test $(GOFLAGS) -coverprofile $(COVERAGE_DIR)/hooks.out ./...

dmcr-test:
	@echo "==> go test (images/dmcr)"
	@cd images/dmcr && $(GO) test $(GOFLAGS) ./...

test: api-test controller-test hooks-test dmcr-test

lint-dmt: ensure-dmt
	@echo "==> dmt lint"
	@$(DMT) lint ./

lint-docs:
	@echo "==> docs markers"
	@python3 ./tools/render-docs.py --check

lint-shell:
	@echo "==> shell syntax"
	@files="$$(find ./images -type f -path './images/*/scripts/*.sh' | sort)"; \
	if [[ -z "$$files" ]]; then \
		echo "==> no shell scripts to check"; \
	else \
		bash -n $$files; \
	fi

lint-controller-complexity: ensure-gocyclo
	@ROOT=$(ROOT) BIN_DIR=$(BIN_DIR) GOCYCLO=$(GOCYCLO) ./tools/check-controller-complexity.sh

lint-controller-size:
	@ROOT=$(ROOT) ./tools/check-controller-loc.sh

lint-controller-test-size:
	@ROOT=$(ROOT) ./tools/check-controller-test-loc.sh

lint-codex-governance:
	@ROOT=$(ROOT) python3 ./tools/check-codex-governance.py

lint-thin-reconcilers:
	@ROOT=$(ROOT) ./tools/check-thin-reconcilers.sh

test-controller-coverage: controller-coverage-artifacts
	@ROOT=$(ROOT) COVERAGE_DIR=$(COVERAGE_DIR) GO=$(GO) ./tools/test-controller-coverage.sh

controller-coverage-artifacts: coverage-dir
	@ROOT=$(ROOT) COVERAGE_DIR=$(COVERAGE_DIR) ./tools/collect-controller-coverage.sh

deadcode-controller: ensure-deadcode
	@ROOT=$(ROOT) BIN_DIR=$(BIN_DIR) DEADCODE=$(DEADCODE) MODE=controller ./tools/check-controller-deadcode.sh

deadcode-hooks: ensure-deadcode
	@ROOT=$(ROOT) BIN_DIR=$(BIN_DIR) DEADCODE=$(DEADCODE) MODE=hooks ./tools/check-controller-deadcode.sh

deadcode: deadcode-controller deadcode-hooks

check-controller-test-evidence:
	@ROOT=$(ROOT) ./tools/check-controller-test-evidence.sh

helper-shell-check:
	@echo "==> tools shell syntax"
	@files="$$(find ./tools -type f -name '*.sh' | sort)"; \
	if [[ -z "$$files" ]]; then \
		echo "==> no tools shell scripts to check"; \
	else \
		bash -n $$files; \
	fi

lint: lint-dmt lint-docs lint-shell

helm-template:
	@./tools/helm-tests/helm-template.sh

kubeconform:
	@./tools/kubeconform/kubeconform.sh

render-docs:
	@python3 ./tools/render-docs.py

verify: lint lint-controller-complexity lint-controller-size lint-controller-test-size lint-codex-governance lint-thin-reconcilers helper-shell-check test-controller-coverage check-controller-test-evidence deadcode test helm-template kubeconform

verify-ci: lint lint-controller-complexity lint-controller-size lint-controller-test-size lint-codex-governance lint-thin-reconcilers helper-shell-check test-controller-coverage check-controller-test-evidence deadcode test helm-template kubeconform

werf-build:
	@$(WERF) build $(if $(WERF_ENV),--env $(WERF_ENV),)

werf-build-dev:
	@$(WERF) build --dev $(if $(WERF_ENV),--env $(WERF_ENV),)
