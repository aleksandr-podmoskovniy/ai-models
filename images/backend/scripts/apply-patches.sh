#!/usr/bin/env bash
#
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

usage() {
  cat <<'EOF'
Usage: apply-patches.sh [--check] [<source-root> [<patches-dir>]]

Apply or validate the local patch queue against an imported upstream source tree.
EOF
}

check_only=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)
      check_only=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      break
      ;;
  esac
done

source_root="${1:-/src/backend}"
patches_dir="${2:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)/patches}"

if [[ ! -d "${source_root}" ]]; then
  echo "Backend source root does not exist: ${source_root}" >&2
  exit 1
fi
source_root="$(cd -- "${source_root}" && pwd)"

if [[ ! -d "${patches_dir}" ]]; then
  echo "Patch directory does not exist: ${patches_dir}" >&2
  exit 1
fi
patches_dir="$(cd -- "${patches_dir}" && pwd)"

shopt -s nullglob
patches=("${patches_dir}"/*.patch)
shopt -u nullglob

if [[ ${#patches[@]} -eq 0 ]]; then
  echo "No backend patches to apply."
  exit 0
fi

for patch in "${patches[@]}"; do
  if [[ ${check_only} -eq 1 ]]; then
    echo "Checking ${patch}"
    (cd "${source_root}" && git apply --check --ignore-space-change --ignore-whitespace "${patch}")
  else
    echo "Applying ${patch}"
    (cd "${source_root}" && git apply --ignore-space-change --ignore-whitespace "${patch}")
  fi
done
