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
Bootstrap the internal machine-oriented MLflow user for oidc-auth deployments.

The browser login path goes through Dex OIDC, but phase-1 still needs a stable
non-browser account for:
- ServiceMonitor basic auth;
- in-cluster import Jobs;
- break-glass operational access.

This script creates or updates that internal account directly in the OIDC auth
store so the rest of the module can keep using Basic auth for machine flows.
"""

from __future__ import annotations

import argparse
import os
import sys

from ai_models_backend_runtime import env, render_db_uri_from_env


def build_parser() -> argparse.ArgumentParser:
    return argparse.ArgumentParser(
        description="Bootstrap the internal ai-models machine account in the MLflow OIDC auth store."
    )


def main() -> int:
    build_parser().parse_args()
    username = env("AI_MODELS_AUTH_MACHINE_USERNAME", "").strip()
    password = env("AI_MODELS_AUTH_MACHINE_PASSWORD", "").strip()
    users_db_uri = env("OIDC_USERS_DB_URI", "").strip() or render_db_uri_from_env()

    if not username:
        print("AI_MODELS_AUTH_MACHINE_USERNAME is required.", file=sys.stderr)
        return 1
    if not password:
        print("AI_MODELS_AUTH_MACHINE_PASSWORD is required.", file=sys.stderr)
        return 1
    os.environ["OIDC_USERS_DB_URI"] = users_db_uri

    from mlflow_oidc_auth.store import store
    from mlflow_oidc_auth.user import create_user

    created, message = create_user(
        username=username,
        display_name="ai-models internal service account",
        is_admin=True,
        is_service_account=True,
    )
    print(message)

    store.update_user(
        username=username,
        password=password,
        is_admin=True,
        is_service_account=True,
    )

    print(
        f"{'Created' if created else 'Updated'} internal MLflow OIDC service account "
        f"'{username}'."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
