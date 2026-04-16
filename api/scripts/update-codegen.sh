#!/bin/bash

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

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd -P)"
API_ROOT="${SCRIPT_DIR}/.."
ROOT="${API_ROOT}/.."
CONTROLLER_GEN_VERSION="v0.18.0"

cd "${API_ROOT}"

go run "sigs.k8s.io/controller-tools/cmd/controller-gen@${CONTROLLER_GEN_VERSION}" \
  "object:headerFile=${SCRIPT_DIR}/boilerplate.go.txt,year=2026" \
  paths="./core/..."

OUTPUT_DIR="$(mktemp -d)"
trap 'rm -rf "${OUTPUT_DIR}"' EXIT

go run "sigs.k8s.io/controller-tools/cmd/controller-gen@${CONTROLLER_GEN_VERSION}" \
  "crd:crdVersions=v1" \
  paths="./core/..." \
  "output:crd:dir=${OUTPUT_DIR}"

mkdir -p "${ROOT}/crds"
cp "${OUTPUT_DIR}/ai.deckhouse.io_models.yaml" "${ROOT}/crds/"
cp "${OUTPUT_DIR}/ai.deckhouse.io_clustermodels.yaml" "${ROOT}/crds/"
