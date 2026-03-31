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
import configparser
import os
from pathlib import Path
from urllib.parse import quote_plus
from urllib.parse import urlparse

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


def parse_s3_uri(uri: str) -> tuple[str, str]:
    parsed = urlparse(uri)
    if parsed.scheme != "s3":
        raise ValueError(f"unsupported artifact URI scheme for cleanup: {uri}")
    if not parsed.netloc:
        raise ValueError(f"S3 artifact URI must include a bucket: {uri}")
    return parsed.netloc, parsed.path.lstrip("/")


def resolve_s3_addressing_style() -> str | None:
    explicit = env("AI_MODELS_S3_ADDRESSING_STYLE", "").strip().lower()
    if explicit in {"path", "virtual", "auto"}:
        return explicit

    config_path = env("AWS_CONFIG_FILE", "").strip()
    if not config_path:
        return None

    path = Path(config_path)
    if not path.is_file():
        return None

    parser = configparser.RawConfigParser()
    try:
        parser.read(path, encoding="utf-8")
    except Exception:
        return None

    if not parser.has_option("default", "s3"):
        return None

    s3_block = parser.get("default", "s3", fallback="")
    for raw_line in s3_block.splitlines():
        line = raw_line.strip()
        if not line or not line.startswith("addressing_style"):
            continue
        _, _, value = line.partition("=")
        style = value.strip().lower()
        if style in {"path", "virtual", "auto"}:
            return style

    return None


def build_s3_client():
    import boto3
    from botocore.config import Config

    apply_s3_environment_bridge()

    session = boto3.session.Session(
        region_name=env("AWS_REGION", env("AWS_DEFAULT_REGION", "us-east-1"))
    )
    client_kwargs: dict[str, object] = {}

    endpoint = env("AI_MODELS_S3_ENDPOINT_URL", "").strip()
    if endpoint:
        client_kwargs["endpoint_url"] = endpoint

    if env_bool("AI_MODELS_S3_IGNORE_TLS", False):
        client_kwargs["verify"] = False

    addressing_style = resolve_s3_addressing_style()
    if addressing_style:
        client_kwargs["config"] = Config(s3={"addressing_style": addressing_style})

    return session.client("s3", **client_kwargs)


def delete_s3_prefix(uri: str) -> dict[str, int | str]:
    bucket, prefix = parse_s3_uri(uri)
    s3 = build_s3_client()
    paginator = s3.get_paginator("list_objects_v2")

    before = 0
    keys: list[str] = []
    for page in paginator.paginate(Bucket=bucket, Prefix=prefix):
        contents = page.get("Contents", [])
        before += len(contents)
        keys.extend(obj["Key"] for obj in contents if "Key" in obj)

    deleted = 0
    for index in range(0, len(keys), 1000):
        batch = [{"Key": key} for key in keys[index : index + 1000]]
        response = s3.delete_objects(
            Bucket=bucket,
            Delete={"Objects": batch, "Quiet": True},
        )
        errors = response.get("Errors", [])
        if errors:
            raise RuntimeError(f"failed to delete S3 objects under {uri}: {errors}")
        deleted += len(batch)

    after = 0
    for page in paginator.paginate(Bucket=bucket, Prefix=prefix):
        after += len(page.get("Contents", []))

    return {
        "bucket": bucket,
        "prefix": prefix,
        "keys_before": before,
        "keys_deleted": deleted,
        "keys_after": after,
    }


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
