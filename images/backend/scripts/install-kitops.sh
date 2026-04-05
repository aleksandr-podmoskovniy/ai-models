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

Install the pinned KitOps CLI binary into the backend runtime image.
EOF
}

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd -- "${script_dir}/.." && pwd)"
metadata_file="${backend_dir}/kitops.lock"
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
    url="${LINUX_AMD64_URL}"
    sha256="${LINUX_AMD64_SHA256}"
    ;;
  aarch64|arm64)
    url="${LINUX_ARM64_URL}"
    sha256="${LINUX_ARM64_SHA256}"
    ;;
  *)
    echo "Unsupported architecture for KitOps install: ${arch}" >&2
    exit 1
    ;;
esac

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

archive="${tmpdir}/kitops.tar.gz"
curl -fsSLo "${archive}" "${url}"
echo "${sha256}  ${archive}" | sha256sum -c -
tar -xzf "${archive}" -C "${tmpdir}"

mkdir -p "$(dirname -- "${dest}")"
install -m 0755 "${tmpdir}/kit" "${dest}"
