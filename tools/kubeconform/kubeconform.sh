#!/usr/bin/env bash

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

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RENDERS_DIR="${ROOT_DIR}/tools/kubeconform/renders"
REPORT_FILE="${ROOT_DIR}/tools/kubeconform/kubeconform-report.json"
HELM_RENDER="${ROOT_DIR}/tools/kubeconform/helm-template-render.yaml"
KUBECONFORM_VERSION="${KUBECONFORM_VERSION:-0.7.0}"
KUBECONFORM_BIN=""

ensure_kubeconform() {
  if command -v kubeconform >/dev/null 2>&1; then
    KUBECONFORM_BIN="$(command -v kubeconform)"
    return 0
  fi

  local os arch cache_dir archive_name url tmp_dir
  case "$(uname -s)" in
    Linux) os="linux" ;;
    Darwin) os="darwin" ;;
    *)
      echo "unsupported OS $(uname -s)" >&2
      exit 1
      ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)
      echo "unsupported architecture $(uname -m)" >&2
      exit 1
      ;;
  esac

  cache_dir="${ROOT_DIR}/.cache/kubeconform/v${KUBECONFORM_VERSION}/${os}-${arch}"
  KUBECONFORM_BIN="${cache_dir}/kubeconform"
  if [[ -x "${KUBECONFORM_BIN}" ]]; then
    return 0
  fi

  archive_name="kubeconform-${os}-${arch}.tar.gz"
  url="https://github.com/yannh/kubeconform/releases/download/v${KUBECONFORM_VERSION}/${archive_name}"
  tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/kubeconform.XXXXXX")"
  trap 'rm -rf "${tmp_dir}"' RETURN
  mkdir -p "${cache_dir}"
  curl -fsSL "${url}" -o "${tmp_dir}/kubeconform.tgz"
  tar -xzf "${tmp_dir}/kubeconform.tgz" -C "${tmp_dir}"
  install -m 0755 "${tmp_dir}/kubeconform" "${KUBECONFORM_BIN}"
}

if [[ ! -d "${RENDERS_DIR}" ]]; then
  echo "renders not found. Run 'make helm-template' first." >&2
  exit 1
fi

render_files=()
while IFS= read -r render_file; do
  [[ -n "${render_file}" ]] && render_files+=("${render_file}")
done < <(ls "${RENDERS_DIR}"/helm-template-*.yaml 2>/dev/null | sort)

if [[ ${#render_files[@]} -eq 0 ]]; then
  echo "no renders found in ${RENDERS_DIR}" >&2
  exit 1
fi

cat "${render_files[@]}" > "${HELM_RENDER}"

ensure_kubeconform

cat "${HELM_RENDER}" | "${KUBECONFORM_BIN}" -verbose -strict \
  -kubernetes-version 1.30.0 \
  -schema-location default \
  -skip Postgres,PostgresClass,Certificate,ServiceMonitor \
  -output json - > "${REPORT_FILE}"

cat "${REPORT_FILE}" | python3 -c 'import json,sys; json.load(sys.stdin); print("kubeconform report is valid JSON")'

exit_code=$(python3 - <<'PY'
import json
from pathlib import Path
report = json.loads(Path("tools/kubeconform/kubeconform-report.json").read_text())
resources = report.get("resources", [])
errors = [r for r in resources if r.get("status") in {"statusError", "statusInvalid"}]
print(len(errors))
PY
)

exit "${exit_code}"
