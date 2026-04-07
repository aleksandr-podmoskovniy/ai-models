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
Usage: install-kitops.sh [--metadata <file>] [--dest <path>]

Install the pinned KitOps CLI binary into the controller runtime image.
Optional environment overrides:
  KITOPS_SOURCE_ARCHIVE       use a pre-fetched archive instead of downloading it
  KITOPS_LINUX_AMD64_URL      override amd64 archive URL from the metadata file
  KITOPS_LINUX_AMD64_SHA256   override amd64 archive checksum from the metadata file
  KITOPS_LINUX_ARM64_URL      override arm64 archive URL from the metadata file
  KITOPS_LINUX_ARM64_SHA256   override arm64 archive checksum from the metadata file
EOF
}

controller_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
metadata_file="${controller_dir}/kitops.lock"
dest="/usr/local/bin/kit"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --metadata)
      metadata_file="${2:?missing value for --metadata}"
      shift 2
      ;;
    --dest)
      dest="${2:?missing value for --dest}"
      shift 2
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

if [[ ! -f "${metadata_file}" ]]; then
  echo "KitOps metadata file not found: ${metadata_file}" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "${metadata_file}"

arch="$(uname -m)"
case "${arch}" in
  x86_64|amd64)
    url="${KITOPS_LINUX_AMD64_URL:-${LINUX_AMD64_URL}}"
    sha256="${KITOPS_LINUX_AMD64_SHA256:-${LINUX_AMD64_SHA256}}"
    ;;
  aarch64|arm64)
    url="${KITOPS_LINUX_ARM64_URL:-${LINUX_ARM64_URL}}"
    sha256="${KITOPS_LINUX_ARM64_SHA256:-${LINUX_ARM64_SHA256}}"
    ;;
  *)
    echo "Unsupported architecture for KitOps install: ${arch}" >&2
    exit 1
    ;;
esac

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

archive="${tmpdir}/kitops.tar.gz"
if [[ -n "${KITOPS_SOURCE_ARCHIVE:-}" ]]; then
  cp "${KITOPS_SOURCE_ARCHIVE}" "${archive}"
else
  curl -fsSLo "${archive}" "${url}"
fi
echo "${sha256}  ${archive}" | sha256sum -c -
tar -xzf "${archive}" -C "${tmpdir}"

if [[ ! -f "${tmpdir}/kit" ]]; then
  echo "KitOps archive does not contain the expected 'kit' binary." >&2
  exit 1
fi

mkdir -p "$(dirname -- "${dest}")"
install -m 0755 "${tmpdir}/kit" "${dest}"
