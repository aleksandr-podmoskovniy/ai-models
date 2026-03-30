#!/usr/bin/env python3

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

"""
Import a Hugging Face model snapshot into ai-models / MLflow without loading the
full model into RAM as a transformers pipeline.

The flow is:
1. Download a Hugging Face snapshot to local disk.
2. Log it to MLflow as a local checkpoint.
3. Optionally register a model version.

This is the phase-1 runtime entrypoint for both:
- local operator helpers;
- in-cluster one-shot import Jobs;
- future controller-owned import Jobs for Model / ClusterModel.
"""

from __future__ import annotations

import argparse
import os
import re
import sys
from pathlib import Path

import mlflow
from huggingface_hub import snapshot_download


def env(name: str, default: str = "") -> str:
    return os.environ.get(name, default)


def env_bool(name: str, default: bool = False) -> bool:
    raw = os.environ.get(name)
    if raw is None:
        return default
    return raw.lower() in {"1", "true", "yes", "on"}


def default_registered_model_name(hf_model_id: str) -> str:
    name = re.sub(r"[^A-Za-z0-9._-]+", "--", hf_model_id).strip("-")
    return name or "hf-model"


def default_snapshot_dir(hf_model_id: str) -> str:
    base = env("AI_MODELS_IMPORT_WORKDIR", os.path.join(env("HOME", "/tmp"), "ai-models-import"))
    return os.path.join(base, default_registered_model_name(hf_model_id))


def apply_s3_environment_bridge() -> None:
    endpoint = env("AI_MODELS_S3_ENDPOINT_URL", "")
    ignore_tls = env("AI_MODELS_S3_IGNORE_TLS", "")

    if endpoint and not env("MLFLOW_S3_ENDPOINT_URL", ""):
        os.environ["MLFLOW_S3_ENDPOINT_URL"] = endpoint
    if ignore_tls and not env("MLFLOW_S3_IGNORE_TLS", ""):
        os.environ["MLFLOW_S3_IGNORE_TLS"] = ignore_tls


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Download a Hugging Face snapshot, log it to ai-models / MLflow, and register it."
    )

    default_tracking_uri = env(
        "AI_MODELS_IMPORT_TRACKING_URI",
        env("MLFLOW_TRACKING_URI", "http://127.0.0.1:5000"),
    )
    default_experiment_name = env("AI_MODELS_IMPORT_EXPERIMENT_NAME", "Default")
    default_hf_model_id = env("AI_MODELS_IMPORT_HF_MODEL_ID", "")
    default_task = env("AI_MODELS_IMPORT_TASK", "")
    default_registered_model_name_value = env("AI_MODELS_IMPORT_REGISTERED_MODEL_NAME", "")
    default_artifact_name = env("AI_MODELS_IMPORT_ARTIFACT_NAME", "model")
    default_revision = env("AI_MODELS_IMPORT_HF_REVISION", "")
    default_snapshot_dir_value = env("AI_MODELS_IMPORT_SNAPSHOT_DIR", "")
    default_await_registration = env("AI_MODELS_IMPORT_AWAIT_REGISTRATION_FOR", "300")
    default_workspace = env("AI_MODELS_IMPORT_WORKSPACE", "default")

    parser.add_argument(
        "--tracking-uri",
        default=default_tracking_uri,
        help="MLflow tracking URI. Defaults to AI_MODELS_IMPORT_TRACKING_URI / MLFLOW_TRACKING_URI / http://127.0.0.1:5000.",
    )
    parser.add_argument(
        "--experiment-name",
        default=default_experiment_name,
        help="MLflow experiment name. Default: Default.",
    )
    parser.add_argument(
        "--hf-model-id",
        required=not bool(default_hf_model_id),
        default=default_hf_model_id,
        help="Hugging Face model ID, for example openai/gpt-oss-20b.",
    )
    parser.add_argument(
        "--task",
        required=not bool(default_task),
        default=default_task,
        help="Transformers task, for example text-generation.",
    )
    parser.add_argument(
        "--registered-model-name",
        default=default_registered_model_name_value,
        help="Target registered model name. Defaults to a sanitized version of --hf-model-id.",
    )
    parser.add_argument(
        "--artifact-name",
        default=default_artifact_name,
        help="Artifact name used inside the MLflow run. Default: model.",
    )
    parser.add_argument(
        "--workspace",
        default=default_workspace,
        help="MLflow workspace name. Default: default.",
    )
    parser.add_argument(
        "--revision",
        default=default_revision,
        help="Optional Hugging Face revision.",
    )
    parser.add_argument(
        "--snapshot-dir",
        default=default_snapshot_dir_value,
        help="Optional local snapshot directory. Defaults to AI_MODELS_IMPORT_SNAPSHOT_DIR or a workdir under HOME.",
    )
    parser.add_argument(
        "--await-registration-for",
        default=default_await_registration,
        help="Seconds to wait for MLflow model registration. Default: 300.",
    )
    parser.add_argument(
        "--skip-registration",
        action="store_true",
        default=env_bool("AI_MODELS_IMPORT_SKIP_REGISTRATION", False),
        help="Only log the model artifacts, do not create a registered model version.",
    )
    return parser


def main() -> int:
    args = build_parser().parse_args()
    apply_s3_environment_bridge()

    tracking_uri = args.tracking_uri
    registered_model_name = args.registered_model_name or default_registered_model_name(
        args.hf_model_id
    )
    snapshot_dir = args.snapshot_dir or default_snapshot_dir(args.hf_model_id)
    hf_token = env("HF_TOKEN", env("HUGGING_FACE_HUB_TOKEN", ""))
    await_registration_for = int(args.await_registration_for)

    Path(snapshot_dir).mkdir(parents=True, exist_ok=True)

    print(f"Tracking URI: {tracking_uri}")
    print(f"Experiment: {args.experiment_name}")
    print(f"Hugging Face model: {args.hf_model_id}")
    print(f"Task: {args.task}")
    print(f"Registered model: {registered_model_name}")
    print(f"Artifact name: {args.artifact_name}")
    print(f"Workspace: {args.workspace}")
    print(f"Snapshot dir: {snapshot_dir}")
    if args.revision:
        print(f"Revision: {args.revision}")
    if hf_token:
        print("HF token: provided via environment")

    print("Downloading Hugging Face snapshot to local disk...")
    checkpoint_dir = snapshot_download(
        repo_id=args.hf_model_id,
        revision=args.revision or None,
        local_dir=snapshot_dir,
        token=hf_token or None,
    )
    print(f"Snapshot path: {checkpoint_dir}")

    mlflow.set_tracking_uri(tracking_uri)
    if args.workspace:
        mlflow.set_workspace(args.workspace)
    mlflow.set_experiment(args.experiment_name)

    with mlflow.start_run() as run:
        model_info = mlflow.transformers.log_model(
            transformers_model=checkpoint_dir,
            task=args.task,
            name=args.artifact_name,
        )
        model_uri = model_info.model_uri

        print(f"Run ID: {run.info.run_id}")
        print(f"Logged model URI: {model_uri}")

        if args.skip_registration:
            print("Registration skipped by flag.")
            print("Done.")
            return 0

        print("Registering model version...")
        registered = mlflow.register_model(
            model_uri,
            registered_model_name,
            await_registration_for=await_registration_for,
        )
        print(f"Registered model name: {registered.name}")
        print(f"Registered model version: {registered.version}")
        print("Done.")

    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        print("Interrupted.", file=sys.stderr)
        raise SystemExit(130)
