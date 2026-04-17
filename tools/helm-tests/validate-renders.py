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
