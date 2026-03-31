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
from urllib.parse import quote_plus


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


def apply_s3_environment_bridge() -> None:
    endpoint = env("AI_MODELS_S3_ENDPOINT_URL", "")
    ignore_tls = env("AI_MODELS_S3_IGNORE_TLS", "")

    if endpoint and not env("MLFLOW_S3_ENDPOINT_URL", ""):
        os.environ["MLFLOW_S3_ENDPOINT_URL"] = endpoint
    if ignore_tls and not env("MLFLOW_S3_IGNORE_TLS", ""):
        os.environ["MLFLOW_S3_IGNORE_TLS"] = ignore_tls


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
