#!/usr/bin/env bash

# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN_DIR="${ROOT}/.bin"
COVERAGE_DIR="${ROOT}/artifacts/coverage"

GO="${GO:-}"
if [[ -z "${GO}" ]]; then
  if command -v go >/dev/null 2>&1; then
    GO="$(command -v go)"
  elif [[ -x /opt/homebrew/bin/go ]]; then
    GO=/opt/homebrew/bin/go
  elif [[ -x /usr/local/go/bin/go ]]; then
    GO=/usr/local/go/bin/go
  fi
fi

export PATH="${BIN_DIR}:${PATH}"

TEST_FLAGS=()
if [[ -n "${GOFLAGS:-}" ]]; then
  # shellcheck disable=SC2206
  TEST_FLAGS=(${GOFLAGS})
else
  TEST_FLAGS=(-count=1)
fi

ensure_dmt() {
  INSTALL_DIR="${BIN_DIR}" DMT_VERSION="${DMT_VERSION:-0.1.69}" "${ROOT}/tools/install-dmt.sh"
}

ensure_gocyclo() {
  INSTALL_DIR="${BIN_DIR}" GOCYCLO_VERSION="${GOCYCLO_VERSION:-0.6.0}" "${ROOT}/tools/install-gocyclo.sh"
}

ensure_deadcode() {
  if [[ -z "${GO}" ]]; then
    echo "go binary is required for deadcode install" >&2
    exit 1
  fi
  INSTALL_DIR="${BIN_DIR}" DEADCODE_VERSION="${DEADCODE_VERSION:-0.43.0}" "${ROOT}/tools/install-deadcode.sh"
}

require_go() {
  if [[ -z "${GO}" ]]; then
    echo "go binary is required for this suite" >&2
    exit 1
  fi
}

run_lint() {
  ensure_dmt

  echo "==> dmt lint"
  "${BIN_DIR}/dmt" lint ./

  echo "==> docs markers"
  python3 "${ROOT}/tools/render-docs.py" --check

  echo "==> shell syntax"
  files=()
  while IFS= read -r file; do
    files+=("${file}")
  done < <(find "${ROOT}/images" -type f -path "${ROOT}/images/*/scripts/*.sh" | sort)
  if [[ ${#files[@]} -eq 0 ]]; then
    echo "==> no shell scripts to check"
  else
    bash -n "${files[@]}"
  fi
}

run_tests() {
  require_go
  mkdir -p "${COVERAGE_DIR}"

  echo "==> go test (api)"
  (
    cd "${ROOT}/api"
    "${GO}" test "${TEST_FLAGS[@]}" -coverprofile "${COVERAGE_DIR}/api.out" ./...
  )

  echo "==> go test (images/controller)"
  (
    cd "${ROOT}/images/controller"
    "${GO}" test "${TEST_FLAGS[@]}" -coverprofile "${COVERAGE_DIR}/controller.out" ./...
  )

  echo "==> go test (images/hooks)"
  (
    cd "${ROOT}/images/hooks"
    "${GO}" test "${TEST_FLAGS[@]}" -coverprofile "${COVERAGE_DIR}/hooks.out" ./...
  )

  echo "==> go test (images/dmcr)"
  (
    cd "${ROOT}/images/dmcr"
    "${GO}" test "${TEST_FLAGS[@]}" ./...
  )
}

run_verify() {
  require_go
  run_lint
  ensure_gocyclo
  ensure_deadcode

  ROOT="${ROOT}" BIN_DIR="${BIN_DIR}" GOCYCLO="${BIN_DIR}/gocyclo" "${ROOT}/tools/check-controller-complexity.sh"
  ROOT="${ROOT}" "${ROOT}/tools/check-controller-loc.sh"
  ROOT="${ROOT}" "${ROOT}/tools/check-controller-test-loc.sh"
  ROOT="${ROOT}" python3 "${ROOT}/tools/check-codex-governance.py"
  ROOT="${ROOT}" "${ROOT}/tools/check-thin-reconcilers.sh"

  echo "==> tools shell syntax"
  tool_files=()
  while IFS= read -r file; do
    tool_files+=("${file}")
  done < <(find "${ROOT}/tools" -type f -name '*.sh' | sort)
  if [[ ${#tool_files[@]} -eq 0 ]]; then
    echo "==> no tools shell scripts to check"
  else
    bash -n "${tool_files[@]}"
  fi

  ROOT="${ROOT}" COVERAGE_DIR="${COVERAGE_DIR}" "${ROOT}/tools/collect-controller-coverage.sh"
  ROOT="${ROOT}" COVERAGE_DIR="${COVERAGE_DIR}" GO="${GO}" "${ROOT}/tools/test-controller-coverage.sh"
  ROOT="${ROOT}" "${ROOT}/tools/check-controller-test-evidence.sh"
  ROOT="${ROOT}" BIN_DIR="${BIN_DIR}" DEADCODE="${BIN_DIR}/deadcode" MODE=controller "${ROOT}/tools/check-controller-deadcode.sh"
  ROOT="${ROOT}" BIN_DIR="${BIN_DIR}" DEADCODE="${BIN_DIR}/deadcode" MODE=hooks "${ROOT}/tools/check-controller-deadcode.sh"

  run_tests

  "${ROOT}/tools/helm-tests/helm-template.sh"
  "${ROOT}/tools/kubeconform/kubeconform.sh"
}

case "${1:-}" in
  lint)
    run_lint
    ;;
  test)
    run_tests
    ;;
  verify)
    run_verify
    ;;
  *)
    echo "usage: $0 {lint|test|verify}" >&2
    exit 1
    ;;
esac
