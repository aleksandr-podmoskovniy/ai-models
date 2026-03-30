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
import re
from pathlib import Path

def split_documents(text: str) -> list[str]:
    documents = re.split(r"(?m)^---\s*$", text)
    return [document for document in documents if document.strip()]


def parse_postgres_document(document: str) -> tuple[str, list[dict[str, str]]] | None:
    kind = ""
    metadata_name = "<unknown>"
    users: list[dict[str, str]] = []
    current_user: dict[str, str] | None = None
    in_metadata = False
    in_spec = False
    in_users = False

    for raw_line in document.splitlines():
        if not raw_line.strip() or raw_line.lstrip().startswith("#"):
            continue

        indent = len(raw_line) - len(raw_line.lstrip(" "))
        line = raw_line.strip()

        if indent == 0:
            in_metadata = line == "metadata:"
            in_spec = line == "spec:"
            if line.startswith("kind:"):
                kind = line.split(":", 1)[1].strip()
            if in_users and current_user is not None:
                users.append(current_user)
                current_user = None
            in_users = False
            continue

        if in_metadata and indent == 2 and line.startswith("name:"):
            metadata_name = line.split(":", 1)[1].strip().strip('"')
            continue

        if in_spec and indent == 2 and line == "users:":
            in_users = True
            continue

        if in_users:
            if indent <= 2:
                if current_user is not None:
                    users.append(current_user)
                    current_user = None
                in_users = False
                continue

            if indent == 4 and line.startswith("- "):
                if current_user is not None:
                    users.append(current_user)
                current_user = {}
                item = line[2:].strip()
                if ":" in item:
                    key, value = item.split(":", 1)
                    current_user[key.strip()] = value.strip().strip('"')
                continue

            if indent >= 6 and current_user is not None and ":" in line:
                key, value = line.split(":", 1)
                current_user[key.strip()] = value.strip().strip('"')

    if in_users and current_user is not None:
        users.append(current_user)

    if kind != "Postgres":
        return None

    return metadata_name, users


def validate_postgres_users(path: Path) -> list[str]:
    errors: list[str] = []
    content = path.read_text(encoding="utf-8")
    for document in split_documents(content):
        parsed = parse_postgres_document(document)
        if parsed is None:
            continue

        name, users = parsed
        if not users:
            errors.append(f"{path.name}: Postgres/{name} has no users")
            continue

        for idx, user in enumerate(users):
            password = user.get("password", "")
            hashed_password = user.get("hashedPassword", "")
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
