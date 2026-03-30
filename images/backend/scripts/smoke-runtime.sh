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
Usage: smoke-runtime.sh

Run a minimal runtime smoke suite against the installed backend CLI.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for runtime smoke tests." >&2
  exit 1
fi

backend_cli="$(command -v ai-models-backend || command -v mlflow)"
python3 -c "import mlflow; print(mlflow.__version__)"
python3 -c "from mlflow import *"
python3 -c "import mlflow.server.auth; print('basic-auth ok')"
python3 -c "import transformers, huggingface_hub; print(transformers.__version__)"
"${backend_cli}" server --help >/dev/null
ai-models-backend-render-db-uri >/dev/null 2>&1 || true
ai-models-backend-render-auth-config --help >/dev/null 2>&1 || true
ai-models-backend-hf-import --help >/dev/null
