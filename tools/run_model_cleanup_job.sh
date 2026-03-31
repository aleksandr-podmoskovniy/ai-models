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

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=tools/libai_models_job.sh
source "${script_dir}/libai_models_job.sh"

usage() {
  cat <<'EOF'
Usage: run_model_cleanup_job.sh --registered-model-name <name> --version <version> [options]

Render or apply a one-shot in-cluster Job that deletes an MLflow model version
together with the linked logged model, source run, and S3 artifact prefixes
using the currently deployed backend image.

Examples:
  tools/run_model_cleanup_job.sh \
    --registered-model-name google-gemma-3-4b-it \
    --version 1

  tools/run_model_cleanup_job.sh \
    --registered-model-name google-gemma-3-4b-it \
    --version 1 \
    --print-only

Options:
  --namespace <ns>                    Default: d8-ai-models
  --job-name <name>                   Default: ai-models-model-cleanup-<timestamp>
  --workspace <name>                  Default: default
  --registered-model-name <name>      Required
  --version <version>                 Required
  --print-only                        Only print the manifest
  --dry-run                           Resolve cleanup target only; do not delete anything
EOF
}

namespace="d8-ai-models"
job_name=""
workspace="default"
registered_model_name=""
version=""
print_only="false"
dry_run="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace) namespace="$2"; shift 2 ;;
    --job-name) job_name="$2"; shift 2 ;;
    --workspace) workspace="$2"; shift 2 ;;
    --registered-model-name) registered_model_name="$2"; shift 2 ;;
    --version) version="$2"; shift 2 ;;
    --print-only) print_only="true"; shift ;;
    --dry-run) dry_run="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "${registered_model_name}" || -z "${version}" ]]; then
  echo "--registered-model-name and --version are required." >&2
  usage >&2
  exit 1
fi

if [[ -z "${job_name}" ]]; then
  job_name="ai-models-model-cleanup-$(date +%s)"
fi

load_deployed_backend_runtime "${namespace}"

tracking_uri_yaml="$(yaml_quote "http://ai-models.${namespace}.svc")"
workspace_yaml="$(yaml_quote "${workspace}")"
registered_model_name_yaml="$(yaml_quote "${registered_model_name}")"
version_yaml="$(yaml_quote "${version}")"
s3_endpoint_url_yaml="$(yaml_quote "${s3_endpoint_url}")"
s3_ignore_tls_yaml="$(yaml_quote "${s3_ignore_tls}")"
aws_region_yaml="$(yaml_quote "${aws_region}")"
aws_config_file_yaml="$(yaml_quote "/etc/ai-models/aws/config")"
home_yaml="$(yaml_quote "/work")"
tmpdir_yaml="$(yaml_quote "/work/tmp")"
s3_ca_file_yaml="$(yaml_quote "/etc/ai-models/artifacts-ca/ca.crt")"

artifacts_ca_env_block=""
artifacts_ca_volume_mount_block=""
artifacts_ca_volume_block=""
if [[ -n "${artifacts_ca_secret_name}" ]]; then
  artifacts_ca_env_block="$(cat <<EOF
            - name: AI_MODELS_S3_CA_FILE
              value: ${s3_ca_file_yaml}
EOF
)"
  artifacts_ca_volume_mount_block="$(cat <<'EOF'
            - name: artifacts-ca
              mountPath: /etc/ai-models/artifacts-ca
              readOnly: true
EOF
)"
  artifacts_ca_volume_block="$(cat <<EOF
        - name: artifacts-ca
          secret:
            secretName: ${artifacts_ca_secret_name}
EOF
)"
fi

dry_run_env_block=""
if [[ "${dry_run}" == "true" ]]; then
  dry_run_env_block="$(cat <<'EOF'
            - name: AI_MODELS_CLEANUP_DRY_RUN
              value: "true"
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
    app.kubernetes.io/name: ai-models-model-cleanup
    app.kubernetes.io/part-of: ai-models
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 86400
  template:
    metadata:
      labels:
        app.kubernetes.io/name: ai-models-model-cleanup
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
        - name: model-cleanup
          image: ${backend_image}
          imagePullPolicy: IfNotPresent
          command: ["ai-models-backend-model-cleanup"]
          env:
            - name: AI_MODELS_CLEANUP_TRACKING_URI
              value: ${tracking_uri_yaml}
            - name: AI_MODELS_CLEANUP_WORKSPACE
              value: ${workspace_yaml}
            - name: AI_MODELS_CLEANUP_REGISTERED_MODEL_NAME
              value: ${registered_model_name_yaml}
            - name: AI_MODELS_CLEANUP_VERSION
              value: ${version_yaml}
            - name: AI_MODELS_S3_ENDPOINT_URL
              value: ${s3_endpoint_url_yaml}
            - name: AI_MODELS_S3_IGNORE_TLS
              value: ${s3_ignore_tls_yaml}
${artifacts_ca_env_block}
${dry_run_env_block}
            - name: MLFLOW_TRACKING_USERNAME
              valueFrom:
                secretKeyRef:
                  name: ai-models-backend-auth
                  key: machineUsername
            - name: MLFLOW_TRACKING_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: ai-models-backend-auth
                  key: machinePassword
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
              value: ${aws_region_yaml}
            - name: AWS_DEFAULT_REGION
              value: ${aws_region_yaml}
            - name: AWS_EC2_METADATA_DISABLED
              value: "true"
            - name: AWS_CONFIG_FILE
              value: ${aws_config_file_yaml}
            - name: HOME
              value: ${home_yaml}
            - name: TMPDIR
              value: ${tmpdir_yaml}
          volumeMounts:
            - name: work
              mountPath: /work
            - name: runtime-config
              mountPath: /etc/ai-models/aws/config
              subPath: aws-config
${artifacts_ca_volume_mount_block}
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
            limits:
              memory: 512Mi
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
${artifacts_ca_volume_block}
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
