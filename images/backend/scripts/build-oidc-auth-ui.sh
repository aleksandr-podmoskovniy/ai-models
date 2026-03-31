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
Usage: build-oidc-auth-ui.sh [<source-root>]

Build the upstream mlflow-oidc-auth React UI bundle under mlflow_oidc_auth/ui.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

source_root="${1:-/src/oidc-auth}"
web_dir="${source_root}/web-react"
ui_dir="${source_root}/mlflow_oidc_auth/ui"

if [[ ! -d "${web_dir}" ]]; then
  echo "OIDC auth web source directory does not exist: ${web_dir}" >&2
  exit 1
fi

if ! command -v node >/dev/null 2>&1; then
  echo "node is required to build the OIDC auth UI." >&2
  exit 1
fi

if command -v yarn >/dev/null 2>&1; then
  yarn_cmd=(yarn)
else
  if ! command -v corepack >/dev/null 2>&1; then
    echo "Neither yarn nor corepack is available for OIDC auth UI build." >&2
    exit 1
  fi
  corepack enable >/dev/null 2>&1
  yarn_cmd=(corepack yarn)
fi

cd "${web_dir}"
"${yarn_cmd[@]}" install
"${yarn_cmd[@]}" build

if [[ ! -f "${ui_dir}/index.html" ]]; then
  echo "OIDC auth UI build did not produce ${ui_dir}/index.html" >&2
  exit 1
fi
