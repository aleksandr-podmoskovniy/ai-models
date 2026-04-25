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

import base64
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

LEGACY_RENDER_MARKERS = (
    "name: ai-models-backend-auth",
    "name: ai-models-backend-crypto",
    "name: ai-models-backend-trust-ca",
    "app.kubernetes.io/component: backend",
)

DISALLOWED_RENDER_MARKERS = (
    "kind: Postgres\n",
    "kind: PostgresClass\n",
)

MAX_PORT_NAME_LENGTH = 15
DMCR_RESTART_CHECKSUM_ANNOTATION = "ai.deckhouse.io/dmcr-pod-secret-checksum"


def _find_secret(
    documents: list[dict[object, object]], name: str
) -> dict[object, object] | None:
    for document in documents:
        if not isinstance(document, dict):
            continue
        metadata = document.get("metadata")
        if not isinstance(metadata, dict):
            continue
        if document.get("kind") == "Secret" and metadata.get("name") == name:
            return document
    return None


def _split_yaml_documents(content: str) -> list[str]:
    documents: list[str] = []
    current: list[str] = []
    for line in content.splitlines():
        if line.strip() == "---":
            if current:
                documents.append("\n".join(current))
                current = []
            continue
        current.append(line)
    if current:
        documents.append("\n".join(current))
    return documents


def _leading_spaces(line: str) -> int:
    return len(line) - len(line.lstrip(" "))


def _parse_inline_scalar(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
        if value[0] == '"':
            try:
                return json.loads(value)
            except json.JSONDecodeError:
                return value[1:-1]
        return value[1:-1].replace("''", "'")
    return value


def _skip_nested_block(lines: list[str], index: int, parent_indent: int) -> int:
    while index < len(lines):
        line = lines[index]
        if not line.strip():
            index += 1
            continue
        if _leading_spaces(line) <= parent_indent:
            break
        index += 1
    return index


def _parse_yaml_block_map(
    lines: list[str], index: int, parent_indent: int
) -> tuple[dict[str, object], int]:
    data: dict[str, object] = {}
    while index < len(lines):
        line = lines[index]
        if not line.strip():
            index += 1
            continue
        indent = _leading_spaces(line)
        if indent <= parent_indent:
            break
        if indent != parent_indent + 2:
            index += 1
            continue

        key, separator, rest = line.strip().partition(":")
        if separator == "":
            index += 1
            continue

        rest = rest.lstrip()
        if rest == "|":
            block_indent = indent + 2
            index += 1
            block_lines: list[str] = []
            while index < len(lines):
                block_line = lines[index]
                if not block_line.strip():
                    block_lines.append("")
                    index += 1
                    continue
                block_line_indent = _leading_spaces(block_line)
                if block_line_indent < block_indent:
                    break
                block_lines.append(block_line[block_indent:])
                index += 1
            data[key] = "\n".join(block_lines).rstrip("\n")
            continue

        if rest == "":
            nested, index = _parse_yaml_block_map(lines, index + 1, indent)
            data[key] = nested
            continue

        data[key] = _parse_inline_scalar(rest)
        index += 1

    return data, index


def _parse_render_documents(content: str) -> list[dict[object, object]]:
    documents: list[dict[object, object]] = []
    for raw_document in _split_yaml_documents(content):
        lines = raw_document.splitlines()
        kind = ""
        metadata: dict[str, object] = {}
        spec: dict[str, object] = {}
        string_data: dict[str, object] = {}
        index = 0
        while index < len(lines):
            line = lines[index]
            if not line.strip():
                index += 1
                continue
            if _leading_spaces(line) != 0:
                index += 1
                continue

            stripped = line.strip()
            if stripped.startswith("kind:"):
                kind = _parse_inline_scalar(stripped.split(":", 1)[1])
                index += 1
                continue
            if stripped == "metadata:":
                metadata, index = _parse_yaml_block_map(lines, index + 1, 0)
                continue
            if stripped == "spec:":
                spec, index = _parse_yaml_block_map(lines, index + 1, 0)
                continue
            if stripped == "stringData:":
                string_data, index = _parse_yaml_block_map(lines, index + 1, 0)
                continue
            index += 1

        if kind and metadata.get("name"):
            document: dict[str, object] = {"kind": kind, "metadata": metadata}
            if spec:
                document["spec"] = spec
            if string_data:
                document["stringData"] = string_data
            documents.append(document)
    return documents


def _parse_secret_documents(content: str) -> list[dict[object, object]]:
    return [
        document
        for document in _parse_render_documents(content)
        if document.get("kind") == "Secret"
    ]


def _find_document(
    documents: list[dict[object, object]], kind: str, name: str
) -> dict[object, object] | None:
    for document in documents:
        metadata = document.get("metadata")
        if not isinstance(metadata, dict):
            continue
        if document.get("kind") == kind and metadata.get("name") == name:
            return document
    return None


def _nested_string(mapping: object, *keys: str) -> str | None:
    current = mapping
    for key in keys:
        if not isinstance(current, dict):
            return None
        current = current.get(key)
    if not isinstance(current, str):
        return None
    return current


def _expect_string_data(
    path: Path, secret: dict[object, object], key: str
) -> str | None:
    string_data = secret.get("stringData")
    if not isinstance(string_data, dict):
        return None
    value = string_data.get(key)
    if not isinstance(value, str):
        return None
    return value


def _validate_dmcr_auth_consistency(path: Path, content: str) -> list[str]:
    errors: list[str] = []
    documents = _parse_secret_documents(content)

    auth_secret = _find_secret(documents, "ai-models-dmcr-auth")
    write_secret = _find_secret(documents, "ai-models-dmcr-auth-write")
    read_secret = _find_secret(documents, "ai-models-dmcr-auth-read")
    if not auth_secret or not write_secret or not read_secret:
        return errors

    auth_write_password = _expect_string_data(path, auth_secret, "write.password")
    auth_read_password = _expect_string_data(path, auth_secret, "read.password")
    write_password = _expect_string_data(path, write_secret, "password")
    read_password = _expect_string_data(path, read_secret, "password")
    write_username = _expect_string_data(path, write_secret, "username")
    read_username = _expect_string_data(path, read_secret, "username")
    write_config = _expect_string_data(path, write_secret, ".dockerconfigjson")
    read_config = _expect_string_data(path, read_secret, ".dockerconfigjson")

    if auth_write_password != write_password:
        errors.append(
            f"{path.name}: DMCR write auth password drift between ai-models-dmcr-auth and ai-models-dmcr-auth-write"
        )

    if auth_read_password != read_password:
        errors.append(
            f"{path.name}: DMCR read auth password drift between ai-models-dmcr-auth and ai-models-dmcr-auth-read"
        )

    write_htpasswd = _expect_string_data(path, auth_secret, "write.htpasswd")
    read_htpasswd = _expect_string_data(path, auth_secret, "read.htpasswd")
    write_checksum = _expect_string_data(path, auth_secret, "write.htpasswd.checksum")
    read_checksum = _expect_string_data(path, auth_secret, "read.htpasswd.checksum")

    if auth_write_password and write_checksum:
        expected_write_checksum = hashlib.sha256(
            auth_write_password.encode("utf-8")
        ).hexdigest()
        if write_checksum != expected_write_checksum:
            errors.append(
                f"{path.name}: ai-models-dmcr-auth write.htpasswd.checksum does not match write.password"
            )
    elif auth_write_password and not write_checksum:
        errors.append(
            f"{path.name}: ai-models-dmcr-auth is missing write.htpasswd.checksum"
        )

    if auth_read_password and read_checksum:
        expected_read_checksum = hashlib.sha256(
            auth_read_password.encode("utf-8")
        ).hexdigest()
        if read_checksum != expected_read_checksum:
            errors.append(
                f"{path.name}: ai-models-dmcr-auth read.htpasswd.checksum does not match read.password"
            )
    elif auth_read_password and not read_checksum:
        errors.append(
            f"{path.name}: ai-models-dmcr-auth is missing read.htpasswd.checksum"
        )

    if auth_write_password and write_htpasswd:
        errors.extend(
            _validate_htpasswd_entry(
                path,
                secret_name="ai-models-dmcr-auth",
                username="ai-models",
                password=auth_write_password,
                htpasswd_entry=write_htpasswd,
            )
        )
    if auth_read_password and read_htpasswd:
        errors.extend(
            _validate_htpasswd_entry(
                path,
                secret_name="ai-models-dmcr-auth",
                username="ai-models-reader",
                password=auth_read_password,
                htpasswd_entry=read_htpasswd,
            )
        )

    if write_config and write_username and write_password:
        errors.extend(
            _validate_registry_dockerconfig(
                path,
                secret_name="ai-models-dmcr-auth-write",
                dockerconfig_json=write_config,
                expected_username=write_username,
                expected_password=write_password,
            )
        )

    if read_config and read_username and read_password:
        errors.extend(
            _validate_registry_dockerconfig(
                path,
                secret_name="ai-models-dmcr-auth-read",
                dockerconfig_json=read_config,
                expected_username=read_username,
                expected_password=read_password,
            )
        )

    return errors


def _validate_dmcr_secret_restart_contract(path: Path, content: str) -> list[str]:
    errors: list[str] = []
    documents = _parse_render_documents(content)

    deployment = _find_document(documents, "Deployment", "dmcr")
    auth_secret = _find_document(documents, "Secret", "ai-models-dmcr-auth")
    tls_secret = _find_document(documents, "Secret", "ai-models-dmcr-tls")
    if not deployment or not auth_secret or not tls_secret:
        return errors

    auth_checksum = _nested_string(
        auth_secret, "metadata", "annotations", DMCR_RESTART_CHECKSUM_ANNOTATION
    )
    tls_checksum = _nested_string(
        tls_secret, "metadata", "annotations", DMCR_RESTART_CHECKSUM_ANNOTATION
    )
    deployment_checksum = _nested_string(
        deployment,
        "spec",
        "template",
        "metadata",
        "annotations",
        "checksum/secret",
    )

    if not auth_checksum:
        errors.append(
            f"{path.name}: ai-models-dmcr-auth is missing {DMCR_RESTART_CHECKSUM_ANNOTATION}"
        )
    if not tls_checksum:
        errors.append(
            f"{path.name}: ai-models-dmcr-tls is missing {DMCR_RESTART_CHECKSUM_ANNOTATION}"
        )
    if not deployment_checksum:
        errors.append(f"{path.name}: Deployment/dmcr is missing checksum/secret")
    if not auth_checksum or not tls_checksum or not deployment_checksum:
        return errors

    expected_deployment_checksum = hashlib.sha256(
        f"{auth_checksum}\n{tls_checksum}".encode("utf-8")
    ).hexdigest()
    if deployment_checksum != expected_deployment_checksum:
        errors.append(
            f"{path.name}: Deployment/dmcr checksum/secret does not match DMCR runtime Secret restart annotations"
        )

    for secret_name in (
        "ai-models-dmcr-auth-write",
        "ai-models-dmcr-auth-read",
        "ai-models-dmcr-ca",
    ):
        secret = _find_document(documents, "Secret", secret_name)
        if not secret:
            continue
        if _nested_string(
            secret, "metadata", "annotations", DMCR_RESTART_CHECKSUM_ANNOTATION
        ):
            errors.append(
                f"{path.name}: {secret_name} must not participate in Deployment/dmcr restart checksum"
            )

    return errors


def _validate_port_name_lengths(path: Path, content: str) -> list[str]:
    errors: list[str] = []
    lines = content.splitlines()
    ports_indent: int | None = None

    for line in lines:
        stripped = line.strip()
        if not stripped:
            continue

        indent = _leading_spaces(line)
        if ports_indent is not None and indent <= ports_indent:
            ports_indent = None

        if stripped == "ports:":
            ports_indent = indent
            continue

        if ports_indent is None:
            continue

        if indent != ports_indent + 2:
            continue

        if not stripped.startswith("- name:"):
            continue

        port_name = _parse_inline_scalar(stripped.split(":", 1)[1].strip())
        if len(port_name) > MAX_PORT_NAME_LENGTH:
            errors.append(
                f"{path.name}: port name {port_name!r} exceeds Kubernetes {MAX_PORT_NAME_LENGTH}-character limit"
            )

    return errors


def _validate_dmcr_secret_delete_rbac(path: Path, content: str) -> list[str]:
    errors: list[str] = []

    for raw_document in _split_yaml_documents(content):
        lines = raw_document.splitlines()
        kind = ""
        metadata_name = ""
        saw_secret_resources = False
        saw_secret_delete = False

        for line in lines:
            stripped = line.strip()
            if stripped.startswith("kind:"):
                kind = _parse_inline_scalar(stripped.split(":", 1)[1])
                continue
            if stripped.startswith("name:") and metadata_name == "":
                metadata_name = _parse_inline_scalar(stripped.split(":", 1)[1])
                continue
            if stripped == 'resources: ["secrets"]':
                saw_secret_resources = True
                continue
            if (
                saw_secret_resources
                and stripped.startswith("verbs:")
                and '"delete"' in stripped
            ):
                saw_secret_delete = True

        if kind == "Role" and metadata_name == "dmcr" and saw_secret_resources:
            if not saw_secret_delete:
                errors.append(
                    f"{path.name}: Role/dmcr must grant delete on secrets for dmcr garbage-collection request cleanup"
                )

    return errors


def _validate_node_cache_runtime_plane(path: Path, content: str) -> list[str]:
    errors: list[str] = []
    if '--node-cache-node-selector-json="' in content:
        errors.append(
            f"{path.name}: controller render must not wrap --node-cache-node-selector-json value in extra quotes inside the argument"
        )
    if '--node-cache-block-device-selector-json="' in content:
        errors.append(
            f"{path.name}: controller render must not wrap --node-cache-block-device-selector-json value in extra quotes inside the argument"
        )
    if "--node-cache-enabled=true" not in content:
        return errors

    if "kind: DaemonSet" in content and "name: ai-models-node-cache-runtime" in content:
        errors.append(
            f"{path.name}: node-cache-enabled render must not keep legacy DaemonSet/ai-models-node-cache-runtime after stable per-node runtime plane rollout"
        )
    if '--node-cache-shared-volume-size=' not in content:
        errors.append(
            f"{path.name}: controller render must pass --node-cache-shared-volume-size for the stable node-cache runtime PVC contract"
        )
    if "kind: ServiceAccount" not in content or "name: ai-models-node-cache-runtime" not in content:
        errors.append(
            f"{path.name}: node-cache-enabled render must include ServiceAccount/ai-models-node-cache-runtime"
        )
    if "kind: Role" not in content or "name: ai-models-node-cache-runtime" not in content:
        errors.append(
            f"{path.name}: node-cache-enabled render must include Role/ai-models-node-cache-runtime"
        )
    if "kind: RoleBinding" not in content or "name: ai-models-node-cache-runtime" not in content:
        errors.append(
            f"{path.name}: node-cache-enabled render must include RoleBinding/ai-models-node-cache-runtime"
        )
    return errors


def _validate_htpasswd_entry(
    path: Path,
    *,
    secret_name: str,
    username: str,
    password: str,
    htpasswd_entry: str,
) -> list[str]:
    htpasswd_bin = shutil.which("htpasswd")
    if not htpasswd_bin:
        return []

    with tempfile.NamedTemporaryFile("w", delete=False) as handle:
        handle.write(htpasswd_entry)
        temp_path = handle.name
    try:
        result = subprocess.run(
            [htpasswd_bin, "-vb", temp_path, username, password],
            capture_output=True,
            text=True,
            check=False,
        )
    finally:
        os.unlink(temp_path)

    if result.returncode == 0:
        return []

    detail = result.stderr.strip() or result.stdout.strip() or f"exit code {result.returncode}"
    return [
        f"{path.name}: {secret_name} htpasswd entry for {username} does not match projected password ({detail})"
    ]


def _validate_registry_dockerconfig(
    path: Path,
    *,
    secret_name: str,
    dockerconfig_json: str,
    expected_username: str,
    expected_password: str,
) -> list[str]:
    errors: list[str] = []
    try:
        payload = json.loads(dockerconfig_json)
    except json.JSONDecodeError as err:
        return [f"{path.name}: {secret_name} has invalid .dockerconfigjson: {err}"]

    auths = payload.get("auths")
    if not isinstance(auths, dict) or len(auths) != 1:
        return [
            f"{path.name}: {secret_name} must contain exactly one registry auth entry"
        ]

    registry_entry = next(iter(auths.values()))
    if not isinstance(registry_entry, dict):
        return [f"{path.name}: {secret_name} auth entry must be an object"]

    username = registry_entry.get("username")
    password = registry_entry.get("password")
    auth = registry_entry.get("auth")
    if username != expected_username or password != expected_password:
        errors.append(
            f"{path.name}: {secret_name} .dockerconfigjson does not match projected username/password"
        )

    expected_auth = base64.b64encode(
        f"{expected_username}:{expected_password}".encode("utf-8")
    ).decode("utf-8")
    if auth != expected_auth:
        errors.append(
            f"{path.name}: {secret_name} .dockerconfigjson auth field does not match projected username/password"
        )

    return errors


def validate_render(path: Path) -> list[str]:
    errors: list[str] = []
    content = path.read_text(encoding="utf-8")

    for marker in LEGACY_RENDER_MARKERS:
        if marker in content:
            errors.append(
                f"{path.name}: rendered output must not contain legacy backend/auth surface marker {marker!r}"
            )

    for marker in DISALLOWED_RENDER_MARKERS:
        if marker in content:
            errors.append(
                f"{path.name}: rendered output must not contain retired PostgreSQL shell marker {marker.strip()!r}"
            )

    errors.extend(_validate_port_name_lengths(path, content))
    errors.extend(_validate_dmcr_secret_delete_rbac(path, content))
    errors.extend(_validate_node_cache_runtime_plane(path, content))
    errors.extend(_validate_dmcr_auth_consistency(path, content))
    errors.extend(_validate_dmcr_secret_restart_contract(path, content))

    return errors


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: validate-renders.py <renders-dir>", file=sys.stderr)
        return 1

    renders_dir = Path(sys.argv[1])
    if not renders_dir.is_dir():
        print(f"renders directory not found: {renders_dir}", file=sys.stderr)
        return 1

    errors: list[str] = []
    for render in sorted(renders_dir.glob("helm-template-*.yaml")):
        errors.extend(validate_render(render))

    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
