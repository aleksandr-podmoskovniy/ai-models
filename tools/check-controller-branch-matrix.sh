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
BASE="${ROOT}/images/controller/internal"

dirs=()
while IFS= read -r dir; do
  dirs+=("${dir}")
done < <(find "${BASE}" -type d \( -path '*/domain/*' -o -path '*/application/*' \) | sort)
if [[ ${#dirs[@]} -eq 0 ]]; then
  echo "==> no domain/application controller packages for branch-matrix gate yet"
  exit 0
fi

echo "==> controller branch-matrix artifacts"
failures=()
for dir in "${dirs[@]}"; do
  if ! find "${dir}" -maxdepth 1 -type f -name '*.go' ! -name '*_test.go' | grep -q .; then
    continue
  fi
  if [[ ! -f "${dir}/BRANCH_MATRIX.ru.md" ]]; then
    failures+=("${dir#${ROOT}/}/BRANCH_MATRIX.ru.md is required")
  fi
done

if [[ ${#failures[@]} -gt 0 ]]; then
  printf '%s\n' "${failures[@]}" >&2
  exit 1
fi
