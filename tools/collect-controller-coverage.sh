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

ROOT=${ROOT:-$(pwd)}
MODULE_DIR="${ROOT}/images/controller"
COVERAGE_DIR=${COVERAGE_DIR:-"${ROOT}/artifacts/coverage"}
GOFLAGS=${GOFLAGS:--count=1}

mkdir -p "${COVERAGE_DIR}"
rm -f "${COVERAGE_DIR}"/controller-*.out

packages=()
while IFS= read -r pkg; do
  packages+=("${pkg}")
done < <(
  cd "${MODULE_DIR}" && find ./internal -type d \( -path '*/domain/*' -o -path '*/application/*' \) | sort
)

if [[ ${#packages[@]} -eq 0 ]]; then
  echo "==> no domain/application controller packages for coverage artifacts"
  exit 0
fi

echo "==> controller coverage artifacts"
for pkg in "${packages[@]}"; do
  if ! find "${MODULE_DIR}/${pkg#./}" -maxdepth 1 -type f -name '*.go' ! -name '*_test.go' | grep -q .; then
    continue
  fi
  filename="controller-$(sed 's#^\./internal/##; s#/#-#g' <<<"${pkg}").out"
  echo "==> go test -coverprofile ${filename} (${pkg})"
  (
    cd "${MODULE_DIR}" &&
      go test ${GOFLAGS} -coverprofile "${COVERAGE_DIR}/${filename}" "${pkg}"
  )
done
