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
Usage: build-ui.sh [<source-root>]

Build the upstream backend UI bundle under mlflow/server/js/build.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

source_root="${1:-/src/backend}"
js_dir="${source_root}/mlflow/server/js"
rc_file="${js_dir}/.yarnrc.yml"
max_old_space_size="${BACKEND_UI_MAX_OLD_SPACE_SIZE:-4096}"
node_modules_dir="${BACKEND_NODE_MODULES_DIR:-}"

if [[ ! -d "${js_dir}" ]]; then
  echo "Backend UI source directory does not exist: ${js_dir}" >&2
  exit 1
fi

if ! command -v node >/dev/null 2>&1; then
  echo "node is required to build the backend UI." >&2
  exit 1
fi

if [[ ! -f "${rc_file}" ]]; then
  echo "Missing Yarn configuration: ${rc_file}" >&2
  exit 1
fi

yarn_relpath="$(sed -n 's/^yarnPath:[[:space:]]*//p' "${rc_file}" | head -n1)"
if [[ -z "${yarn_relpath}" ]]; then
  echo "Failed to resolve yarnPath from ${rc_file}" >&2
  exit 1
fi

yarn_bin="${js_dir}/${yarn_relpath}"
if [[ ! -f "${yarn_bin}" ]]; then
  echo "Yarn runtime referenced by .yarnrc.yml is missing: ${yarn_bin}" >&2
  exit 1
fi

if [[ ! "${max_old_space_size}" =~ ^[0-9]+$ ]]; then
  echo "BACKEND_UI_MAX_OLD_SPACE_SIZE must be an integer, got: ${max_old_space_size}" >&2
  exit 1
fi

if [[ -n "${node_modules_dir}" ]]; then
  mkdir -p "${node_modules_dir}"
  rm -rf "${js_dir}/node_modules"
  ln -s "${node_modules_dir}" "${js_dir}/node_modules"
fi

cd "${js_dir}"
node "${yarn_bin}" install --immutable
GENERATE_SOURCEMAP=false node "${yarn_bin}" exec craco --max_old_space_size="${max_old_space_size}" build
