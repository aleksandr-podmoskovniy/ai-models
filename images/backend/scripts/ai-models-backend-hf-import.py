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
from importlib import metadata as importlib_metadata
import json
import os
import re
import shutil
import sys
from pathlib import Path
from typing import Any

import mlflow
from huggingface_hub import HfApi, ModelCard, snapshot_download
from ai_models_backend_runtime import apply_s3_environment_bridge, env, env_bool


def default_registered_model_name(hf_model_id: str) -> str:
    name = re.sub(r"[^A-Za-z0-9._-]+", "--", hf_model_id).strip("-")
    return name or "hf-model"


def default_snapshot_dir(hf_model_id: str) -> str:
    base = env("AI_MODELS_IMPORT_WORKDIR", os.path.join(env("HOME", "/tmp"), "ai-models-import"))
    return os.path.join(base, default_registered_model_name(hf_model_id))


def optional_pinned_requirement(distribution: str, module: str | None = None) -> str | None:
    try:
        version = importlib_metadata.version(distribution)
    except importlib_metadata.PackageNotFoundError:
        if not module:
            return None
        try:
            imported = __import__(module)
        except Exception:
            return None
        version = getattr(imported, "__version__", None)
        if not version:
            return None

    return f"{distribution}=={version}"


def build_checkpoint_pip_requirements(task: str) -> list[str]:
    requirements: list[str] = []

    for distribution, module in (
        ("mlflow", None),
        ("transformers", None),
        ("huggingface-hub", "huggingface_hub"),
        ("safetensors", None),
        ("sentencepiece", None),
        ("pillow", "PIL"),
    ):
        requirement = optional_pinned_requirement(distribution, module)
        if requirement and requirement not in requirements:
            requirements.append(requirement)

    # Local-checkpoint imports intentionally avoid loading the full model into memory.
    # MLflow falls back to inferring both torch and tensorflow requirements when it
    # cannot determine the execution engine from a local path, which breaks in our
    # lightweight import image where tensorflow is not installed. Declare the serving
    # framework requirements explicitly instead of relying on that auto-inference.
    requirements.append("torch")
    if "image" in task or "vision" in task:
        requirements.append("torchvision")
    requirements.append("accelerate")

    return requirements


def compact_mapping(values: dict[str, Any]) -> dict[str, Any]:
    return {
        key: value
        for key, value in values.items()
        if value not in (None, "", [], {}, ())
    }


def stringify_metadata_value(value: Any) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (list, tuple, set)):
        return ",".join(str(item) for item in value)
    return str(value)


def human_size(num_bytes: int) -> str:
    units = ("B", "KiB", "MiB", "GiB", "TiB")
    size = float(num_bytes)
    for unit in units:
        if size < 1024.0 or unit == units[-1]:
            if unit == "B":
                return f"{int(size)} {unit}"
            return f"{size:.2f} {unit}"
        size /= 1024.0
    return f"{num_bytes} B"


def load_json_file(path: Path) -> dict[str, Any] | None:
    if not path.is_file():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return None


def prune_snapshot_runtime_cache(snapshot_dir: str) -> None:
    cache_dir = Path(snapshot_dir) / ".cache"
    if cache_dir.is_dir():
        shutil.rmtree(cache_dir, ignore_errors=True)


def build_snapshot_manifest(snapshot_dir: str) -> dict[str, Any]:
    root = Path(snapshot_dir)
    files: list[dict[str, Any]] = []
    total_bytes = 0

    for path in sorted(root.rglob("*")):
        if not path.is_file():
            continue
        relative_path = path.relative_to(root)
        if relative_path.parts and relative_path.parts[0] == ".cache":
            continue

        size = path.stat().st_size
        total_bytes += size
        files.append(
            {
                "path": str(relative_path),
                "size_bytes": size,
                "size_human": human_size(size),
            }
        )

    return {
        "file_count": len(files),
        "total_bytes": total_bytes,
        "total_size_human": human_size(total_bytes),
        "files": files,
    }


def extract_config_summary(snapshot_dir: str) -> dict[str, Any]:
    config = load_json_file(Path(snapshot_dir) / "config.json")
    if not config:
        return {}

    text_config = config.get("text_config") if isinstance(config.get("text_config"), dict) else {}
    summary_source = text_config or config
    architectures = config.get("architectures")

    return compact_mapping(
        {
            "model_type": config.get("model_type"),
            "architectures": ", ".join(architectures) if isinstance(architectures, list) else None,
            "torch_dtype": config.get("torch_dtype"),
            "hidden_size": summary_source.get("hidden_size"),
            "num_hidden_layers": summary_source.get("num_hidden_layers"),
            "num_attention_heads": summary_source.get("num_attention_heads"),
            "num_key_value_heads": summary_source.get("num_key_value_heads"),
            "max_position_embeddings": summary_source.get("max_position_embeddings"),
            "vocab_size": summary_source.get("vocab_size"),
        }
    )


def fetch_hf_model_context(
    hf_model_id: str, revision: str, hf_token: str
) -> tuple[dict[str, Any], ModelCard | None, dict[str, Any]]:
    revision_or_none = revision or None
    token_or_none = hf_token or None
    model_card = None
    card_data: dict[str, Any] = {}
    context: dict[str, Any] = {
        "repo_id": hf_model_id,
        "requested_revision": revision,
    }

    try:
        info = HfApi().model_info(
            hf_model_id,
            revision=revision_or_none,
            token=token_or_none,
        )
        raw_card_data = getattr(info, "card_data", None)
        if raw_card_data is not None and hasattr(raw_card_data, "to_dict"):
            card_data = raw_card_data.to_dict()

        context.update(
            compact_mapping(
                {
                    "repo_id": getattr(info, "id", hf_model_id),
                    "resolved_revision": getattr(info, "sha", None),
                    "private": getattr(info, "private", None),
                    "gated": getattr(info, "gated", None),
                    "disabled": getattr(info, "disabled", None),
                    "downloads": getattr(info, "downloads", None),
                    "likes": getattr(info, "likes", None),
                    "library_name": getattr(info, "library_name", None),
                    "pipeline_tag": getattr(info, "pipeline_tag", None),
                    "tags": getattr(info, "tags", None),
                    "license": card_data.get("license"),
                    "base_model": card_data.get("base_model"),
                }
            )
        )
    except Exception as exc:
        print(f"Warning: failed to fetch Hugging Face model metadata: {exc}", file=sys.stderr)

    try:
        model_card = ModelCard.load(hf_model_id, token=token_or_none)
    except Exception as exc:
        print(f"Warning: failed to load Hugging Face model card: {exc}", file=sys.stderr)

    return context, model_card, card_data


def build_run_params(
    hf_context: dict[str, Any], task: str, snapshot_manifest: dict[str, Any], config_summary: dict[str, Any]
) -> dict[str, str]:
    values = compact_mapping(
        {
            "ai_models.import.source": "huggingface",
            "hf.repo_id": hf_context.get("repo_id"),
            "hf.requested_revision": hf_context.get("requested_revision"),
            "hf.resolved_revision": hf_context.get("resolved_revision"),
            "hf.task": task,
            "hf.pipeline_tag": hf_context.get("pipeline_tag"),
            "hf.library_name": hf_context.get("library_name"),
            "hf.license": hf_context.get("license"),
            "hf.base_model": hf_context.get("base_model"),
            "snapshot.file_count": snapshot_manifest.get("file_count"),
            "snapshot.total_bytes": snapshot_manifest.get("total_bytes"),
            "config.model_type": config_summary.get("model_type"),
            "config.architectures": config_summary.get("architectures"),
            "config.torch_dtype": config_summary.get("torch_dtype"),
        }
    )
    return {key: stringify_metadata_value(value) for key, value in values.items()}


def build_common_tags(
    hf_context: dict[str, Any], task: str, snapshot_manifest: dict[str, Any], config_summary: dict[str, Any]
) -> dict[str, str]:
    values = compact_mapping(
        {
            "ai-models.import.source": "huggingface",
            "ai-models.import.storage-mode": "local-checkpoint",
            "hf.repo_id": hf_context.get("repo_id"),
            "hf.requested_revision": hf_context.get("requested_revision"),
            "hf.resolved_revision": hf_context.get("resolved_revision"),
            "hf.pipeline_tag": hf_context.get("pipeline_tag"),
            "hf.task": task,
            "hf.license": hf_context.get("license"),
            "hf.base_model": hf_context.get("base_model"),
            "hf.gated": hf_context.get("gated"),
            "snapshot.file_count": snapshot_manifest.get("file_count"),
            "snapshot.total_bytes": snapshot_manifest.get("total_bytes"),
            "config.model_type": config_summary.get("model_type"),
            "config.architectures": config_summary.get("architectures"),
        }
    )
    return {key: stringify_metadata_value(value) for key, value in values.items()}


def build_model_metadata(
    hf_context: dict[str, Any], task: str, snapshot_manifest: dict[str, Any], config_summary: dict[str, Any]
) -> dict[str, Any]:
    return {
        "import_source": "huggingface",
        "hf": compact_mapping(
            {
                "repo_id": hf_context.get("repo_id"),
                "requested_revision": hf_context.get("requested_revision"),
                "resolved_revision": hf_context.get("resolved_revision"),
                "pipeline_tag": hf_context.get("pipeline_tag"),
                "library_name": hf_context.get("library_name"),
                "license": hf_context.get("license"),
                "base_model": hf_context.get("base_model"),
                "gated": hf_context.get("gated"),
                "private": hf_context.get("private"),
                "downloads": hf_context.get("downloads"),
                "likes": hf_context.get("likes"),
            }
        ),
        "task": task,
        "snapshot": compact_mapping(
            {
                "file_count": snapshot_manifest.get("file_count"),
                "total_bytes": snapshot_manifest.get("total_bytes"),
                "total_size_human": snapshot_manifest.get("total_size_human"),
            }
        ),
        "config_summary": config_summary,
    }


def build_registered_model_description(
    hf_context: dict[str, Any], task: str, config_summary: dict[str, Any]
) -> str:
    lines = [f"Imported from Hugging Face `{hf_context.get('repo_id')}`."]
    facts = compact_mapping(
        {
            "Task": task,
            "Pipeline tag": hf_context.get("pipeline_tag"),
            "License": hf_context.get("license"),
            "Base model": hf_context.get("base_model"),
            "Model type": config_summary.get("model_type"),
            "Architectures": config_summary.get("architectures"),
        }
    )
    lines.extend(f"- {key}: `{value}`" for key, value in facts.items())
    return "\n".join(lines)


def build_import_run_name(hf_context: dict[str, Any]) -> str:
    repo_id = hf_context.get("repo_id", "hf-model")
    revision = hf_context.get("resolved_revision") or hf_context.get("requested_revision")
    if revision:
        return f"hf-import:{repo_id}@{revision[:12]}"
    return f"hf-import:{repo_id}"


def build_model_version_description(
    hf_context: dict[str, Any], snapshot_manifest: dict[str, Any]
) -> str:
    lines = [f"Imported from Hugging Face `{hf_context.get('repo_id')}`."]
    facts = compact_mapping(
        {
            "Revision": hf_context.get("resolved_revision") or hf_context.get("requested_revision"),
            "Snapshot files": snapshot_manifest.get("file_count"),
            "Snapshot size": snapshot_manifest.get("total_size_human"),
        }
    )
    lines.extend(f"- {key}: `{value}`" for key, value in facts.items())
    return "\n".join(lines)


def maybe_set_registered_model_metadata(
    client: mlflow.MlflowClient,
    registered_model_name: str,
    description: str,
    tags: dict[str, str],
) -> None:
    try:
        registered_model = client.get_registered_model(registered_model_name)
        if not (registered_model.description or "").strip():
            client.update_registered_model(registered_model_name, description)
    except Exception as exc:
        print(
            f"Warning: failed to update registered model description for {registered_model_name}: {exc}",
            file=sys.stderr,
        )

    for key, value in tags.items():
        try:
            client.set_registered_model_tag(registered_model_name, key, value)
        except Exception as exc:
            print(
                f"Warning: failed to set registered model tag {key} on {registered_model_name}: {exc}",
                file=sys.stderr,
            )


def set_model_version_metadata(
    client: mlflow.MlflowClient,
    registered_model_name: str,
    version: str,
    description: str,
    tags: dict[str, str],
) -> None:
    try:
        client.update_model_version(registered_model_name, version, description)
    except Exception as exc:
        print(
            f"Warning: failed to update model version description for {registered_model_name} v{version}: {exc}",
            file=sys.stderr,
        )

    for key, value in tags.items():
        try:
            client.set_model_version_tag(registered_model_name, version, key, value)
        except Exception as exc:
            print(
                f"Warning: failed to set model version tag {key} on {registered_model_name} v{version}: {exc}",
                file=sys.stderr,
            )


def log_import_metadata_artifacts(
    hf_context: dict[str, Any],
    card_data: dict[str, Any],
    model_card: ModelCard | None,
    snapshot_manifest: dict[str, Any],
    config_summary: dict[str, Any],
    snapshot_dir: str,
) -> None:
    mlflow.log_dict(
        compact_mapping(
            {
                "hf": hf_context,
                "config_summary": config_summary,
            }
        ),
        "hf/model-info.json",
    )
    mlflow.log_dict(snapshot_manifest, "hf/snapshot-manifest.json")

    if card_data:
        mlflow.log_dict(card_data, "hf/model-card-data.json")
    if model_card is not None and getattr(model_card, "text", None):
        mlflow.log_text(model_card.text, "hf/model-card.md")

    for artifact_name in ("config.json", "generation_config.json", "tokenizer_config.json"):
        config_path = Path(snapshot_dir) / artifact_name
        if config_path.is_file():
            mlflow.log_artifact(str(config_path), artifact_path="hf")


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
    prune_snapshot_runtime_cache(checkpoint_dir)
    print(f"Snapshot path: {checkpoint_dir}")

    hf_context, model_card, card_data = fetch_hf_model_context(
        args.hf_model_id,
        args.revision,
        hf_token,
    )
    snapshot_manifest = build_snapshot_manifest(checkpoint_dir)
    config_summary = extract_config_summary(checkpoint_dir)
    run_params = build_run_params(hf_context, args.task, snapshot_manifest, config_summary)
    common_tags = build_common_tags(hf_context, args.task, snapshot_manifest, config_summary)
    model_metadata = build_model_metadata(hf_context, args.task, snapshot_manifest, config_summary)
    registered_model_description = build_registered_model_description(
        hf_context,
        args.task,
        config_summary,
    )
    model_version_description = build_model_version_description(hf_context, snapshot_manifest)

    print(
        "Snapshot summary: "
        f"{snapshot_manifest['file_count']} files, {snapshot_manifest['total_size_human']}"
    )
    if resolved_revision := hf_context.get("resolved_revision"):
        print(f"Resolved revision: {resolved_revision}")

    mlflow.set_tracking_uri(tracking_uri)
    if args.workspace:
        mlflow.set_workspace(args.workspace)
    mlflow.set_experiment(args.experiment_name)

    with mlflow.start_run(run_name=build_import_run_name(hf_context)) as run:
        mlflow.log_params(run_params)
        mlflow.set_tags(common_tags)
        log_import_metadata_artifacts(
            hf_context=hf_context,
            card_data=card_data,
            model_card=model_card,
            snapshot_manifest=snapshot_manifest,
            config_summary=config_summary,
            snapshot_dir=checkpoint_dir,
        )

        model_info = mlflow.transformers.log_model(
            transformers_model=checkpoint_dir,
            task=args.task,
            name=args.artifact_name,
            model_card=model_card,
            metadata=model_metadata,
            pip_requirements=build_checkpoint_pip_requirements(args.task),
            params=run_params,
            tags=common_tags,
        )
        model_uri = model_info.model_uri
        client = mlflow.MlflowClient(tracking_uri=tracking_uri)

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
        maybe_set_registered_model_metadata(
            client,
            registered_model_name,
            registered_model_description,
            common_tags,
        )
        set_model_version_metadata(
            client,
            registered.name,
            registered.version,
            model_version_description,
            common_tags,
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
