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

INSTALL_DIR=${INSTALL_DIR:-$(pwd)/.bin}
VERSION=${GOCYCLO_VERSION:-0.6.0}
BINARY="${INSTALL_DIR}/gocyclo"

mkdir -p "${INSTALL_DIR}"

if [[ -x "${BINARY}" ]]; then
  current="$("${BINARY}" -h 2>&1 || true)"
  if grep -Fq "gocyclo" <<<"${current}"; then
    exit 0
  fi
fi

GOBIN="${INSTALL_DIR}" go install "github.com/fzipp/gocyclo/cmd/gocyclo@v${VERSION}"
