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
Publish a model source into the current backend artifact plane without routing
through MLflow metadata entities.

Current live scope:
- Hugging Face source
- HTTP archive source with safe extraction guards
- Upload source with HuggingFaceDirectory archives
- KitOps packaging and OCI publication
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import ssl
import stat
import subprocess
import sys
import tarfile
import tempfile
from typing import Any
from urllib import parse as urllib_parse
from urllib import request as urllib_request
import zipfile

from huggingface_hub import HfApi, snapshot_download

from ai_models_backend_runtime import env


def compact_mapping(values: dict[str, Any]) -> dict[str, Any]:
    return {
        key: value
        for key, value in values.items()
        if value not in (None, "", [], {}, ())
    }


def load_json_file(path: Path) -> dict[str, Any] | None:
    if not path.is_file():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return None


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


def first_csv_item(value: str | None) -> str | None:
    if not value:
        return None
    for item in value.split(","):
        clean = item.strip()
        if clean:
            return clean
    return None


def default_snapshot_dir(source_reference: str) -> str:
    base = env("AI_MODELS_IMPORT_WORKDIR", os.path.join(env("HOME", "/tmp"), "ai-models-publish"))
    safe = source_reference.replace("/", "--").replace(":", "--")
    return os.path.join(base, safe)


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


def fetch_hf_model_context(hf_model_id: str, revision: str, hf_token: str) -> dict[str, Any]:
    revision_or_none = revision or None
    token_or_none = hf_token or None
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
        card_data = raw_card_data.to_dict() if raw_card_data is not None and hasattr(raw_card_data, "to_dict") else {}
        context.update(
            compact_mapping(
                {
                    "repo_id": getattr(info, "id", hf_model_id),
                    "resolved_revision": getattr(info, "sha", None),
                    "private": getattr(info, "private", None),
                    "gated": getattr(info, "gated", None),
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
        print(f"Warning: failed to fetch Hugging Face metadata: {exc}", file=sys.stderr)

    return context


def materialize_http_ca_bundle(snapshot_dir: str, inline_b64: str) -> str:
    if not inline_b64:
        return ""

    try:
        decoded = base64.b64decode(inline_b64)
    except Exception as exc:
        raise RuntimeError(f"failed to decode http CA bundle: {exc}") from exc

    if not decoded.strip():
        return ""

    ca_path = Path(snapshot_dir) / ".http-source-ca.crt"
    ca_path.write_bytes(decoded.strip() + b"\n")
    return str(ca_path)


def http_ssl_context(snapshot_dir: str, inline_b64: str) -> ssl.SSLContext:
    ca_file = materialize_http_ca_bundle(snapshot_dir, inline_b64)
    if ca_file:
        return ssl.create_default_context(cafile=ca_file)
    return ssl.create_default_context()


def http_auth_headers_from_dir(auth_dir: str) -> dict[str, str]:
    if not auth_dir:
        return {}

    root = Path(auth_dir)
    if not root.is_dir():
        raise RuntimeError(f"HTTP auth directory does not exist: {auth_dir}")

    authorization = root / "authorization"
    if authorization.is_file():
        return {"Authorization": authorization.read_text(encoding="utf-8").strip()}

    username = root / "username"
    password = root / "password"
    if username.is_file() and password.is_file():
        token = base64.b64encode(
            f"{username.read_text(encoding='utf-8').strip()}:{password.read_text(encoding='utf-8').strip()}".encode(
                "utf-8"
            )
        ).decode("utf-8")
        return {"Authorization": f"Basic {token}"}

    return {}


def filename_from_http_response(url: str, response: Any) -> str:
    content_disposition = response.headers.get("Content-Disposition", "")
    match = re.search(r'filename\*?=(?:UTF-8\'\')?"?([^\";]+)"?', content_disposition)
    if match:
        return Path(match.group(1)).name or "model-download"

    parsed = urllib_parse.urlparse(url)
    basename = Path(parsed.path).name
    if basename:
        return basename

    return "model-download"


def archive_relative_path(name: str) -> Path:
    raw_name = name.strip().replace("\\", "/")
    if not raw_name:
        raise RuntimeError("archive entry name must not be empty")

    path = PurePosixPath(raw_name)
    if path.is_absolute():
        raise RuntimeError(f"refusing to extract absolute archive entry {name!r}")

    parts = []
    for part in path.parts:
        if part in ("", "."):
            continue
        if part == "..":
            raise RuntimeError(f"refusing to extract archive entry outside of destination: {name!r}")
        parts.append(part)

    if not parts:
        return Path(".")

    return Path(*parts)


def archive_target_path(destination: Path, name: str) -> Path:
    relative = archive_relative_path(name)
    if relative == Path("."):
        return destination
    return destination / relative


def safe_extract_tar(archive_path: Path, destination: Path) -> None:
    with tarfile.open(archive_path, "r:*") as archive:
        for member in archive.getmembers():
            target = archive_target_path(destination, member.name)
            if member.issym():
                raise RuntimeError(f"refusing to extract symbolic link tar entry {member.name!r}")
            if member.islnk():
                raise RuntimeError(f"refusing to extract hard link tar entry {member.name!r}")
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
                continue
            if not member.isfile():
                raise RuntimeError(f"refusing to extract unsupported tar entry type for {member.name!r}")

            target.parent.mkdir(parents=True, exist_ok=True)
            extracted = archive.extractfile(member)
            if extracted is None:
                raise RuntimeError(f"failed to read tar entry {member.name!r}")
            with extracted, target.open("wb") as stream:
                shutil.copyfileobj(extracted, stream)


def is_zip_symlink(member: zipfile.ZipInfo) -> bool:
    mode = (member.external_attr >> 16) & 0xFFFF
    return stat.S_ISLNK(mode)


def safe_extract_zip(archive_path: Path, destination: Path) -> None:
    with zipfile.ZipFile(archive_path) as archive:
        for member in archive.infolist():
            target = archive_target_path(destination, member.filename)
            if member.is_dir():
                target.mkdir(parents=True, exist_ok=True)
                continue
            if is_zip_symlink(member):
                raise RuntimeError(f"refusing to extract symbolic link zip entry {member.filename!r}")

            target.parent.mkdir(parents=True, exist_ok=True)
            with archive.open(member, "r") as extracted, target.open("wb") as stream:
                shutil.copyfileobj(extracted, stream)


def normalize_extracted_root(destination: Path) -> Path:
    entries = [
        entry
        for entry in destination.iterdir()
        if entry.name not in {".DS_Store", "__MACOSX"}
    ]
    if len(entries) == 1 and entries[0].is_dir():
        return entries[0]
    return destination


def unpack_http_archive(archive_path: Path, destination: Path) -> Path:
    destination.mkdir(parents=True, exist_ok=True)
    if tarfile.is_tarfile(archive_path):
        safe_extract_tar(archive_path, destination)
        return normalize_extracted_root(destination)
    if zipfile.is_zipfile(archive_path):
        safe_extract_zip(archive_path, destination)
        return normalize_extracted_root(destination)

    raise RuntimeError(
        "HTTP source currently expects a .tar, .tar.gz, .tgz, or .zip archive containing a Hugging Face checkpoint"
    )


def download_http_archive(
    http_url: str,
    snapshot_dir: str,
    inline_ca_b64: str,
    http_auth_dir: str,
) -> tuple[Path, dict[str, Any]]:
    request = urllib_request.Request(http_url, headers=http_auth_headers_from_dir(http_auth_dir))
    context = http_ssl_context(snapshot_dir, inline_ca_b64)

    download_dir = Path(snapshot_dir) / ".download"
    download_dir.mkdir(parents=True, exist_ok=True)

    with urllib_request.urlopen(request, context=context) as response:
        filename = filename_from_http_response(http_url, response)
        archive_path = download_dir / filename
        with archive_path.open("wb") as stream:
            shutil.copyfileobj(response, stream)

        metadata = compact_mapping(
            {
                "url": http_url,
                "filename": filename,
                "etag": response.headers.get("ETag"),
                "last_modified": response.headers.get("Last-Modified"),
                "content_type": response.headers.get("Content-Type"),
            }
        )

    return archive_path, metadata


def resolved_http_revision(http_context: dict[str, Any]) -> str:
    if etag := http_context.get("etag"):
        return f"etag:{etag}"
    if last_modified := http_context.get("last_modified"):
        return f"last-modified:{last_modified}"
    return ""


def kube_api_base() -> str:
    host = os.getenv("KUBERNETES_SERVICE_HOST", "").strip()
    port = os.getenv("KUBERNETES_SERVICE_PORT", "").strip()
    if not host or not port:
        raise RuntimeError("in-cluster Kubernetes API environment is not available")
    return f"https://{host}:{port}"


def service_account_token() -> str:
    token_path = Path("/var/run/secrets/kubernetes.io/serviceaccount/token")
    if not token_path.is_file():
        raise RuntimeError("service account token is not mounted")
    return token_path.read_text(encoding="utf-8").strip()


def kubernetes_ssl_context() -> ssl.SSLContext:
    ca_path = Path("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
    if not ca_path.is_file():
        raise RuntimeError("service account CA bundle is not mounted")
    return ssl.create_default_context(cafile=str(ca_path))


def update_result_configmap(name: str, namespace: str, mutate: Any) -> None:
    if not name or not namespace:
        return

    url = f"{kube_api_base()}/api/v1/namespaces/{namespace}/configmaps/{name}"
    headers = {
        "Authorization": f"Bearer {service_account_token()}",
        "Accept": "application/json",
    }
    context = kubernetes_ssl_context()

    get_request = urllib_request.Request(url, headers=headers, method="GET")
    with urllib_request.urlopen(get_request, context=context) as response:
        payload = json.loads(response.read().decode("utf-8"))

    data = payload.get("data") or {}
    mutate(data)
    payload["data"] = data

    put_headers = dict(headers)
    put_headers["Content-Type"] = "application/json"
    put_request = urllib_request.Request(
        url,
        headers=put_headers,
        data=json.dumps(payload).encode("utf-8"),
        method="PUT",
    )
    with urllib_request.urlopen(put_request, context=context):
        return


def write_worker_running(configmap_name: str, configmap_namespace: str) -> None:
    if not configmap_name or not configmap_namespace:
        return
    try:
        update_result_configmap(
            configmap_name,
            configmap_namespace,
            lambda data: data.update({"state": "Running", "message": ""}),
        )
    except Exception as exc:
        print(
            f"Warning: failed to mark worker as running in ConfigMap {configmap_namespace}/{configmap_name}: {exc}",
            file=sys.stderr,
        )


def write_worker_failure(configmap_name: str, configmap_namespace: str, message: str) -> None:
    if not configmap_name or not configmap_namespace:
        return

    clean_message = message.strip() or "source publish failed"
    try:
        update_result_configmap(
            configmap_name,
            configmap_namespace,
            lambda data: data.update(
                {
                    "state": "Failed",
                    "worker-result.json": "",
                    "result.json": "",
                    "worker-failure.txt": clean_message,
                    "message": clean_message,
                }
            ),
        )
    except Exception as exc:
        print(
            f"Warning: failed to write worker failure to ConfigMap {configmap_namespace}/{configmap_name}: {exc}",
            file=sys.stderr,
        )


def write_worker_result(configmap_name: str, configmap_namespace: str, payload: dict[str, Any]) -> None:
    if not configmap_name or not configmap_namespace:
        return

    serialized = json.dumps(payload, separators=(",", ":"), sort_keys=True)
    try:
        update_result_configmap(
            configmap_name,
            configmap_namespace,
            lambda data: data.update(
                {
                    "state": "Succeeded",
                    "worker-result.json": serialized,
                    "result.json": "",
                    "worker-failure.txt": "",
                    "message": "",
                }
            ),
        )
    except Exception as exc:
        print(
            f"Warning: failed to write worker result to ConfigMap {configmap_namespace}/{configmap_name}: {exc}",
            file=sys.stderr,
        )


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while True:
            chunk = handle.read(1024 * 1024)
            if not chunk:
                break
            digest.update(chunk)
    return digest.hexdigest()


def build_directory_digest(snapshot_dir: str) -> str:
    root = Path(snapshot_dir)
    entries: list[dict[str, Any]] = []
    for path in sorted(root.rglob("*")):
        if not path.is_file():
            continue
        relative_path = path.relative_to(root)
        if relative_path.parts and relative_path.parts[0] == ".cache":
            continue
        entries.append(
            {
                "path": str(relative_path),
                "sha256": sha256_file(path),
                "size": path.stat().st_size,
            }
        )

    digest = hashlib.sha256(
        json.dumps(entries, separators=(",", ":"), sort_keys=True).encode("utf-8")
    ).hexdigest()
    return f"sha256:{digest}"


def registry_from_oci_reference(reference: str) -> str:
    clean_reference = reference.strip()
    if not clean_reference:
        raise RuntimeError("artifact-uri is required")

    without_digest = clean_reference.split("@", 1)[0]
    registry, separator, repository = without_digest.partition("/")
    if not separator or not registry or not repository:
        raise RuntimeError(
            f"artifact-uri must include registry host and repository path, got {reference!r}"
        )
    if "." not in registry and ":" not in registry and registry != "localhost":
        raise RuntimeError(
            f"artifact-uri must include an explicit OCI registry host, got {reference!r}"
        )
    return registry


def immutable_oci_reference(reference: str, digest: str) -> str:
    clean_reference = reference.strip()
    clean_digest = digest.strip()
    if not clean_reference or not clean_digest:
        raise RuntimeError("artifact-uri and digest must not be empty")

    without_digest = clean_reference.split("@", 1)[0]
    repository_part = without_digest.rsplit("/", 1)[-1]
    if ":" in repository_part:
        without_digest = without_digest.rsplit(":", 1)[0]

    return f"{without_digest}@{clean_digest}"


def package_name_from_oci_reference(reference: str) -> str:
    clean_reference = reference.strip().split("@", 1)[0]
    if "/" in clean_reference:
        clean_reference = clean_reference.rsplit("/", 1)[-1]
    if ":" in clean_reference:
        clean_reference = clean_reference.rsplit(":", 1)[0]
    clean_reference = clean_reference.strip()
    if clean_reference:
        return clean_reference
    return "model"


def kit_connection_flags() -> list[str]:
    flags: list[str] = []
    if env("AI_MODELS_OCI_INSECURE", "").strip().lower() in ("1", "true", "yes", "on"):
        flags.append("--tls-verify=false")

    return flags


def kit_command(config_dir: str, *args: str, include_log_level: bool = True) -> list[str]:
    command = ["kit", "--config", config_dir, "--progress", "none"]
    if include_log_level:
        command.extend(["--log-level", "error"])
    command.extend(args)
    return command


def kit_runtime_environment(config_dir: str) -> dict[str, str]:
    environment = os.environ.copy()
    environment["HOME"] = config_dir
    environment["DOCKER_CONFIG"] = os.path.join(config_dir, "docker")
    Path(environment["DOCKER_CONFIG"]).mkdir(parents=True, exist_ok=True)

    ca_file = env("AI_MODELS_OCI_CA_FILE", "").strip()
    if ca_file:
        environment["SSL_CERT_FILE"] = ca_file

    return environment


def run_command(
    command: list[str],
    *,
    stdin_text: str | None = None,
    extra_env: dict[str, str] | None = None,
) -> subprocess.CompletedProcess[str]:
    environment = os.environ.copy()
    if extra_env:
        environment.update(extra_env)

    return subprocess.run(
        command,
        input=stdin_text,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        env=environment,
    )


def require_command_success(
    command: list[str],
    *,
    stdin_text: str | None = None,
    extra_env: dict[str, str] | None = None,
    error_prefix: str,
) -> subprocess.CompletedProcess[str]:
    completed = run_command(command, stdin_text=stdin_text, extra_env=extra_env)
    if completed.returncode == 0:
        return completed

    details = completed.stderr.strip() or completed.stdout.strip() or f"exit code {completed.returncode}"
    raise RuntimeError(f"{error_prefix}: {details}")


def configure_kit_cli(config_dir: str) -> None:
    require_command_success(
        kit_command(config_dir, "version", "--show-update-notifications=false"),
        extra_env=kit_runtime_environment(config_dir),
        error_prefix="failed to initialize KitOps CLI config",
    )


def login_kit_registry(config_dir: str, artifact_uri: str) -> None:
    username = env("AI_MODELS_OCI_USERNAME", "").strip()
    password = env("AI_MODELS_OCI_PASSWORD", "")
    if not username:
        raise RuntimeError("AI_MODELS_OCI_USERNAME must not be empty")
    if password == "":
        raise RuntimeError("AI_MODELS_OCI_PASSWORD must not be empty")

    registry = registry_from_oci_reference(artifact_uri)
    command = kit_command(
        config_dir,
        "login",
        registry,
        "-u",
        username,
        "--password-stdin",
        *kit_connection_flags(),
    )
    require_command_success(
        command,
        stdin_text=password,
        extra_env=kit_runtime_environment(config_dir),
        error_prefix=f"failed to login to OCI registry {registry}",
    )


def prepare_modelkit_context(checkpoint_dir: str, artifact_uri: str, description: str) -> str:
    checkpoint_path = Path(checkpoint_dir)
    if not checkpoint_path.is_dir():
        raise RuntimeError(f"checkpoint directory {checkpoint_dir!r} does not exist")

    context_dir = tempfile.mkdtemp(prefix="ai-model-kitops-context-")
    context_path = Path(context_dir)
    (context_path / "model").symlink_to(checkpoint_path, target_is_directory=True)
    description = description.replace('"', "'").strip() or "Published model"
    (context_path / "Kitfile").write_text(
        "\n".join(
            (
                "manifestVersion: v1alpha2",
                "package:",
                f"  name: {package_name_from_oci_reference(artifact_uri)}",
                f'  description: "{description}"',
                "model:",
                "  path: model",
                "",
            )
        ),
        encoding="utf-8",
    )
    return context_dir


def pack_modelkit(config_dir: str, context_dir: str, artifact_uri: str) -> None:
    command = kit_command(
        config_dir,
        "pack",
        context_dir,
        "-t",
        artifact_uri,
        "--use-model-pack",
    )
    require_command_success(
        command,
        extra_env=kit_runtime_environment(config_dir),
        error_prefix="failed to pack ModelKit",
    )


def push_modelkit(config_dir: str, artifact_uri: str) -> None:
    command = kit_command(
        config_dir,
        "push",
        artifact_uri,
        *kit_connection_flags(),
    )
    require_command_success(
        command,
        extra_env=kit_runtime_environment(config_dir),
        error_prefix="failed to push ModelKit to OCI registry",
    )


def inspect_remote_modelkit(config_dir: str, artifact_uri: str) -> dict[str, Any]:
    command = kit_command(
        config_dir,
        "inspect",
        "--remote",
        artifact_uri,
        *kit_connection_flags(),
        include_log_level=False,
    )
    completed = require_command_success(
        command,
        extra_env=kit_runtime_environment(config_dir),
        error_prefix="failed to inspect remote ModelKit",
    )
    payload = completed.stdout.strip()
    if not payload:
        raise RuntimeError("kit inspect returned an empty payload")
    if "{" in payload:
        payload = payload[payload.index("{") :]
    try:
        return json.loads(payload)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"failed to decode kit inspect output: {exc}") from exc


def inspect_modelkit_size(inspect_payload: dict[str, Any]) -> int:
    manifest = inspect_payload.get("manifest")
    if not isinstance(manifest, dict):
        return 0

    total = 0
    config = manifest.get("config")
    if isinstance(config, dict):
        size = config.get("size")
        if isinstance(size, int) and size > 0:
            total += size

    layers = manifest.get("layers")
    if isinstance(layers, list):
        for layer in layers:
            if not isinstance(layer, dict):
                continue
            size = layer.get("size")
            if isinstance(size, int) and size > 0:
                total += size

    return total


def publish_checkpoint_as_modelkit(checkpoint_dir: str, artifact_uri: str, description: str) -> dict[str, Any]:
    with tempfile.TemporaryDirectory(prefix="ai-model-kitops-config-") as config_dir:
        configure_kit_cli(config_dir)
        login_kit_registry(config_dir, artifact_uri)
        context_dir = prepare_modelkit_context(checkpoint_dir, artifact_uri, description)
        try:
            pack_modelkit(config_dir, context_dir, artifact_uri)
            push_modelkit(config_dir, artifact_uri)
            return inspect_remote_modelkit(config_dir, artifact_uri)
        finally:
            shutil.rmtree(context_dir, ignore_errors=True)


def artifact_digest_from_inspect_payload(inspect_payload: dict[str, Any]) -> str:
    digest = str(inspect_payload.get("digest", "")).strip()
    if not digest:
        raise RuntimeError("kit inspect payload is missing digest")
    return digest


def artifact_media_type_from_inspect_payload(inspect_payload: dict[str, Any]) -> str:
    manifest = inspect_payload.get("manifest")
    if isinstance(manifest, dict):
        artifact_type = str(manifest.get("artifactType", "")).strip()
        if artifact_type:
            return artifact_type

    return "application/vnd.cncf.model.manifest.v1+json"


def build_backend_result(
    hf_context: dict[str, Any],
    task: str,
    config_summary: dict[str, Any],
    artifact_uri: str,
    artifact_digest: str,
    artifact_media_type: str,
    artifact_size_bytes: int,
) -> dict[str, Any]:
    published_artifact_uri = immutable_oci_reference(artifact_uri, artifact_digest)
    return compact_mapping(
        {
            "source": compact_mapping(
                {
                    "type": "HuggingFace",
                    "externalReference": hf_context.get("repo_id"),
                    "resolvedRevision": hf_context.get("resolved_revision") or hf_context.get("requested_revision"),
                }
            ),
            "artifact": compact_mapping(
                {
                    "kind": "OCI",
                    "uri": published_artifact_uri,
                    "digest": artifact_digest,
                    "mediaType": artifact_media_type,
                    "sizeBytes": artifact_size_bytes,
                }
            ),
            "resolved": compact_mapping(
                {
                    "task": task,
                    "framework": hf_context.get("library_name") or "transformers",
                    "family": config_summary.get("model_type"),
                    "license": hf_context.get("license"),
                    "architecture": first_csv_item(config_summary.get("architectures")),
                    "format": "HuggingFaceCheckpoint",
                    "contextWindowTokens": config_summary.get("max_position_embeddings"),
                    "sourceRepoID": hf_context.get("repo_id"),
                }
            ),
            "cleanupHandle": compact_mapping(
                {
                    "kind": "BackendArtifact",
                    "artifact": compact_mapping(
                        {
                            "kind": "OCI",
                            "uri": published_artifact_uri,
                            "digest": artifact_digest,
                        }
                    ),
                    "backend": compact_mapping(
                        {
                            "reference": published_artifact_uri,
                        }
                    ),
                }
            ),
        }
    )


def build_http_backend_result(
    http_context: dict[str, Any],
    task: str,
    config_summary: dict[str, Any],
    artifact_uri: str,
    artifact_digest: str,
    artifact_media_type: str,
    artifact_size_bytes: int,
) -> dict[str, Any]:
    published_artifact_uri = immutable_oci_reference(artifact_uri, artifact_digest)
    return compact_mapping(
        {
            "source": compact_mapping(
                {
                    "type": "HTTP",
                    "externalReference": http_context.get("url"),
                    "resolvedRevision": resolved_http_revision(http_context),
                }
            ),
            "artifact": compact_mapping(
                {
                    "kind": "OCI",
                    "uri": published_artifact_uri,
                    "digest": artifact_digest,
                    "mediaType": artifact_media_type,
                    "sizeBytes": artifact_size_bytes,
                }
            ),
            "resolved": compact_mapping(
                {
                    "task": task,
                    "framework": "transformers",
                    "family": config_summary.get("model_type"),
                    "architecture": first_csv_item(config_summary.get("architectures")),
                    "format": "HuggingFaceCheckpoint",
                    "contextWindowTokens": config_summary.get("max_position_embeddings"),
                }
            ),
            "cleanupHandle": compact_mapping(
                {
                    "kind": "BackendArtifact",
                    "artifact": compact_mapping(
                        {
                            "kind": "OCI",
                            "uri": published_artifact_uri,
                            "digest": artifact_digest,
                        }
                    ),
                    "backend": compact_mapping(
                        {
                            "reference": published_artifact_uri,
                        }
                    ),
                }
            ),
        }
    )


def build_upload_backend_result(
    task: str,
    config_summary: dict[str, Any],
    artifact_uri: str,
    artifact_digest: str,
    artifact_media_type: str,
    artifact_size_bytes: int,
) -> dict[str, Any]:
    published_artifact_uri = immutable_oci_reference(artifact_uri, artifact_digest)
    return compact_mapping(
        {
            "source": compact_mapping(
                {
                    "type": "Upload",
                }
            ),
            "artifact": compact_mapping(
                {
                    "kind": "OCI",
                    "uri": published_artifact_uri,
                    "digest": artifact_digest,
                    "mediaType": artifact_media_type,
                    "sizeBytes": artifact_size_bytes,
                }
            ),
            "resolved": compact_mapping(
                {
                    "task": task,
                    "framework": "transformers",
                    "family": config_summary.get("model_type"),
                    "architecture": first_csv_item(config_summary.get("architectures")),
                    "format": "HuggingFaceCheckpoint",
                    "contextWindowTokens": config_summary.get("max_position_embeddings"),
                }
            ),
            "cleanupHandle": compact_mapping(
                {
                    "kind": "BackendArtifact",
                    "artifact": compact_mapping(
                        {
                            "kind": "OCI",
                            "uri": published_artifact_uri,
                            "digest": artifact_digest,
                        }
                    ),
                    "backend": compact_mapping(
                        {
                            "reference": published_artifact_uri,
                        }
                    ),
                }
            ),
        }
    )


def unpack_upload_archive(upload_path: str, upload_format: str, snapshot_dir: str) -> Path:
    if not upload_path:
        raise RuntimeError("upload-path is required")

    upload_source = Path(upload_path)
    if not upload_source.is_file():
        raise RuntimeError(f"upload path {upload_source} does not exist or is not a file")

    if upload_format == "HuggingFaceDirectory":
        return unpack_http_archive(upload_source, Path(snapshot_dir) / "checkpoint")

    if upload_format == "ModelKit":
        raise RuntimeError(
            "upload format ModelKit is not implemented in the current upload session flow; use HuggingFaceDirectory until direct ModelKit ingest is implemented"
        )

    raise RuntimeError(f"unsupported upload format: {upload_format}")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Download a model source, package it with KitOps, and publish it into the current backend OCI artifact plane."
    )

    parser.add_argument(
        "--source-type",
        default=env("AI_MODELS_PUBLISH_SOURCE_TYPE", "HuggingFace"),
        choices=("HuggingFace", "HTTP", "Upload"),
        help="Model source type. Current live support: HuggingFace, HTTP archive, and Upload(HuggingFaceDirectory).",
    )
    parser.add_argument("--artifact-uri", required=True, help="Controller-owned destination URI for the saved artifact.")
    parser.add_argument("--result-configmap-name", default=env("AI_MODELS_IMPORT_RESULT_CONFIGMAP_NAME", ""))
    parser.add_argument("--result-configmap-namespace", default=env("AI_MODELS_IMPORT_RESULT_CONFIGMAP_NAMESPACE", ""))
    parser.add_argument("--hf-model-id", default=env("AI_MODELS_IMPORT_HF_MODEL_ID", ""))
    parser.add_argument("--http-url", default=env("AI_MODELS_IMPORT_HTTP_URL", ""))
    parser.add_argument("--http-ca-bundle-b64", default=env("AI_MODELS_IMPORT_HTTP_CA_BUNDLE_B64", ""))
    parser.add_argument("--http-auth-dir", default=env("AI_MODELS_IMPORT_HTTP_AUTH_DIR", ""))
    parser.add_argument("--upload-path", default=env("AI_MODELS_IMPORT_UPLOAD_PATH", ""))
    parser.add_argument("--upload-format", default=env("AI_MODELS_IMPORT_UPLOAD_FORMAT", "HuggingFaceDirectory"))
    parser.add_argument("--revision", default=env("AI_MODELS_IMPORT_HF_REVISION", ""))
    parser.add_argument("--task", default=env("AI_MODELS_IMPORT_TASK", ""))
    parser.add_argument("--snapshot-dir", default=env("AI_MODELS_IMPORT_SNAPSHOT_DIR", ""))
    return parser


def run(args: argparse.Namespace) -> int:
    artifact_uri = args.artifact_uri.strip()
    if not artifact_uri:
        raise RuntimeError("artifact-uri is required")

    write_worker_running(args.result_configmap_name, args.result_configmap_namespace)

    if args.source_type == "HuggingFace":
        if not args.hf_model_id:
            raise RuntimeError("hf-model-id is required")

        hf_token = env("HF_TOKEN", env("HUGGING_FACE_HUB_TOKEN", ""))
        snapshot_dir = args.snapshot_dir or default_snapshot_dir(args.hf_model_id)
        Path(snapshot_dir).mkdir(parents=True, exist_ok=True)

        print(f"Publishing Hugging Face model {args.hf_model_id} into {artifact_uri}")
        checkpoint_dir = snapshot_download(
            repo_id=args.hf_model_id,
            revision=args.revision or None,
            local_dir=snapshot_dir,
            token=hf_token or None,
        )
        prune_snapshot_runtime_cache(checkpoint_dir)

        hf_context = fetch_hf_model_context(args.hf_model_id, args.revision, hf_token)
        resolved_task = args.task or hf_context.get("pipeline_tag") or ""
        if not resolved_task:
            raise RuntimeError("task is required either via --task or from Hugging Face pipeline_tag metadata")

        snapshot_manifest = build_snapshot_manifest(checkpoint_dir)
        config_summary = extract_config_summary(checkpoint_dir)
        source_digest = build_directory_digest(checkpoint_dir)

        print(
            "Snapshot summary: "
            f"{snapshot_manifest['file_count']} files, {snapshot_manifest['total_size_human']}, source digest {source_digest}"
        )
        inspect_payload = publish_checkpoint_as_modelkit(
            checkpoint_dir,
            artifact_uri,
            f"Published from Hugging Face source {args.hf_model_id}",
        )
        artifact_digest = artifact_digest_from_inspect_payload(inspect_payload)
        artifact_media_type = artifact_media_type_from_inspect_payload(inspect_payload)
        artifact_size_bytes = inspect_modelkit_size(inspect_payload) or snapshot_manifest.get("total_bytes", 0)

        result_payload = build_backend_result(
            hf_context=hf_context,
            task=resolved_task,
            config_summary=config_summary,
            artifact_uri=artifact_uri,
            artifact_digest=artifact_digest,
            artifact_media_type=artifact_media_type,
            artifact_size_bytes=artifact_size_bytes,
        )
    elif args.source_type == "HTTP":
        if not args.http_url:
            raise RuntimeError("http-url is required")
        if not args.task:
            raise RuntimeError("task is required for HTTP source")

        snapshot_dir = args.snapshot_dir or default_snapshot_dir(args.http_url)
        Path(snapshot_dir).mkdir(parents=True, exist_ok=True)

        print(f"Publishing HTTP model archive {args.http_url} into {artifact_uri}")
        archive_path, http_context = download_http_archive(
            args.http_url,
            snapshot_dir,
            args.http_ca_bundle_b64,
            args.http_auth_dir,
        )
        checkpoint_dir = unpack_http_archive(archive_path, Path(snapshot_dir) / "checkpoint")
        prune_snapshot_runtime_cache(str(checkpoint_dir))

        snapshot_manifest = build_snapshot_manifest(str(checkpoint_dir))
        config_summary = extract_config_summary(str(checkpoint_dir))
        source_digest = build_directory_digest(str(checkpoint_dir))

        print(
            "Snapshot summary: "
            f"{snapshot_manifest['file_count']} files, {snapshot_manifest['total_size_human']}, source digest {source_digest}"
        )
        inspect_payload = publish_checkpoint_as_modelkit(
            str(checkpoint_dir),
            artifact_uri,
            f"Published from HTTP source {args.http_url}",
        )
        artifact_digest = artifact_digest_from_inspect_payload(inspect_payload)
        artifact_media_type = artifact_media_type_from_inspect_payload(inspect_payload)
        artifact_size_bytes = inspect_modelkit_size(inspect_payload) or snapshot_manifest.get("total_bytes", 0)

        result_payload = build_http_backend_result(
            http_context=http_context,
            task=args.task,
            config_summary=config_summary,
            artifact_uri=artifact_uri,
            artifact_digest=artifact_digest,
            artifact_media_type=artifact_media_type,
            artifact_size_bytes=artifact_size_bytes,
        )
    elif args.source_type == "Upload":
        if not args.task:
            raise RuntimeError("task is required for Upload source")

        snapshot_dir = args.snapshot_dir or default_snapshot_dir(f"upload-{artifact_uri}")
        Path(snapshot_dir).mkdir(parents=True, exist_ok=True)

        print(f"Publishing uploaded model content from {args.upload_path} into {artifact_uri}")
        checkpoint_dir = unpack_upload_archive(args.upload_path, args.upload_format, snapshot_dir)
        prune_snapshot_runtime_cache(str(checkpoint_dir))

        snapshot_manifest = build_snapshot_manifest(str(checkpoint_dir))
        config_summary = extract_config_summary(str(checkpoint_dir))
        source_digest = build_directory_digest(str(checkpoint_dir))

        print(
            "Snapshot summary: "
            f"{snapshot_manifest['file_count']} files, {snapshot_manifest['total_size_human']}, source digest {source_digest}"
        )
        inspect_payload = publish_checkpoint_as_modelkit(
            str(checkpoint_dir),
            artifact_uri,
            "Published from controller-owned upload source",
        )
        artifact_digest = artifact_digest_from_inspect_payload(inspect_payload)
        artifact_media_type = artifact_media_type_from_inspect_payload(inspect_payload)
        artifact_size_bytes = inspect_modelkit_size(inspect_payload) or snapshot_manifest.get("total_bytes", 0)

        result_payload = build_upload_backend_result(
            task=args.task,
            config_summary=config_summary,
            artifact_uri=artifact_uri,
            artifact_digest=artifact_digest,
            artifact_media_type=artifact_media_type,
            artifact_size_bytes=artifact_size_bytes,
        )
    else:
        raise RuntimeError(f"unsupported source type: {args.source_type}")

    write_worker_result(args.result_configmap_name, args.result_configmap_namespace, result_payload)
    print("Done.")
    return 0


def main() -> int:
    args = build_parser().parse_args()
    try:
        return run(args)
    except Exception as exc:
        write_worker_failure(
            args.result_configmap_name,
            args.result_configmap_namespace,
            str(exc),
        )
        print(f"Error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
