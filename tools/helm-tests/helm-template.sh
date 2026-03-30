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

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
VALUES_BASE="${ROOT_DIR}/fixtures/module-values.yaml"
SCENARIOS_DIR="${ROOT_DIR}/fixtures/render"
RENDERS_DIR="${ROOT_DIR}/tools/kubeconform/renders"
HELM_BIN=""
HELM_TMPDIR=""

cleanup() {
  if [[ -n "${HELM_TMPDIR}" && -d "${HELM_TMPDIR}" ]]; then
    rm -rf "${HELM_TMPDIR}"
  fi
}

trap cleanup EXIT

ensure_helm() {
  local min_version="3.14.0"
  local desired_version="${HELM_DESIRED_VERSION:-3.20.1}"

  version_ge() {
    local left="${1:-}"
    local right="${2:-}"
    [[ "$(printf '%s\n' "$right" "$left" | sort -V | head -n1)" == "$right" ]]
  }

  try_helm_bin() {
    local candidate="${1:-}"
    local current=""
    if [[ -z "${candidate}" || ! -x "${candidate}" ]]; then
      return 1
    fi
    current="$("${candidate}" version --template '{{.Version}}' 2>/dev/null | sed 's/^v//')"
    if [[ -n "${current}" ]] && version_ge "${current}" "${min_version}"; then
      HELM_BIN="${candidate}"
      return 0
    fi
    return 1
  }

  if command -v helm >/dev/null 2>&1 && try_helm_bin "$(command -v helm)"; then
    return 0
  fi

  if try_helm_bin /opt/homebrew/bin/helm; then
    return 0
  fi

  if try_helm_bin /usr/local/bin/helm; then
    return 0
  fi

  local os arch
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
      echo "unsupported arch $(uname -m)" >&2
      exit 1
      ;;
  esac

  local cache_dir="${ROOT_DIR}/.cache/helm/v${desired_version}/${os}-${arch}"
  local cached_bin="${cache_dir}/helm"
  if [[ -x "${cached_bin}" ]]; then
    HELM_BIN="${cached_bin}"
    return 0
  fi

  local cached_root="${ROOT_DIR}/.cache/helm"
  if [[ -d "${cached_root}" ]]; then
    local candidate
    while IFS= read -r candidate; do
      if try_helm_bin "${candidate}"; then
        return 0
      fi
    done < <(find "${cached_root}" -type f -path "*/${os}-${arch}/helm" | sort -V -r)
  fi

  HELM_TMPDIR=$(mktemp -d "${TMPDIR:-/tmp}/helm.XXXXXX")
  curl -fsSL "https://get.helm.sh/helm-v${desired_version}-${os}-${arch}.tar.gz" -o "${HELM_TMPDIR}/helm.tar.gz"
  tar -xzf "${HELM_TMPDIR}/helm.tar.gz" -C "${HELM_TMPDIR}"
  mkdir -p "${cache_dir}"
  cp "${HELM_TMPDIR}/${os}-${arch}/helm" "${cached_bin}"
  chmod +x "${cached_bin}"
  HELM_BIN="${cached_bin}"
}

ensure_helm
mkdir -p "${RENDERS_DIR}"
rm -f "${RENDERS_DIR}"/helm-template-*.yaml

render_scenario() {
  local scenario_name="${1:?scenario name is required}"
  shift

  "${HELM_BIN}" template ai-models "${ROOT_DIR}" \
    -f "${VALUES_BASE}" \
    "$@" \
    --set-string global.enabledModules[0]=ai-models \
    --set-string global.enabledModules[1]=cert-manager \
    --set-string global.enabledModules[2]=managed-postgres \
    --set global.deckhouseVersion="dev" \
    --set global.clusterConfiguration.clusterDomain="cluster.local" \
    --set global.discovery.clusterDomain="cluster.local" \
    --set global.internal.modules.ai-models=true \
    --api-versions managed-services.deckhouse.io/v1alpha1/Postgres \
    --api-versions managed-services.deckhouse.io/v1alpha1/PostgresClass \
    --api-versions cert-manager.io/v1/Certificate \
    --api-versions monitoring.coreos.com/v1/ServiceMonitor \
    --namespace d8-ai-models \
    > "${RENDERS_DIR}/helm-template-${scenario_name}.yaml"

  echo "rendered: ${RENDERS_DIR}/helm-template-${scenario_name}.yaml"
}

if [[ ! -d "${SCENARIOS_DIR}" ]]; then
  echo "scenario fixtures directory not found: ${SCENARIOS_DIR}" >&2
  exit 1
fi

scenario_count=0
while IFS= read -r scenario_file; do
  [[ -n "${scenario_file}" ]] || continue
  scenario_name="$(basename "${scenario_file}" .yaml)"
  render_scenario "${scenario_name}" -f "${scenario_file}"
  scenario_count=$((scenario_count + 1))
done < <(find "${SCENARIOS_DIR}" -maxdepth 1 -type f -name '*.yaml' | sort)

if [[ "${scenario_count}" -eq 0 ]]; then
  echo "no scenario fixtures found in ${SCENARIOS_DIR}" >&2
  exit 1
fi

python3 "${ROOT_DIR}/tools/helm-tests/validate-renders.py" "${RENDERS_DIR}"
