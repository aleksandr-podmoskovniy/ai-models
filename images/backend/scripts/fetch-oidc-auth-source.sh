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
Usage: fetch-oidc-auth-source.sh [--source <checkout>] [--repo <git-url>] [--ref <git-ref>] [--dest <dir>] [--metadata <file>] [--keep-git]

Fetch the pinned mlflow-oidc-auth source tree for local patch validation or image builds.
EOF
}

source_dir="${OIDC_AUTH_SOURCE_DIR:-}"
repo_url=""
ref=""
commit=""
dest=""
metadata_file=""
keep_git=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source)
      source_dir="${2:?missing value for --source}"
      shift 2
      ;;
    --repo)
      repo_url="${2:?missing value for --repo}"
      shift 2
      ;;
    --ref)
      ref="${2:?missing value for --ref}"
      shift 2
      ;;
    --dest)
      dest="${2:?missing value for --dest}"
      shift 2
      ;;
    --metadata)
      metadata_file="${2:?missing value for --metadata}"
      shift 2
      ;;
    --keep-git)
      keep_git=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd -- "${script_dir}/.." && pwd)"
default_metadata="${backend_dir}/oidc-auth.lock"

if [[ -z "${metadata_file}" && -f "${default_metadata}" ]]; then
  metadata_file="${default_metadata}"
fi

if [[ -n "${metadata_file}" ]]; then
  # shellcheck disable=SC1090
  source "${metadata_file}"
  repo_url="${repo_url:-${REPO_URL:-}}"
  ref="${ref:-${REF:-}}"
  commit="${commit:-${COMMIT:-}}"
fi

repo_url="${repo_url:-https://github.com/mlflow-oidc/mlflow-oidc-auth.git}"
ref="${ref:-v6.7.1}"
dest="${dest:-${backend_dir}/../.cache/mlflow-oidc-auth-upstream}"

rm -rf "${dest}"
mkdir -p "$(dirname "${dest}")"

if [[ -n "${source_dir}" ]]; then
  if [[ ! -d "${source_dir}" ]]; then
    echo "OIDC auth source directory does not exist: ${source_dir}" >&2
    exit 1
  fi
  cp -a "${source_dir}" "${dest}"
else
  git clone --depth 1 --branch "${ref}" "${repo_url}" "${dest}"
  if [[ -n "${commit}" ]]; then
    current_commit="$(git -C "${dest}" rev-parse HEAD)"
    if [[ "${current_commit}" != "${commit}" ]]; then
      git -C "${dest}" fetch --depth 1 origin "${commit}"
      git -C "${dest}" checkout "${commit}"
      current_commit="$(git -C "${dest}" rev-parse HEAD)"
    fi
    if [[ "${current_commit}" != "${commit}" ]]; then
      echo "Fetched mlflow-oidc-auth revision ${current_commit}, expected ${commit}" >&2
      exit 1
    fi
  fi
fi

if [[ ${keep_git} -ne 1 ]]; then
  rm -rf "${dest}/.git"
fi

echo "Fetched mlflow-oidc-auth source to ${dest}"
