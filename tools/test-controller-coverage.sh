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
THRESHOLD=${CONTROLLER_MIN_DOMAIN_COVERAGE:-80}
GO_BIN=${GO:-go}
COVERAGE_DIR=${COVERAGE_DIR:-"${ROOT}/artifacts/coverage"}
MODULE_DIR="${ROOT}/images/controller"

profiles=()
while IFS= read -r profile; do
  profiles+=("${profile}")
done < <(find "${COVERAGE_DIR}" -maxdepth 1 -type f -name 'controller-*.out' | sort)

if [[ ${#profiles[@]} -eq 0 ]]; then
  echo "==> no controller coverage profiles found under ${COVERAGE_DIR}"
  exit 0
fi

echo "==> controller domain/application coverage"
failures=()
for profile in "${profiles[@]}"; do
  output=$(cd "${MODULE_DIR}" && "${GO_BIN}" tool cover -func="${profile}")
  printf '%s\n' "${output}"
  coverage=$(awk '/^total:/{gsub(/%/, "", $3); print $3}' <<<"${output}" | tail -n1)
  if [[ -z "${coverage}" ]]; then
    failures+=("missing coverage output for ${profile}")
    continue
  fi
  if awk -v got="${coverage}" -v min="${THRESHOLD}" 'BEGIN {exit !(got+0 < min+0)}'; then
    failures+=("coverage ${coverage}% is below ${THRESHOLD}% for ${profile}")
  fi
done

if [[ ${#failures[@]} -gt 0 ]]; then
  printf '%s\n' "${failures[@]}" >&2
  exit 1
fi
