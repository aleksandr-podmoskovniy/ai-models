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
GOCYCLO=${GOCYCLO:-"${BIN_DIR}/gocyclo"}
THRESHOLD=${CONTROLLER_MAX_CYCLO:-15}
ALLOWLIST=${ALLOWLIST:-"${ROOT}/tools/controller-complexity-allowlist.txt"}

if [[ ! -x "${GOCYCLO}" ]]; then
  echo "gocyclo binary not found: ${GOCYCLO}" >&2
  exit 1
fi

files=()
while IFS= read -r file; do
  files+=("${file}")
done < <(find "${ROOT}/images/controller/internal" -type f -name '*.go' ! -name '*_test.go' | sort)
if [[ ${#files[@]} -eq 0 ]]; then
  echo "==> no controller Go files for complexity check"
  exit 0
fi

check_allowlist() {
  local key=$1
  if [[ ! -f "${ALLOWLIST}" ]]; then
    return 1
  fi
  grep -Fxq "${key}" "${ALLOWLIST}"
}

echo "==> controller cyclomatic complexity"
output="$("${GOCYCLO}" -over "${THRESHOLD}" "${files[@]}" || true)"
if [[ -z "${output}" ]]; then
  exit 0
fi

failures=()
while IFS= read -r line; do
  [[ -n "${line}" ]] || continue
  read -r complexity _package symbol location <<<"${line}"
  path=${location%%:*}
  path=${path#"${ROOT}/"}
  key="${path}|${symbol}"
  if check_allowlist "${key}"; then
    continue
  fi
  failures+=("${line}")
done <<<"${output}"

if [[ ${#failures[@]} -gt 0 ]]; then
  printf '%s\n' "${failures[@]}" >&2
  exit 1
fi
