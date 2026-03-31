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
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd -- "${script_dir}/.." && pwd)"
patches_dir="${backend_dir}/oidc-auth-patches"
metadata_file="${backend_dir}/oidc-auth.lock"
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

python3 -m pip install --no-cache-dir "${src_dir}"
