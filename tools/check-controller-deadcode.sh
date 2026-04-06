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

ROOT=${ROOT:-$(pwd)}
BIN_DIR=${BIN_DIR:-"${ROOT}/.bin"}
DEADCODE=${DEADCODE:-"${BIN_DIR}/deadcode"}
CONTROLLER_DIR="${ROOT}/images/controller"
HOOKS_DIR="${ROOT}/images/hooks"
MODE=${MODE:-all}

if [[ ! -x "${DEADCODE}" ]]; then
  echo "deadcode binary not found: ${DEADCODE}" >&2
  exit 1
fi

run_controller_deadcode() {
  echo "==> deadcode (controller, required)"
  (
    cd "${CONTROLLER_DIR}" &&
      "${DEADCODE}" -test ./cmd/... ./internal/...
  )
}

run_hooks_deadcode() {
  if [[ ! -d "${HOOKS_DIR}" ]]; then
    return 0
  fi

  echo "==> deadcode (hooks)"
  (
    cd "${HOOKS_DIR}" &&
      "${DEADCODE}" -test ./...
  )
}

case "${MODE}" in
  controller)
    run_controller_deadcode
    ;;
  hooks)
    run_hooks_deadcode
    ;;
  all)
    run_controller_deadcode
    run_hooks_deadcode
    ;;
  *)
    echo "unsupported deadcode mode: ${MODE}" >&2
    exit 1
    ;;
esac
