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

"""Delete a published backend artifact from the current KitOps/OCI plane."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile
import sys


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Validate a backend artifact cleanup handle and print the requested cleanup target."
    )
    parser.add_argument(
        "--handle-json",
        required=True,
        help="Encoded cleanup handle JSON passed by the controller.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate and print the cleanup handle without performing any delete.",
    )
    return parser


def validate_handle(payload: str) -> dict:
    try:
        handle = json.loads(payload)
    except json.JSONDecodeError as exc:
        raise SystemExit(f"invalid cleanup handle JSON: {exc}") from exc

    kind = str(handle.get("kind", "")).strip()
    if kind != "BackendArtifact":
        raise SystemExit(f"unsupported cleanup handle kind: {kind!r}")

    backend = handle.get("backend")
    if not isinstance(backend, dict):
        raise SystemExit("cleanup handle backend payload must be an object")

    reference = str(backend.get("reference", "")).strip()
    if not reference:
        raise SystemExit("cleanup handle backend.reference must not be empty")

    artifact = handle.get("artifact")
    if artifact is not None and not isinstance(artifact, dict):
        raise SystemExit("cleanup handle artifact payload must be an object when provided")

    return handle


def registry_from_oci_reference(reference: str) -> str:
    clean_reference = reference.strip()
    if not clean_reference:
        raise SystemExit("cleanup reference must not be empty")

    without_digest = clean_reference.split("@", 1)[0]
    registry, separator, repository = without_digest.partition("/")
    if not separator or not registry or not repository:
        raise SystemExit(f"cleanup reference must include registry host and repository path: {reference!r}")
    if "." not in registry and ":" not in registry and registry != "localhost":
        raise SystemExit(f"cleanup reference must include an explicit registry host: {reference!r}")
    return registry


def kit_connection_flags() -> list[str]:
    if os.getenv("AI_MODELS_OCI_INSECURE", "").strip().lower() in ("1", "true", "yes", "on"):
        return ["--tls-verify=false"]
    return []


def kit_runtime_environment(config_dir: str) -> dict[str, str]:
    environment = os.environ.copy()
    environment["HOME"] = config_dir
    environment["DOCKER_CONFIG"] = os.path.join(config_dir, "docker")
    Path(environment["DOCKER_CONFIG"]).mkdir(parents=True, exist_ok=True)

    ca_file = os.getenv("AI_MODELS_OCI_CA_FILE", "").strip()
    if ca_file:
        environment["SSL_CERT_FILE"] = ca_file

    return environment


def kit_command(config_dir: str, *args: str, include_log_level: bool = True) -> list[str]:
    command = ["kit", "--config", config_dir, "--progress", "none"]
    if include_log_level:
        command.extend(["--log-level", "error"])
    command.extend(args)
    return command


def require_command_success(command: list[str], *, stdin_text: str | None = None, extra_env: dict[str, str], error_prefix: str) -> None:
    completed = subprocess.run(
        command,
        input=stdin_text,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        env=extra_env,
    )
    if completed.returncode == 0:
        return

    details = completed.stderr.strip() or completed.stdout.strip() or f"exit code {completed.returncode}"
    raise SystemExit(f"{error_prefix}: {details}")


def configure_kit_cli(config_dir: str) -> None:
    require_command_success(
        kit_command(config_dir, "version", "--show-update-notifications=false"),
        extra_env=kit_runtime_environment(config_dir),
        error_prefix="failed to initialize KitOps CLI config",
    )


def login_kit_registry(config_dir: str, reference: str) -> None:
    username = os.getenv("AI_MODELS_OCI_USERNAME", "").strip()
    password = os.getenv("AI_MODELS_OCI_PASSWORD", "")
    if not username:
        raise SystemExit("AI_MODELS_OCI_USERNAME must not be empty")
    if password == "":
        raise SystemExit("AI_MODELS_OCI_PASSWORD must not be empty")

    registry = registry_from_oci_reference(reference)
    require_command_success(
        kit_command(
            config_dir,
            "login",
            registry,
            "-u",
            username,
            "--password-stdin",
            *kit_connection_flags(),
        ),
        stdin_text=password,
        extra_env=kit_runtime_environment(config_dir),
        error_prefix=f"failed to login to OCI registry {registry}",
    )


def remove_remote_artifact(config_dir: str, reference: str) -> None:
    require_command_success(
        kit_command(
            config_dir,
            "remove",
            "--remote",
            reference,
            *kit_connection_flags(),
        ),
        extra_env=kit_runtime_environment(config_dir),
        error_prefix=f"failed to remove remote artifact {reference}",
    )


def main() -> int:
    args = build_parser().parse_args()
    handle = validate_handle(args.handle_json)

    print("Backend artifact cleanup request:")
    print(f"  reference: {handle['backend']['reference']}")
    artifact = handle.get("artifact")
    if isinstance(artifact, dict):
        if artifact.get("kind"):
            print(f"  artifact kind: {artifact['kind']}")
        if artifact.get("uri"):
            print(f"  artifact uri: {artifact['uri']}")
        if artifact.get("digest"):
            print(f"  artifact digest: {artifact['digest']}")

    if args.dry_run:
        print("Dry-run mode enabled; no cleanup performed.")
        return 0

    with tempfile.TemporaryDirectory(prefix="ai-model-kitops-cleanup-") as config_dir:
        configure_kit_cli(config_dir)
        login_kit_registry(config_dir, handle["backend"]["reference"])
        remove_remote_artifact(config_dir, handle["backend"]["reference"])

    print("Cleanup completed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
