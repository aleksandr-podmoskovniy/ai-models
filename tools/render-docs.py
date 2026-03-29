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
from pathlib import Path
import sys

MARKER = "<!-- SCHEMA -->"
DOCS = [
    Path("docs/CONFIGURATION.md"),
    Path("docs/CONFIGURATION.ru.md"),
]


def validate_docs() -> list[str]:
    errors: list[str] = []
    for doc in DOCS:
        if not doc.exists():
            errors.append(f"missing documentation file: {doc}")
            continue
        text = doc.read_text(encoding="utf-8")
        if MARKER not in text:
            errors.append(f"missing schema marker in {doc}")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()

    errors = validate_docs()
    if errors:
        for err in errors:
            print(err, file=sys.stderr)
        return 1

    if args.check:
        print("docs markers are valid")
    else:
        print("no generated docs step is wired yet; markers are valid")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
