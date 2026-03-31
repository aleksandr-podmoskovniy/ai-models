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
Delete a registered model version together with the linked logged model, source
run, and S3 artifact prefixes.

Upstream MLflow intentionally treats these as separate entities. This entrypoint
provides the explicit phase-1 operator workflow that orchestrates their cleanup
for ai-models-managed imports.
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import re
import sys

import mlflow
from mlflow import MlflowClient

from ai_models_backend_runtime import apply_s3_environment_bridge, delete_s3_prefix, env, env_bool


MODEL_URI_RE = re.compile(r"^models:/(?P<model_id>m-[^/@]+)(?:/.*)?$")


@dataclass(frozen=True)
class CleanupTarget:
    registered_model_name: str
    version: str
    workspace: str
    source: str | None
    run_id: str | None
    run_artifact_uri: str | None
    logged_model_id: str | None
    logged_model_artifact_uri: str | None


def parse_logged_model_id(source: str | None) -> str | None:
    if not source:
        return None
    match = MODEL_URI_RE.match(source)
    if not match:
        return None
    return match.group("model_id")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Delete an ai-models MLflow model version together with its linked logged model, run, and S3 artifacts."
    )

    default_tracking_uri = env(
        "AI_MODELS_CLEANUP_TRACKING_URI",
        env("MLFLOW_TRACKING_URI", "http://127.0.0.1:5000"),
    )
    default_registered_model_name = env("AI_MODELS_CLEANUP_REGISTERED_MODEL_NAME", "")
    default_version = env("AI_MODELS_CLEANUP_VERSION", "")
    default_workspace = env("AI_MODELS_CLEANUP_WORKSPACE", "default")

    parser.add_argument(
        "--tracking-uri",
        default=default_tracking_uri,
        help="MLflow tracking URI. Defaults to AI_MODELS_CLEANUP_TRACKING_URI / MLFLOW_TRACKING_URI / http://127.0.0.1:5000.",
    )
    parser.add_argument(
        "--workspace",
        default=default_workspace,
        help="MLflow workspace name. Default: default.",
    )
    parser.add_argument(
        "--registered-model-name",
        required=not bool(default_registered_model_name),
        default=default_registered_model_name,
        help="Registered model name.",
    )
    parser.add_argument(
        "--version",
        required=not bool(default_version),
        default=default_version,
        help="Registered model version to remove.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        default=env_bool("AI_MODELS_CLEANUP_DRY_RUN", False),
        help="Resolve and print the cleanup plan without deleting anything.",
    )
    return parser


def resolve_cleanup_target(
    client: MlflowClient,
    registered_model_name: str,
    version: str,
    workspace: str,
) -> CleanupTarget:
    model_version = client.get_model_version(registered_model_name, version)

    run_id = model_version.run_id or None
    logged_model_id = model_version.model_id or parse_logged_model_id(model_version.source)
    logged_model_artifact_uri = None

    if logged_model_id:
        try:
            logged_model = client.get_logged_model(logged_model_id)
        except Exception as exc:
            print(
                f"Warning: failed to fetch logged model {logged_model_id}: {exc}. Continuing with partial cleanup plan.",
                file=sys.stderr,
            )
            logged_model_id = None
        else:
            logged_model_artifact_uri = logged_model.artifact_location
            if not run_id:
                run_id = logged_model.source_run_id

    run_artifact_uri = None
    if run_id:
        try:
            run = client.get_run(run_id)
        except Exception as exc:
            print(
                f"Warning: failed to fetch source run {run_id}: {exc}. Continuing without run artifact cleanup.",
                file=sys.stderr,
            )
            run_id = None
        else:
            run_artifact_uri = run.info.artifact_uri

    return CleanupTarget(
        registered_model_name=registered_model_name,
        version=str(version),
        workspace=workspace,
        source=model_version.source,
        run_id=run_id,
        run_artifact_uri=run_artifact_uri,
        logged_model_id=logged_model_id,
        logged_model_artifact_uri=logged_model_artifact_uri,
    )


def print_cleanup_target(target: CleanupTarget) -> None:
    print("Cleanup target:")
    print(f"  workspace: {target.workspace}")
    print(f"  registered model: {target.registered_model_name}")
    print(f"  version: {target.version}")
    if target.source:
        print(f"  source: {target.source}")
    if target.logged_model_id:
        print(f"  logged model id: {target.logged_model_id}")
    if target.logged_model_artifact_uri:
        print(f"  logged model artifacts: {target.logged_model_artifact_uri}")
    if target.run_id:
        print(f"  source run id: {target.run_id}")
    if target.run_artifact_uri:
        print(f"  run artifacts: {target.run_artifact_uri}")


def main() -> int:
    args = build_parser().parse_args()
    apply_s3_environment_bridge()

    mlflow.set_tracking_uri(args.tracking_uri)
    if args.workspace:
        mlflow.set_workspace(args.workspace)

    client = MlflowClient(tracking_uri=args.tracking_uri)
    target = resolve_cleanup_target(
        client=client,
        registered_model_name=args.registered_model_name,
        version=str(args.version),
        workspace=args.workspace,
    )
    print_cleanup_target(target)

    if args.dry_run:
        print("Dry-run mode enabled; no deletes performed.")
        return 0

    print("Deleting registered model version...")
    client.delete_model_version(target.registered_model_name, target.version)

    if target.logged_model_id:
        print("Deleting linked logged model...")
        client.delete_logged_model(target.logged_model_id)
    else:
        print("No linked logged model resolved from the model version source; skipping logged model delete.")

    if target.run_id:
        print("Deleting linked source run...")
        client.delete_run(target.run_id)
    else:
        print("No linked source run resolved; skipping run delete.")

    artifact_uris = []
    for artifact_uri in (target.logged_model_artifact_uri, target.run_artifact_uri):
        if artifact_uri and artifact_uri not in artifact_uris:
            artifact_uris.append(artifact_uri)

    if artifact_uris:
        print("Deleting linked S3 artifacts...")
    else:
        print("No linked artifact prefixes resolved; skipping physical artifact cleanup.")

    for artifact_uri in artifact_uris:
        result = delete_s3_prefix(artifact_uri)
        print(
            f"  {artifact_uri}: before={result['keys_before']} deleted={result['keys_deleted']} after={result['keys_after']}"
        )

    print("Done.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        print("Interrupted.", file=sys.stderr)
        raise SystemExit(130)
