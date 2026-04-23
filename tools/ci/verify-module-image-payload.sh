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

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_cmd() {
  local cmd="$1"

  command -v "$cmd" >/dev/null 2>&1 || fail "required command is not available: $cmd"
}

module_image_repository() {
  local image_ref="$1"
  local without_digest prefix name_with_tag image_name

  without_digest="${image_ref%%@*}"
  prefix="${without_digest%/*}"
  name_with_tag="${without_digest##*/}"
  image_name="${name_with_tag%%:*}"

  [[ "$prefix" != "$without_digest" ]] || fail "cannot derive image repository from '$image_ref'"
  [[ -n "$image_name" ]] || fail "cannot derive image name from '$image_ref'"

  printf '%s/%s\n' "$prefix" "$image_name"
}

require_image_file_string() {
  local image_ref="$1"
  local path="$2"
  local needle="$3"

  if ! crane export "$image_ref" - | tar -xOf - "$path" 2>/dev/null | strings | grep -F -- "$needle" >/dev/null; then
    fail "image '$image_ref' file '$path' does not contain required marker: $needle"
  fi
}

require_json_key_absent() {
  local file="$1"
  local key="$2"

  if jq -e --arg key "$key" 'has($key)' "$file" >/dev/null; then
    fail "images_digests.json contains retired image key: $key"
  fi
}

main() {
  local module_ref="${1:-}"
  local tmp_dir images_digests module_repo dmcr_digest dmcr_ref retired_path

  [[ -n "$module_ref" ]] || fail "usage: $0 <module-image-ref>"

  require_cmd crane
  require_cmd grep
  require_cmd jq
  require_cmd mktemp
  require_cmd strings
  require_cmd tar

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  crane export "$module_ref" - | tar -x -C "$tmp_dir"

  images_digests="$tmp_dir/images_digests.json"
  [[ -f "$images_digests" ]] || fail "module image '$module_ref' does not contain images_digests.json"

  for retired_path in templates/backend templates/database templates/auth; do
    [[ ! -e "$tmp_dir/$retired_path" ]] || fail "module image '$module_ref' contains retired path: $retired_path"
  done

  require_json_key_absent "$images_digests" backend

  dmcr_digest="$(jq -r '.dmcr // empty' "$images_digests")"
  [[ "$dmcr_digest" == sha256:* ]] || fail "images_digests.json does not contain valid dmcr digest"

  module_repo="${MODULE_IMAGE_REPO:-$(module_image_repository "$module_ref")}"
  dmcr_ref="$module_repo@$dmcr_digest"

  require_image_file_string "$dmcr_ref" usr/bin/dmcr "github.com/deckhouse/ai-models/dmcr/internal/registrydriver/sealeds3"
  require_image_file_string "$dmcr_ref" usr/local/bin/dmcr-cleaner "Cron schedule used to enqueue periodic stale-sweep requests"
  require_image_file_string "$dmcr_ref" usr/local/bin/dmcr-direct-upload "failed to open uploaded object for verification"

  echo "verified module image payload: module=$module_ref dmcr=$dmcr_ref"
}

main "$@"
