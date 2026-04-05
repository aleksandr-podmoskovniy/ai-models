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
THRESHOLD=${CONTROLLER_MAX_RECONCILER_LOC:-250}
ALLOWLIST=${ALLOWLIST:-"${ROOT}/tools/thin-reconciler-allowlist.txt"}

check_allowlist() {
  local key=$1
  if [[ ! -f "${ALLOWLIST}" ]]; then
    return 1
  fi
  grep -Fxq "${key}" "${ALLOWLIST}"
}

echo "==> thin reconciler check"
failures=()
patterns=(
  'corev1\.Pod\s*\{'
  'corev1\.Service\s*\{'
  'corev1\.Secret\s*\{'
  'corev1\.ConfigMap\s*\{'
)

while IFS= read -r path; do
  rel=${path#"${ROOT}/"}
  if check_allowlist "${rel}"; then
    continue
  fi

  lines=$(wc -l < "${path}")
  if (( lines > THRESHOLD )); then
    failures+=("reconciler too large: ${lines} ${rel}")
  fi

  for pattern in "${patterns[@]}"; do
    if grep -Eq "${pattern}" "${path}"; then
      failures+=("reconciler renders K8s object inline: ${rel}")
      break
    fi
  done
done < <(find "${ROOT}/images/controller/internal" -type f -name 'reconciler.go' | sort)

if [[ ${#failures[@]} -gt 0 ]]; then
  printf '%s\n' "${failures[@]}" >&2
  exit 1
fi
