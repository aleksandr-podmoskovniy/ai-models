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
Usage: run_hf_import_job.sh --hf-model-id <repo> --task <task> [options]

Render or apply a one-shot in-cluster Job that imports a Hugging Face snapshot
into ai-models / MLflow using the currently deployed backend image.

Examples:
  tools/run_hf_import_job.sh \
    --hf-model-id openai/gpt-oss-20b \
    --task text-generation \
    --registered-model-name openai-gpt-oss-20b

  tools/run_hf_import_job.sh \
    --hf-model-id openai/gpt-oss-20b \
    --task text-generation \
    --registered-model-name openai-gpt-oss-20b \
    --print-only

Options:
  --namespace <ns>                    Default: d8-ai-models
  --job-name <name>                   Default: ai-models-hf-import-<timestamp>
  --hf-model-id <repo>                Required
  --task <task>                       Required
  --workspace <name>                  Default: default
  --registered-model-name <name>      Default: sanitized hf-model-id
  --experiment-name <name>            Default: Default
  --artifact-name <name>              Default: model
  --revision <rev>                    Optional HF revision
  --hf-token-secret <name>            Optional Secret name in target namespace
  --hf-token-key <key>                Default: token
  --cpu-request <value>               Default: 250m
  --memory-request <value>            Default: 512Mi
  --memory-limit <value>              Default: 1Gi
  --ephemeral-storage-request <val>   Default: 80Gi
  --ephemeral-storage-limit <val>     Default: 120Gi
  --print-only                        Only print the manifest
EOF
}

sanitize_name() {
  printf '%s' "$1" | sed -E 's/[^A-Za-z0-9._-]+/--/g; s/^-+//; s/-+$//'
}

namespace="d8-ai-models"
job_name=""
hf_model_id=""
task=""
workspace="default"
registered_model_name=""
experiment_name="Default"
artifact_name="model"
revision=""
hf_token_secret=""
hf_token_key="token"
cpu_request="250m"
memory_request="512Mi"
memory_limit="1Gi"
ephemeral_storage_request="80Gi"
ephemeral_storage_limit="120Gi"
print_only="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace) namespace="$2"; shift 2 ;;
    --job-name) job_name="$2"; shift 2 ;;
    --hf-model-id) hf_model_id="$2"; shift 2 ;;
    --task) task="$2"; shift 2 ;;
    --workspace) workspace="$2"; shift 2 ;;
    --registered-model-name) registered_model_name="$2"; shift 2 ;;
    --experiment-name) experiment_name="$2"; shift 2 ;;
    --artifact-name) artifact_name="$2"; shift 2 ;;
    --revision) revision="$2"; shift 2 ;;
    --hf-token-secret) hf_token_secret="$2"; shift 2 ;;
    --hf-token-key) hf_token_key="$2"; shift 2 ;;
    --cpu-request) cpu_request="$2"; shift 2 ;;
    --memory-request) memory_request="$2"; shift 2 ;;
    --memory-limit) memory_limit="$2"; shift 2 ;;
    --ephemeral-storage-request) ephemeral_storage_request="$2"; shift 2 ;;
    --ephemeral-storage-limit) ephemeral_storage_limit="$2"; shift 2 ;;
    --print-only) print_only="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "${hf_model_id}" || -z "${task}" ]]; then
  echo "--hf-model-id and --task are required." >&2
  usage >&2
  exit 1
fi

if [[ -z "${registered_model_name}" ]]; then
  registered_model_name="$(sanitize_name "${hf_model_id}")"
fi

if [[ -z "${job_name}" ]]; then
  job_name="ai-models-hf-import-$(date +%s)"
fi

backend_image="$(kubectl -n "${namespace}" get deployment ai-models -o jsonpath='{.spec.template.spec.containers[?(@.name=="backend")].image}')"
if [[ -z "${backend_image}" ]]; then
  echo "Failed to detect deployed backend image in namespace ${namespace}." >&2
  exit 1
fi

readarray -t deployment_values < <(
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
'
)
s3_endpoint_url="${deployment_values[0]:-}"
s3_ignore_tls="${deployment_values[1]:-false}"
aws_region="${deployment_values[2]:-us-east-1}"
artifacts_secret_name="${deployment_values[3]:-ai-models-artifacts}"

image_pull_secret_block=""
if kubectl -n "${namespace}" get secret ai-models-module-registry >/dev/null 2>&1; then
  image_pull_secret_block="$(cat <<'EOF'
      imagePullSecrets:
        - name: ai-models-module-registry
EOF
)"
fi

hf_token_env_block=""
if [[ -n "${hf_token_secret}" ]]; then
  hf_token_env_block="$(cat <<EOF
            - name: HF_TOKEN
              valueFrom:
                secretKeyRef:
                  name: ${hf_token_secret}
                  key: ${hf_token_key}
EOF
)"
fi

manifest="$(cat <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job_name}
  namespace: ${namespace}
  labels:
    app.kubernetes.io/name: ai-models-hf-import
    app.kubernetes.io/part-of: ai-models
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 86400
  template:
    metadata:
      labels:
        app.kubernetes.io/name: ai-models-hf-import
        app.kubernetes.io/part-of: ai-models
    spec:
${image_pull_secret_block}
      serviceAccountName: ai-models
      automountServiceAccountToken: false
      restartPolicy: Never
      securityContext:
        runAsUser: 64535
        runAsGroup: 64535
        fsGroup: 64535
      containers:
        - name: hf-import
          image: ${backend_image}
          imagePullPolicy: IfNotPresent
          command: ["ai-models-backend-hf-import"]
          env:
            - name: AI_MODELS_IMPORT_TRACKING_URI
              value: http://ai-models.${namespace}.svc
            - name: AI_MODELS_IMPORT_EXPERIMENT_NAME
              value: ${experiment_name}
            - name: AI_MODELS_IMPORT_WORKSPACE
              value: ${workspace}
            - name: AI_MODELS_IMPORT_HF_MODEL_ID
              value: ${hf_model_id}
            - name: AI_MODELS_IMPORT_TASK
              value: ${task}
            - name: AI_MODELS_IMPORT_HF_REVISION
              value: ${revision}
            - name: AI_MODELS_IMPORT_REGISTERED_MODEL_NAME
              value: ${registered_model_name}
            - name: AI_MODELS_IMPORT_ARTIFACT_NAME
              value: ${artifact_name}
            - name: AI_MODELS_IMPORT_WORKDIR
              value: /work
            - name: AI_MODELS_IMPORT_SNAPSHOT_DIR
              value: /work/snapshot
            - name: AI_MODELS_S3_ENDPOINT_URL
              value: ${s3_endpoint_url}
            - name: AI_MODELS_S3_IGNORE_TLS
              value: ${s3_ignore_tls}
            - name: MLFLOW_TRACKING_USERNAME
              valueFrom:
                secretKeyRef:
                  name: ai-models-backend-auth
                  key: adminUsername
            - name: MLFLOW_TRACKING_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: ai-models-backend-auth
                  key: adminPassword
            - name: AWS_ACCESS_KEY_ID
              valueFrom:
                secretKeyRef:
                  name: ${artifacts_secret_name}
                  key: accessKey
            - name: AWS_SECRET_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: ${artifacts_secret_name}
                  key: secretKey
            - name: AWS_REGION
              value: ${aws_region}
            - name: AWS_DEFAULT_REGION
              value: ${aws_region}
            - name: AWS_EC2_METADATA_DISABLED
              value: "true"
            - name: AWS_CONFIG_FILE
              value: /etc/ai-models/aws/config
            - name: HOME
              value: /work
            - name: TMPDIR
              value: /work/tmp
            - name: HF_HOME
              value: /work/hf-home
            - name: HUGGINGFACE_HUB_CACHE
              value: /work/hf-cache
            - name: TRANSFORMERS_CACHE
              value: /work/transformers-cache
${hf_token_env_block}
          volumeMounts:
            - name: work
              mountPath: /work
            - name: runtime-config
              mountPath: /etc/ai-models/aws/config
              subPath: aws-config
          resources:
            requests:
              cpu: ${cpu_request}
              memory: ${memory_request}
              ephemeral-storage: ${ephemeral_storage_request}
            limits:
              memory: ${memory_limit}
              ephemeral-storage: ${ephemeral_storage_limit}
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            runAsNonRoot: true
            seccompProfile:
              type: RuntimeDefault
      volumes:
        - name: work
          emptyDir: {}
        - name: runtime-config
          configMap:
            name: ai-models-runtime
            defaultMode: 0755
EOF
)"

if [[ "${print_only}" == "true" ]]; then
  printf '%s\n' "${manifest}"
  exit 0
fi

printf '%s\n' "${manifest}" | kubectl apply -f -
printf 'Created job %s in namespace %s\n' "${job_name}" "${namespace}"
printf 'Watch logs with:\n'
printf '  kubectl -n %s logs -f job/%s\n' "${namespace}" "${job_name}"
