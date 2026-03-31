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

from mlflow.store.db import utils as db_utils
from ai_models_backend_runtime import render_db_uri_from_env


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Initialize and upgrade the ai-models MLflow metadata database."
    )
    parser.add_argument(
        "database_uri",
        nargs="?",
        help="Optional explicit SQLAlchemy database URI. Defaults to AI_MODELS_DATABASE_* env vars.",
    )
    return parser


def main() -> int:
    args = build_parser().parse_args()
    backend_store_uri = args.database_uri or render_db_uri_from_env()
    engine = db_utils.create_sqlalchemy_engine_with_retry(backend_store_uri)
    try:
        db_utils._safe_initialize_tables(engine)
        db_utils._upgrade_db(engine)
    finally:
        engine.dispose()

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
