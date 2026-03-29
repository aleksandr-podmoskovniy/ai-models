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
Usage: smoke-release-install.sh [<dist-dir>]

Install locally built backend distributions and run import / CLI smoke checks.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

dist_dir="${1:-/dist}"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for backend wheel smoke tests." >&2
  exit 1
fi

wheel_release="$(find "${dist_dir}" -maxdepth 1 -type f -name 'mlflow-*.whl' | sort | tail -n1)"
wheel_skinny="$(find "${dist_dir}" -maxdepth 1 -type f -name 'mlflow_skinny-*.whl' | sort | tail -n1)"
wheel_tracing="$(find "${dist_dir}" -maxdepth 1 -type f -name 'mlflow_tracing-*.whl' | sort | tail -n1)"

if [[ -z "${wheel_release}" || -z "${wheel_skinny}" || -z "${wheel_tracing}" ]]; then
  echo "Expected release, skinny, and tracing wheels in ${dist_dir}" >&2
  exit 1
fi

python3 -m pip install --no-cache-dir --force-reinstall "${wheel_skinny}" "${wheel_tracing}" "${wheel_release}"
backend_cli="$(command -v ai-models-backend || command -v mlflow)"
python3 -c "import mlflow; print(mlflow.__version__)"
python3 -c "from mlflow import *"
"${backend_cli}" server --help >/dev/null
