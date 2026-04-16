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
VERSION=${DMT_VERSION:-0.1.69}
REPO="deckhouse/dmt"

case "$(uname -s)" in
  Linux) OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)
    echo "Unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

ASSET="dmt-${VERSION}-${OS}-${ARCH}.tar.gz"
CHECKSUM_FILE="dmt-${VERSION}-checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"

mkdir -p "${INSTALL_DIR}"
if [[ -x "${INSTALL_DIR}/dmt" ]]; then
  current_version="$("${INSTALL_DIR}/dmt" --version 2>/dev/null | awk '{print $3}')"
  if [[ "${current_version}" == "v${VERSION}" ]]; then
    exit 0
  fi
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

ARCHIVE_PATH="${TMP_DIR}/${ASSET}"
CHECKSUM_PATH="${TMP_DIR}/${CHECKSUM_FILE}"

curl -fsSL "${BASE_URL}/${ASSET}" -o "${ARCHIVE_PATH}"
curl -fsSL "${BASE_URL}/${CHECKSUM_FILE}" -o "${CHECKSUM_PATH}"

EXPECTED_SUM=$(grep " ${ASSET}$" "${CHECKSUM_PATH}" | awk '{print $1}')
if [[ -z "${EXPECTED_SUM}" ]]; then
  echo "Checksum for ${ASSET} not found in ${CHECKSUM_FILE}" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_SUM=$(sha256sum "${ARCHIVE_PATH}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL_SUM=$(shasum -a 256 "${ARCHIVE_PATH}" | awk '{print $1}')
else
  echo "sha256sum or shasum is required to verify checksums" >&2
  exit 1
fi

if [[ "${EXPECTED_SUM}" != "${ACTUAL_SUM}" ]]; then
  echo "Checksum mismatch for ${ASSET}" >&2
  exit 1
fi

tar -xf "${ARCHIVE_PATH}" -C "${TMP_DIR}"

DMT_BIN=$(find "${TMP_DIR}" -type f -name dmt -perm -u+x | head -n 1)
if [[ -z "${DMT_BIN}" ]]; then
  echo "dmt executable not found inside archive" >&2
  exit 1
fi

install -m 0755 "${DMT_BIN}" "${INSTALL_DIR}/dmt"
