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
THRESHOLD=${CONTROLLER_MAX_FILE_LOC:-350}
ALLOWLIST=${ALLOWLIST:-"${ROOT}/tools/controller-loc-allowlist.txt"}

check_allowlist() {
  local key=$1
  if [[ ! -f "${ALLOWLIST}" ]]; then
    return 1
  fi
  grep -Fxq "${key}" "${ALLOWLIST}"
}

echo "==> controller file size"
failures=()
while IFS= read -r path; do
  rel=${path#"${ROOT}/"}
  lines=$(wc -l < "${path}")
  if (( lines <= THRESHOLD )); then
    continue
  fi
  if check_allowlist "${rel}"; then
    continue
  fi
  failures+=("${lines} ${rel}")
done < <(find "${ROOT}/images/controller/internal" -type f -name '*.go' ! -name '*_test.go' | sort)

if [[ ${#failures[@]} -gt 0 ]]; then
  printf '%s\n' "${failures[@]}" >&2
  exit 1
fi
