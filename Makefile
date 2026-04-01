SHELL := /bin/bash

ROOT := $(CURDIR)
BIN_DIR := $(ROOT)/.bin
export PATH := $(BIN_DIR):$(PATH)

GO ?= $(shell command -v go 2>/dev/null || { test -x /opt/homebrew/bin/go && echo /opt/homebrew/bin/go; } || { test -x /usr/local/go/bin/go && echo /usr/local/go/bin/go; })
GOFMT ?= $(dir $(GO))gofmt
GOFLAGS ?= -count=1
GOLANGCI_LINT_VERSION ?= 2.6.2
DMT_VERSION ?= 0.1.68
MODULE_SDK_VERSION ?= 0.10.3
OPERATOR_SDK_VERSION ?= 1.42.2
HELM_DESIRED_VERSION ?= 3.20.1

GOLANGCI_LINT ?= $(BIN_DIR)/golangci-lint
DMT ?= $(BIN_DIR)/dmt
MODULE_SDK ?= $(BIN_DIR)/module-sdk
OPERATOR_SDK ?= $(BIN_DIR)/operator-sdk
WERF ?= $(shell command -v werf 2>/dev/null || { test -x /opt/homebrew/bin/werf && echo /opt/homebrew/bin/werf; } || { test -x /usr/local/bin/werf && echo /usr/local/bin/werf; })
DOCKER ?= docker
BACKEND_UPSTREAM_METADATA ?= $(ROOT)/images/backend/upstream.lock
BACKEND_VERSION ?= $(shell sed -n 's/^version:[[:space:]]*//p' images/backend/upstream.lock | head -n1)
BACKEND_NODE_IMAGE ?= node:22.19.0-bookworm-slim@sha256:4a4884e8a44826194dff92ba316264f392056cbe243dcc9fd3551e71cea02b90
BACKEND_PYTHON_IMAGE ?= python:3.10-slim-bullseye@sha256:f1fb49e4d5501ac93d0ca519fb7ee6250842245aba8612926a46a0832a1ed089
BACKEND_IMAGE_TAG ?= ai-models-backend:$(subst +,-,$(BACKEND_VERSION))
BACKEND_SOURCE_CACHE_DIR ?= $(ROOT)/.cache/backend-upstream
BACKEND_WORKTREE_DIR ?= $(ROOT)/.cache/backend-worktree
BACKEND_DIST_DIR ?= $(ROOT)/.cache/backend-dist
BACKEND_NODE_MODULES_VOLUME ?= ai-models-backend-js-node-modules
BACKEND_UI_MAX_OLD_SPACE_SIZE ?= 4096
OIDC_AUTH_UPSTREAM_METADATA ?= $(ROOT)/images/backend/oidc-auth.lock

.PHONY: ensure-bin-dir ensure-golangci-lint ensure-dmt ensure-module-sdk ensure-operator-sdk ensure-tools ensure-ci-tools fmt generate test lint-dmt lint-docs lint-shell lint helper-shell-check helm-template kubeconform render-docs verify verify-ci backend-fetch-source backend-oidc-auth-patches-check backend-oidc-auth-install-layout-check backend-oidc-auth-werf-layout-check backend-runtime-entrypoints-check backend-shell-check backend-build-ui backend-build-dist backend-build-image backend-smoke-image backend-build-local werf-build werf-build-dev

ensure-bin-dir:
	@mkdir -p $(BIN_DIR)

ensure-golangci-lint: ensure-bin-dir
	@INSTALL_DIR=$(BIN_DIR) GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION) ./tools/install-golangci-lint.sh

ensure-dmt: ensure-bin-dir
	@INSTALL_DIR=$(BIN_DIR) DMT_VERSION=$(DMT_VERSION) ./tools/install-dmt.sh

ensure-module-sdk: ensure-bin-dir
	@INSTALL_DIR=$(BIN_DIR) MODULE_SDK_VERSION=$(MODULE_SDK_VERSION) ./tools/install-module-sdk.sh

ensure-operator-sdk: ensure-bin-dir
	@INSTALL_DIR=$(BIN_DIR) OPERATOR_SDK_VERSION=$(OPERATOR_SDK_VERSION) ./tools/install-operator-sdk.sh

ensure-tools: ensure-golangci-lint ensure-dmt ensure-module-sdk ensure-operator-sdk

ensure-ci-tools: ensure-dmt

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

test:
	@mods="$$(find . -name go.mod -not -path '*/.cache/*' -not -path '*/vendor/*' -exec dirname {} \; | sort)"; \
	if [[ -z "$$mods" ]]; then \
		echo "==> no Go modules to test yet"; \
	else \
		while IFS= read -r dir; do \
			[[ -n "$$dir" ]] || continue; \
			echo "==> go test ($$dir)"; \
			( cd "$$dir" && $(GO) test $(GOFLAGS) ./... ); \
		done <<< "$$mods"; \
	fi

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

helper-shell-check:
	@echo "==> operator helper shell syntax"
	@bash -n \
		./tools/libai_models_job.sh \
		./tools/run_hf_import_job.sh \
		./tools/run_model_cleanup_job.sh

backend-oidc-auth-patches-check:
	@tmp_dir="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	echo "==> oidc-auth patch queue"; \
	bash ./images/backend/scripts/fetch-oidc-auth-source.sh --metadata "$(OIDC_AUTH_UPSTREAM_METADATA)" --dest "$$tmp_dir/src" $(if $(OIDC_AUTH_SOURCE_DIR),--source "$(OIDC_AUTH_SOURCE_DIR)",) >/dev/null; \
	bash ./images/backend/scripts/apply-patches.sh --check "$$tmp_dir/src" ./images/backend/oidc-auth-patches

backend-oidc-auth-install-layout-check:
	@tmp_dir="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	echo "==> oidc-auth install layout"; \
	mkdir -p "$$tmp_dir/scripts" "$$tmp_dir/oidc-auth-patches" "$$tmp_dir/metadata" "$$tmp_dir/oidc-auth-ui"; \
	cp ./images/backend/scripts/install-oidc-auth-from-source.sh "$$tmp_dir/scripts/"; \
	cp ./images/backend/scripts/fetch-oidc-auth-source.sh "$$tmp_dir/scripts/"; \
	cp ./images/backend/scripts/apply-patches.sh "$$tmp_dir/scripts/"; \
	cp ./images/backend/scripts/build-oidc-auth-ui.sh "$$tmp_dir/scripts/"; \
	cp ./images/backend/oidc-auth.lock "$$tmp_dir/metadata/"; \
	cp ./images/backend/oidc-auth-patches/*.patch "$$tmp_dir/oidc-auth-patches/"; \
	printf '%s\n' '<!doctype html><html><body>oidc-auth-ui</body></html>' >"$$tmp_dir/oidc-auth-ui/index.html"; \
	( cd "$$tmp_dir" && OIDC_AUTH_METADATA_FILE="$$tmp_dir/metadata/oidc-auth.lock" OIDC_AUTH_PATCHES_DIR="$$tmp_dir/oidc-auth-patches" OIDC_AUTH_PREBUILT_UI_DIR="$$tmp_dir/oidc-auth-ui" OIDC_AUTH_SKIP_PIP_INSTALL=true bash "$$tmp_dir/scripts/install-oidc-auth-from-source.sh" >/dev/null )

backend-oidc-auth-werf-layout-check:
	@echo "==> oidc-auth werf layout"; \
	awk 'BEGIN { in_backend=0; has_image=0; has_to=0 } \
		/^---$$/ { if (in_backend) exit; next } \
		/^image: backend$$/ { in_backend=1; next } \
		in_backend && /image: backend-oidc-auth-ui-build/ { has_image=1 } \
		in_backend && /to: \/oidc-auth-ui/ { has_to=1 } \
		END { if (!(has_image && has_to)) { print "final backend image in images/backend/werf.inc.yaml must import backend-oidc-auth-ui-build to /oidc-auth-ui" > "/dev/stderr"; exit 1 } }' \
		./images/backend/werf.inc.yaml

backend-runtime-entrypoints-check:
	@echo "==> backend runtime entrypoints"; \
	grep -Fq 'ai-models-backend-model-cleanup' ./images/backend/werf.inc.yaml; \
	grep -Fq 'ai-models-backend-model-cleanup' ./images/backend/Dockerfile.local; \
	grep -Fq 'ai-models-backend-model-cleanup --help' ./images/backend/scripts/smoke-runtime.sh

lint: lint-dmt lint-docs lint-shell

helm-template:
	@./tools/helm-tests/helm-template.sh

kubeconform:
	@./tools/kubeconform/kubeconform.sh

render-docs:
	@python3 ./tools/render-docs.py

verify: lint helper-shell-check test helm-template kubeconform backend-oidc-auth-patches-check backend-oidc-auth-install-layout-check backend-oidc-auth-werf-layout-check backend-runtime-entrypoints-check

verify-ci: lint helper-shell-check test helm-template kubeconform backend-oidc-auth-patches-check backend-oidc-auth-install-layout-check backend-oidc-auth-werf-layout-check backend-runtime-entrypoints-check

backend-fetch-source:
	@bash ./images/backend/scripts/fetch-source.sh --metadata "$(BACKEND_UPSTREAM_METADATA)" --dest "$(BACKEND_SOURCE_CACHE_DIR)" $(if $(BACKEND_SOURCE_DIR),--source "$(BACKEND_SOURCE_DIR)",)

backend-shell-check:
	@bash -n ./images/backend/scripts/*.sh

backend-build-ui: backend-fetch-source
	@rm -rf "$(BACKEND_WORKTREE_DIR)"
	@mkdir -p "$(BACKEND_WORKTREE_DIR)"
	@tar -C "$(BACKEND_SOURCE_CACHE_DIR)" -cf - . | tar -C "$(BACKEND_WORKTREE_DIR)" -xf -
	@bash ./images/backend/scripts/apply-patches.sh "$(BACKEND_WORKTREE_DIR)" ./images/backend/patches
	@$(DOCKER) run --rm \
		-v "$(ROOT)":/work \
		-v "$(BACKEND_NODE_MODULES_VOLUME)":/work/.cache/backend-node-modules \
		-e BACKEND_UI_MAX_OLD_SPACE_SIZE=$(BACKEND_UI_MAX_OLD_SPACE_SIZE) \
		-e BACKEND_NODE_MODULES_DIR=/work/.cache/backend-node-modules \
		-w /work \
		$(BACKEND_NODE_IMAGE) \
		bash -lc 'set -euo pipefail; \
			bash images/backend/scripts/apt-install.sh ca-certificates git python3 make g++; \
			bash images/backend/scripts/build-ui.sh /work/.cache/backend-worktree'

backend-build-dist: backend-build-ui
	@rm -rf "$(BACKEND_DIST_DIR)"
	@mkdir -p "$(BACKEND_DIST_DIR)"
	@$(DOCKER) run --rm \
		-v "$(ROOT)":/work \
		-w /work \
		$(BACKEND_PYTHON_IMAGE) \
		bash -lc 'set -euo pipefail; \
			bash images/backend/scripts/apt-install.sh ca-certificates git; \
			python3 -m pip install --no-cache-dir --upgrade pip >/tmp/pip.log; \
			python3 -m pip install --no-cache-dir build setuptools wheel >>/tmp/pip.log; \
			bash images/backend/scripts/build-distributions.sh /work/.cache/backend-worktree /work/.cache/backend-dist; \
			bash images/backend/scripts/smoke-release-install.sh /work/.cache/backend-dist'

backend-build-image: backend-build-ui
	@$(DOCKER) build --progress=plain \
		--build-arg BACKEND_NODE_IMAGE=$(BACKEND_NODE_IMAGE) \
		--build-arg BACKEND_PYTHON_IMAGE=$(BACKEND_PYTHON_IMAGE) \
		--target runtime \
		-t $(BACKEND_IMAGE_TAG) \
		-f images/backend/Dockerfile.local \
		.

backend-smoke-image:
	@$(DOCKER) run --rm $(BACKEND_IMAGE_TAG) --version
	@$(DOCKER) run --rm $(BACKEND_IMAGE_TAG) server --help >/dev/null

backend-build-local: backend-build-dist backend-build-image backend-smoke-image

werf-build:
	@$(WERF) build $(if $(WERF_ENV),--env $(WERF_ENV),)

werf-build-dev:
	@$(WERF) build --dev $(if $(WERF_ENV),--env $(WERF_ENV),)
