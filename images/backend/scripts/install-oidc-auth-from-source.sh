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
Usage: install-oidc-auth-from-source.sh

Fetch the pinned mlflow-oidc-auth source, apply the local patch queue, and install the patched package.

Set `OIDC_AUTH_SKIP_PIP_INSTALL=true` to stop after fetch/apply validation.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd -- "${script_dir}/.." && pwd)"
patches_dir="${OIDC_AUTH_PATCHES_DIR:-}"
metadata_file="${OIDC_AUTH_METADATA_FILE:-}"

if [[ -z "${patches_dir}" ]]; then
  if [[ -d "${backend_dir}/oidc-auth-patches" ]]; then
    patches_dir="${backend_dir}/oidc-auth-patches"
  elif [[ -d /oidc-auth-patches ]]; then
    patches_dir="/oidc-auth-patches"
  else
    echo "Failed to locate the mlflow-oidc-auth patch bundle." >&2
    exit 1
  fi
fi

if [[ -z "${metadata_file}" ]]; then
  if [[ -f "${backend_dir}/oidc-auth.lock" ]]; then
    metadata_file="${backend_dir}/oidc-auth.lock"
  elif [[ -f /metadata/oidc-auth.lock ]]; then
    metadata_file="/metadata/oidc-auth.lock"
  else
    echo "Failed to locate the mlflow-oidc-auth metadata lock file." >&2
    exit 1
  fi
fi

workdir="$(mktemp -d)"
src_dir="${workdir}/src"

cleanup() {
  rm -rf "${workdir}"
}
trap cleanup EXIT

bash "${script_dir}/fetch-oidc-auth-source.sh" \
  --metadata "${metadata_file}" \
  --dest "${src_dir}"

bash "${script_dir}/apply-patches.sh" "${src_dir}" "${patches_dir}"

if [[ "${OIDC_AUTH_SKIP_PIP_INSTALL:-false}" == "true" ]]; then
  exit 0
fi

python3 -m pip install --no-cache-dir "${src_dir}"
