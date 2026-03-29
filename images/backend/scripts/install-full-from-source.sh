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
Usage: install-full-from-source.sh [<source-root>]

Install the upstream-equivalent backend full image payload from source using locally imported libs.
Optional overlays can be added via BACKEND_OVERLAY_EXTRAS, for example "auth".
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

source_root="${1:-/src/backend}"
base_extras="${BACKEND_BASE_EXTRAS:-extras,azure,db,gateway,genai}"
overlay_extras="${BACKEND_OVERLAY_EXTRAS:-}"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to install the backend from source." >&2
  exit 1
fi

if [[ ! -d "${source_root}" ]]; then
  echo "Backend source directory does not exist: ${source_root}" >&2
  exit 1
fi

ui_build_dir="${source_root}/mlflow/server/js/build"
if [[ ! -d "${ui_build_dir}" ]] || [[ -z "$(find "${ui_build_dir}" -mindepth 1 -print -quit 2>/dev/null || true)" ]]; then
  echo "Backend UI assets are missing at ${ui_build_dir}" >&2
  exit 1
fi

extras="${base_extras}"
if [[ -n "${overlay_extras}" ]]; then
  extras="${extras},${overlay_extras}"
fi

backup="$(mktemp "${source_root}/pyproject.toml.XXXXXX")"
cp "${source_root}/pyproject.toml" "${backup}"
trap 'mv -f "'"${backup}"'" "'"${source_root}/pyproject.toml"'"' EXIT

cp "${source_root}/pyproject.release.toml" "${source_root}/pyproject.toml"
python3 -m pip install --no-cache-dir \
  "${source_root}/libs/skinny" \
  "${source_root}/libs/tracing" \
  "${source_root}[${extras}]"
