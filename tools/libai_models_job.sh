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

yaml_quote() {
  python3 - "$1" <<'PY'
import json
import sys

print(json.dumps(sys.argv[1]))
PY
}


load_deployed_backend_runtime() {
  local namespace="$1"

  backend_image="$(kubectl -n "${namespace}" get deployment ai-models -o jsonpath='{.spec.template.spec.containers[?(@.name=="backend")].image}')"
  if [[ -z "${backend_image}" ]]; then
    echo "Failed to detect deployed backend image in namespace ${namespace}." >&2
    return 1
  fi

  local deployment_values=()
  while IFS= read -r value; do
    deployment_values+=("${value}")
  done < <(
    kubectl -n "${namespace}" get deployment ai-models -o json | python3 -c '
import json, sys
doc = json.load(sys.stdin)
container = next(
    c for c in doc["spec"]["template"]["spec"]["containers"] if c["name"] == "backend"
)
env = {entry["name"]: entry for entry in container.get("env", [])}
for key in ("AI_MODELS_S3_ENDPOINT_URL", "AI_MODELS_S3_IGNORE_TLS", "AWS_REGION"):
    print(env.get(key, {}).get("value", ""))
secret_ref = env.get("AWS_ACCESS_KEY_ID", {}).get("valueFrom", {}).get("secretKeyRef", {})
print(secret_ref.get("name", "ai-models-artifacts"))
volumes = {volume.get("name", ""): volume for volume in doc["spec"]["template"]["spec"].get("volumes", [])}
artifacts_ca_secret_name = ""
for mount in container.get("volumeMounts", []):
    if mount.get("mountPath") not in ("/etc/ai-models/artifacts-ca", "/etc/ai-models/platform-ca"):
        continue
    volume = volumes.get(mount.get("name", ""), {})
    artifacts_ca_secret_name = volume.get("secret", {}).get("secretName", "")
    if artifacts_ca_secret_name:
        break
print(artifacts_ca_secret_name)
'
  )

  s3_endpoint_url="${deployment_values[0]:-}"
  s3_ignore_tls="${deployment_values[1]:-false}"
  aws_region="${deployment_values[2]:-us-east-1}"
  artifacts_secret_name="${deployment_values[3]:-ai-models-artifacts}"
  artifacts_ca_secret_name="${deployment_values[4]:-}"

  image_pull_secret_block=""
  if kubectl -n "${namespace}" get secret ai-models-module-registry >/dev/null 2>&1; then
    image_pull_secret_block="$(cat <<'EOF'
      imagePullSecrets:
        - name: ai-models-module-registry
EOF
)"
  fi
}
