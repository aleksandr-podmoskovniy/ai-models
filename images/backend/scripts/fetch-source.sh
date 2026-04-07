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
Usage: fetch-source.sh [--source <upstream-checkout>] [--repo <git-url>] [--ref <git-ref>] [--dest <dir>] [--metadata <file>] [--keep-git]

Fetch pinned upstream backend engine source into a build-only directory.
If --source is passed, the local checkout is copied into <dir>.
Otherwise the script clones repository/ref from the metadata file.
EOF
}

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/../../.." && pwd)"

metadata_file="${repo_root}/images/backend/upstream.lock"
source_dir="${BACKEND_SOURCE_DIR:-}"
repo_url="${BACKEND_UPSTREAM_REPOSITORY:-}"
ref="${BACKEND_UPSTREAM_REF:-}"
dest_dir="${repo_root}/.cache/backend-upstream"
keep_git=0
skip_submodules=0
expected_version=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source)
      source_dir="$2"
      shift 2
      ;;
    --repo)
      repo_url="$2"
      shift 2
      ;;
    --ref)
      ref="$2"
      shift 2
      ;;
    --dest)
      dest_dir="$2"
      shift 2
      ;;
    --metadata)
      metadata_file="$2"
      shift 2
      ;;
    --keep-git)
      keep_git=1
      shift
      ;;
    --skip-submodules)
      skip_submodules=1
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

if [[ -f "${metadata_file}" ]]; then
  if [[ -z "${repo_url}" ]]; then
    repo_url="$(sed -n 's/^repository:[[:space:]]*//p' "${metadata_file}" | head -n1)"
  fi
  if [[ -z "${ref}" ]]; then
    ref="$(sed -n 's/^ref:[[:space:]]*//p' "${metadata_file}" | head -n1)"
  fi
  expected_version="$(sed -n 's/^version:[[:space:]]*//p' "${metadata_file}" | head -n1)"
fi

required_paths=()
if [[ -f "${metadata_file}" ]]; then
  while IFS= read -r path; do
    [[ -n "${path}" ]] || continue
    required_paths+=("${path}")
  done < <(awk '
    /^required_paths:/ { in_list=1; next }
    in_list && /^[^[:space:]-]/ { in_list=0 }
    in_list && $1 == "-" { print $2 }
  ' "${metadata_file}")
fi

if [[ -z "${source_dir}" ]]; then
  if [[ -z "${repo_url}" || -z "${ref}" ]]; then
    echo "Either --source or metadata with repository/ref is required." >&2
    exit 1
  fi
  if ! command -v git >/dev/null 2>&1; then
    echo "git is required to clone upstream backend sources." >&2
    exit 1
  fi
fi

copy_tree() {
  local from_dir="$1"
  local to_dir="$2"

  rm -rf "${to_dir}"
  mkdir -p "${to_dir}"

  tar -C "${from_dir}" \
    --exclude='.git' \
    --exclude='.DS_Store' \
    --exclude='.claude' \
    --exclude='.vscode' \
    --exclude='CLAUDE.md' \
    --exclude='docs' \
    --exclude='mlflow/server/js/build' \
    --exclude='node_modules' \
    --exclude='build' \
    --exclude='dist' \
    --exclude='__pycache__' \
    --exclude='.pytest_cache' \
    --exclude='.ruff_cache' \
    --exclude='.mypy_cache' \
    --exclude='.cache' \
    -cf - . | tar -C "${to_dir}" -xf -
}

resolve_source_root() {
  local candidate="$1"

  if [[ -f "${candidate}/pyproject.release.toml" ]]; then
    printf '%s\n' "${candidate}"
    return 0
  fi

  if [[ -f "${candidate}/mlflow/pyproject.release.toml" ]]; then
    printf '%s\n' "${candidate}/mlflow"
    return 0
  fi

  printf '%s\n' "${candidate}"
}

ensure_required_paths() {
  local root="$1"
  local required_path

  for required_path in "${required_paths[@]}"; do
    if [[ ! -e "${root}/${required_path}" ]]; then
      echo "Required upstream path is missing: ${root}/${required_path}" >&2
      exit 1
    fi
  done
}

rewrite_repo_url() {
  local original_url="$1"
  local mirror="${SOURCE_REPO_GIT:-${SOURCE_REPO:-}}"

  if [[ -z "${mirror}" ]]; then
    printf '%s\n' "${original_url}"
    return 0
  fi

  case "${original_url}" in
    https://github.com/*)
      printf '%s/%s\n' "${mirror%/}" "${original_url#https://github.com/}"
      ;;
    https://gitlab.com/*)
      printf '%s/%s\n' "${mirror%/}" "${original_url#https://gitlab.com/}"
      ;;
    *)
      printf '%s\n' "${original_url}"
      ;;
  esac
}

if [[ -n "${source_dir}" ]]; then
  if [[ ! -d "${source_dir}" ]]; then
    echo "Source directory does not exist: ${source_dir}" >&2
    exit 1
  fi

  source_dir="$(resolve_source_root "${source_dir}")"

  if [[ ${skip_submodules} -eq 0 && -f "${source_dir}/.gitmodules" ]]; then
    if ! command -v git >/dev/null 2>&1; then
      echo "git is required to initialize upstream submodules from a local checkout." >&2
      exit 1
    fi
    if git -C "${source_dir}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      git -C "${source_dir}" submodule update --init --recursive
    fi
  fi

  copy_tree "${source_dir}" "${dest_dir}"
else
  repo_url="$(rewrite_repo_url "${repo_url}")"
  rm -rf "${dest_dir}"
  git clone --filter=blob:none "${repo_url}" "${dest_dir}"
  git -C "${dest_dir}" checkout "${ref}"
  if [[ ${skip_submodules} -eq 0 && -f "${dest_dir}/.gitmodules" ]]; then
    git -C "${dest_dir}" submodule update --init --recursive
  fi
fi

ensure_required_paths "${dest_dir}"

if [[ -n "${expected_version}" ]]; then
  actual_version="$(awk -F'"' '/^version = / { print $2; exit }' "${dest_dir}/pyproject.release.toml")"
  if [[ -z "${actual_version}" ]]; then
    echo "Failed to parse version from ${dest_dir}/pyproject.release.toml" >&2
    exit 1
  fi
  if [[ "${actual_version}" != "${expected_version}" ]]; then
    echo "Fetched upstream version ${actual_version} does not match locked version ${expected_version}" >&2
    exit 1
  fi
fi

if [[ ${keep_git} -eq 0 ]]; then
  find "${dest_dir}" -name .git -prune -exec rm -rf {} +
fi

echo "Prepared backend source in ${dest_dir}"
