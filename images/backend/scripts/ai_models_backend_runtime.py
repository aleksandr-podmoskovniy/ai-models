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

from __future__ import annotations

import argparse
import os
from pathlib import Path
from urllib.parse import quote_plus

SYSTEM_CA_BUNDLE = Path("/etc/ssl/certs/ca-certificates.crt")
MERGED_S3_CA_BUNDLE = Path("/tmp/ai-models-s3-ca-bundle.crt")


def env(name: str, default: str = "") -> str:
    return os.environ.get(name, default)


def env_bool(name: str, default: bool = False) -> bool:
    raw = os.environ.get(name)
    if raw is None:
        return default
    return raw.lower() in {"1", "true", "yes", "on"}


def render_db_uri_from_env() -> str:
    password = quote_plus(os.environ["AI_MODELS_DATABASE_PASSWORD"])
    user = quote_plus(os.environ["AI_MODELS_DATABASE_USER"])
    host = os.environ["AI_MODELS_DATABASE_HOST"]
    port = os.environ["AI_MODELS_DATABASE_PORT"]
    database = os.environ["AI_MODELS_DATABASE_NAME"]
    sslmode = os.environ["AI_MODELS_DATABASE_SSLMODE"]

    return (
        f"postgresql+psycopg2://{user}:{password}@{host}:{port}/{database}?sslmode={sslmode}"
    )


def render_auth_db_uri_from_env() -> str:
    password = quote_plus(os.environ["AI_MODELS_DATABASE_PASSWORD"])
    user = quote_plus(os.environ["AI_MODELS_DATABASE_USER"])
    host = os.environ["AI_MODELS_DATABASE_HOST"]
    port = os.environ["AI_MODELS_DATABASE_PORT"]
    database = os.environ.get("AI_MODELS_AUTH_DATABASE_NAME") or f"{os.environ['AI_MODELS_DATABASE_NAME']}-auth"
    sslmode = os.environ["AI_MODELS_DATABASE_SSLMODE"]

    return (
        f"postgresql+psycopg2://{user}:{password}@{host}:{port}/{database}?sslmode={sslmode}"
    )


def apply_s3_ca_trust() -> None:
    ca_file = env("AI_MODELS_S3_CA_FILE", "").strip()
    if not ca_file:
        return

    ca_path = Path(ca_file)
    if not ca_path.is_file() or ca_path.stat().st_size == 0:
        return

    source_paths: list[Path] = []
    seen_sources: set[str] = set()

    def add_source(path: Path) -> None:
        try:
            resolved = str(path.resolve())
        except OSError:
            return
        if resolved in seen_sources:
            return
        if not path.is_file() or path.stat().st_size == 0:
            return
        seen_sources.add(resolved)
        source_paths.append(path)

    add_source(SYSTEM_CA_BUNDLE)
    for env_name in ("SSL_CERT_FILE", "REQUESTS_CA_BUNDLE", "CURL_CA_BUNDLE", "AWS_CA_BUNDLE"):
        current = env(env_name, "").strip()
        if current:
            add_source(Path(current))
    add_source(ca_path)

    if not source_paths:
        return

    MERGED_S3_CA_BUNDLE.parent.mkdir(parents=True, exist_ok=True)
    with MERGED_S3_CA_BUNDLE.open("wb") as merged:
        for source in source_paths:
            merged.write(source.read_bytes().rstrip(b"\n"))
            merged.write(b"\n")

    merged_path = str(MERGED_S3_CA_BUNDLE)
    os.environ["AWS_CA_BUNDLE"] = merged_path
    os.environ["SSL_CERT_FILE"] = merged_path
    os.environ["REQUESTS_CA_BUNDLE"] = merged_path
    os.environ["CURL_CA_BUNDLE"] = merged_path


def apply_s3_environment_bridge() -> None:
    endpoint = env("AI_MODELS_S3_ENDPOINT_URL", "")
    ignore_tls = env("AI_MODELS_S3_IGNORE_TLS", "")

    if endpoint and not env("MLFLOW_S3_ENDPOINT_URL", ""):
        os.environ["MLFLOW_S3_ENDPOINT_URL"] = endpoint
    if ignore_tls and not env("MLFLOW_S3_IGNORE_TLS", ""):
        os.environ["MLFLOW_S3_IGNORE_TLS"] = ignore_tls
    apply_s3_ca_trust()


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Shared runtime helper for ai-models backend image."
    )
    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser(
        "render-db-uri",
        help="Render the SQLAlchemy PostgreSQL URI from AI_MODELS_DATABASE_* env vars.",
    )
    subparsers.add_parser(
        "render-auth-db-uri",
        help="Render the SQLAlchemy PostgreSQL URI for the separate MLflow OIDC auth database.",
    )
    return parser


def main() -> int:
    args = build_parser().parse_args()

    if args.command == "render-db-uri":
        print(render_db_uri_from_env())
        return 0
    if args.command == "render-auth-db-uri":
        print(render_auth_db_uri_from_env())
        return 0

    raise RuntimeError(f"unsupported command: {args.command}")


if __name__ == "__main__":
    raise SystemExit(main())
