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

import sys
from pathlib import Path

import yaml


def iter_yaml_documents(path: Path):
    with path.open("r", encoding="utf-8") as stream:
        for document in yaml.safe_load_all(stream):
            if document:
                yield document


def validate_postgres_users(path: Path) -> list[str]:
    errors: list[str] = []
    for document in iter_yaml_documents(path):
        if document.get("kind") != "Postgres":
            continue

        metadata = document.get("metadata") or {}
        name = metadata.get("name", "<unknown>")
        users = (((document.get("spec") or {}).get("users")) or [])
        if not users:
            errors.append(f"{path.name}: Postgres/{name} has no users")
            continue

        for idx, user in enumerate(users):
            password = (user or {}).get("password", "")
            hashed_password = (user or {}).get("hashedPassword", "")
            if not password and not hashed_password:
                errors.append(
                    f"{path.name}: Postgres/{name} user[{idx}] must set password or hashedPassword"
                )

    return errors


def main() -> int:
    renders_dir = Path(sys.argv[1])
    errors: list[str] = []

    for render in sorted(renders_dir.glob("helm-template-*.yaml")):
        errors.extend(validate_postgres_users(render))

    if errors:
        for error in errors:
            print(error, file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
