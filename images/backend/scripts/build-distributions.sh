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
Usage: build-distributions.sh [<source-root> [<out-dir>]]

Build upstream-equivalent backend distributions from imported source without relying on git metadata.
By default it builds release, skinny, and tracing distributions into <source-root>/dist.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

source_root="${1:-/src/backend}"
out_dir="${2:-${source_root}/dist}"
build_types="${BACKEND_BUILD_TYPES:-release,skinny,tracing}"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to build backend distributions." >&2
  exit 1
fi

if [[ ! -d "${source_root}" ]]; then
  echo "Backend source directory does not exist: ${source_root}" >&2
  exit 1
fi

if ! python3 -m build --help >/dev/null 2>&1; then
  echo "python3 -m build is unavailable. Install the build package first." >&2
  exit 1
fi

ui_build_dir="${source_root}/mlflow/server/js/build"
if [[ ! -d "${ui_build_dir}" ]] || [[ -z "$(find "${ui_build_dir}" -mindepth 1 -print -quit 2>/dev/null || true)" ]]; then
  echo "Backend release build requires prebuilt UI assets at ${ui_build_dir}" >&2
  exit 1
fi

cleanup_build_artifacts() {
  rm -rf "${out_dir}"
  rm -rf "${source_root}/build" "${source_root}/dist"
  find "${source_root}" -type d -name '*.egg-info' -prune -exec rm -rf {} +
  find "${source_root}/libs" -maxdepth 2 -type d \( -name build -o -name dist \) -prune -exec rm -rf {} +
  mkdir -p "${out_dir}"
}

validate_wheel_assets() {
  local wheel_path="$1"
  local expect_ui_assets="$2"

  python3 - "$wheel_path" "$expect_ui_assets" <<'PY'
import sys
import zipfile

wheel_path, expect_ui_assets = sys.argv[1], sys.argv[2] == "true"
prefix = "mlflow/server/js/build/"

with zipfile.ZipFile(wheel_path) as zf:
    has_ui = any(name.startswith(prefix) for name in zf.namelist())

if expect_ui_assets and not has_ui:
    raise SystemExit(f"UI assets are missing from {wheel_path}")
if not expect_ui_assets and has_ui:
    raise SystemExit(f"Unexpected UI assets found in {wheel_path}")
PY
}

build_release() {
  local backup

  backup="$(mktemp "${source_root}/pyproject.toml.XXXXXX")"
  cp "${source_root}/pyproject.toml" "${backup}"
  trap 'mv -f "'"${backup}"'" "'"${source_root}/pyproject.toml"'"' RETURN

  cp "${source_root}/pyproject.release.toml" "${source_root}/pyproject.toml"
  python3 -m build "${source_root}" --outdir "${out_dir}"

  local wheel_path
  wheel_path="$(find "${out_dir}" -maxdepth 1 -type f -name 'mlflow-*.whl' | sort | tail -n1)"
  if [[ -z "${wheel_path}" ]]; then
    echo "Failed to find built release wheel in ${out_dir}" >&2
    exit 1
  fi

  validate_wheel_assets "${wheel_path}" true
  trap - RETURN
  mv -f "${backup}" "${source_root}/pyproject.toml"
}

build_subpackage() {
  local package_dir="$1"
  local wheel_glob="$2"

  python3 -m build "${package_dir}" --outdir "${out_dir}"

  local wheel_path
  wheel_path="$(find "${out_dir}" -maxdepth 1 -type f -name "${wheel_glob}" | sort | tail -n1)"
  if [[ -z "${wheel_path}" ]]; then
    echo "Failed to find built wheel ${wheel_glob} in ${out_dir}" >&2
    exit 1
  fi

  validate_wheel_assets "${wheel_path}" false
}

cleanup_build_artifacts

IFS=',' read -r -a types <<< "${build_types}"
for raw_type in "${types[@]}"; do
  type="$(echo "${raw_type}" | xargs)"
  case "${type}" in
    release)
      build_release
      ;;
    skinny)
      build_subpackage "${source_root}/libs/skinny" 'mlflow_skinny-*.whl'
      ;;
    tracing)
      build_subpackage "${source_root}/libs/tracing" 'mlflow_tracing-*.whl'
      ;;
    "")
      ;;
    *)
      echo "Unsupported backend build type: ${type}" >&2
      exit 1
      ;;
  esac
done
