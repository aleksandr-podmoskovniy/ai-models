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

import json
import os
import re
import sys
from pathlib import Path


def normalize_item(line: str) -> str:
    match = re.search(r"`([^`]+)`", line)
    if match:
        return match.group(1)
    match = re.match(r"-\s*([A-Za-z0-9_.\\/-]+)", line.strip())
    if match:
        return match.group(1)
    return line.strip()


def extract_bullets_after_label(text: str, label: str) -> list[str]:
    lines = text.splitlines()
    for idx, line in enumerate(lines):
        if line.strip() != label:
            continue
        items: list[str] = []
        for candidate in lines[idx + 1 :]:
            stripped = candidate.strip()
            if not stripped:
                if items:
                    break
                continue
            if stripped.startswith("- "):
                items.append(normalize_item(stripped))
                continue
            break
        return items
    raise ValueError(f"missing section label: {label}")


def extract_front_matter_name(text: str) -> str | None:
    match = re.search(r"^---\s*\nname:\s*([^\n]+)\n", text, re.MULTILINE)
    return match.group(1).strip() if match else None


def extract_toml_value(text: str, key: str) -> str | None:
    match = re.search(rf'^{re.escape(key)}\s*=\s*"([^"]+)"\s*$', text, re.MULTILINE)
    return match.group(1) if match else None


def check_ordered_substrings(text: str, items: list[str], label: str, failures: list[str]) -> None:
    cursor = -1
    for item in items:
        pos = text.find(item)
        if pos < 0:
            failures.append(f"{label}: missing `{item}`")
            return
        if pos <= cursor:
            failures.append(f"{label}: wrong order for `{item}`")
            return
        cursor = pos


def main() -> int:
    root = Path(os.environ.get("ROOT", Path(__file__).resolve().parents[1]))
    inventory_path = root / ".codex/governance-inventory.json"
    inventory = json.loads(inventory_path.read_text(encoding="utf-8"))
    failures: list[str] = []

    def read(rel_path: str) -> str:
        path = root / rel_path
        if not path.exists():
            failures.append(f"missing file: {rel_path}")
            return ""
        return path.read_text(encoding="utf-8")

    print("==> codex governance")

    agents_md = read("AGENTS.md")
    codex_readme = read(".codex/README.md")

    if agents_md:
        check_ordered_substrings(
            agents_md,
            inventory["precedence"],
            "AGENTS.md precedence",
            failures,
        )
    if codex_readme:
        check_ordered_substrings(
            codex_readme,
            inventory["precedence"],
            ".codex/README.md precedence",
            failures,
        )

    for spec in inventory["required_files"]:
        text = read(spec["path"])
        for needle in spec["must_include"]:
            if text and needle not in text:
                failures.append(f"{spec['path']}: missing required text `{needle}`")

    expected_skill_names = [spec["name"] for spec in inventory["skills"]]
    actual_skill_names = sorted(
        path.name for path in (root / ".agents/skills").iterdir() if path.is_dir()
    )
    if actual_skill_names != sorted(expected_skill_names):
        failures.append(
            "skill inventory mismatch: expected "
            f"{sorted(expected_skill_names)} got {actual_skill_names}"
        )

    for spec in inventory["skills"]:
        text = read(spec["path"])
        if not text:
            continue
        actual_name = extract_front_matter_name(text)
        if actual_name != spec["name"]:
            failures.append(
                f"{spec['path']}: front matter name `{actual_name}` != `{spec['name']}`"
            )
        for needle in spec["must_include"]:
            if needle not in text:
                failures.append(f"{spec['path']}: missing required text `{needle}`")

    expected_agent_names = [spec["name"] for spec in inventory["agents"]]
    actual_agent_names = sorted(
        extract_toml_value(path.read_text(encoding="utf-8"), "name") or path.stem
        for path in sorted((root / ".codex/agents").glob("*.toml"))
    )
    if actual_agent_names != sorted(expected_agent_names):
        failures.append(
            "agent inventory mismatch: expected "
            f"{sorted(expected_agent_names)} got {actual_agent_names}"
        )

    for spec in inventory["agents"]:
        text = read(spec["path"])
        if not text:
            continue
        actual_name = extract_toml_value(text, "name")
        if actual_name != spec["name"]:
            failures.append(
                f"{spec['path']}: name `{actual_name}` != `{spec['name']}`"
            )
        sandbox_mode = extract_toml_value(text, "sandbox_mode")
        if sandbox_mode != spec["sandbox_mode"]:
            failures.append(
                f"{spec['path']}: sandbox_mode `{sandbox_mode}` != `{spec['sandbox_mode']}`"
            )
        for needle in spec["must_include"]:
            if needle not in text:
                failures.append(f"{spec['path']}: missing required text `{needle}`")

    if codex_readme:
        core_skills = extract_bullets_after_label(codex_readme, "Core skills:")
        if core_skills != inventory["core_skills"]:
            failures.append(
                f".codex/README.md: core skills {core_skills} != {inventory['core_skills']}"
            )

        core_read_only_agents = extract_bullets_after_label(
            codex_readme, "Core read-only agents:"
        )
        if core_read_only_agents != inventory["core_read_only_agents"]:
            failures.append(
                ".codex/README.md: core read-only agents "
                f"{core_read_only_agents} != {inventory['core_read_only_agents']}"
            )

        overlay_skills = extract_bullets_after_label(
            codex_readme, "Project-specific overlay skills:"
        )
        if overlay_skills != inventory["overlay_skills"]:
            failures.append(
                ".codex/README.md: overlay skills "
                f"{overlay_skills} != {inventory['overlay_skills']}"
            )

        overlay_agents = extract_bullets_after_label(
            codex_readme, "Project-specific overlay agents:"
        )
        if overlay_agents != inventory["overlay_agents"]:
            failures.append(
                ".codex/README.md: overlay agents "
                f"{overlay_agents} != {inventory['overlay_agents']}"
            )

        write_capable_agents = extract_bullets_after_label(
            codex_readme, "Write-capable agents:"
        )
        if write_capable_agents != inventory["write_capable_agents"]:
            failures.append(
                ".codex/README.md: write-capable agents "
                f"{write_capable_agents} != {inventory['write_capable_agents']}"
            )

        porting_review_files = extract_bullets_after_label(
            codex_readme, "Files that must be reviewed and rewritten during porting:"
        )
        if porting_review_files != inventory["porting_review_files"]:
            failures.append(
                ".codex/README.md: porting review files "
                f"{porting_review_files} != {inventory['porting_review_files']}"
            )

    overlap = set(inventory["core_read_only_agents"]) & set(inventory["write_capable_agents"])
    if overlap:
        failures.append(f"inventory overlap between read-only and write-capable agents: {sorted(overlap)}")

    skill_overlap = set(inventory["core_skills"]) & set(inventory["overlay_skills"])
    if skill_overlap:
        failures.append(f"inventory overlap between core and overlay skills: {sorted(skill_overlap)}")

    agent_overlap = (
        set(inventory["core_read_only_agents"]) | set(inventory["write_capable_agents"])
    ) & set(inventory["overlay_agents"])
    if agent_overlap:
        failures.append(f"inventory overlap between core and overlay agents: {sorted(agent_overlap)}")

    unknown_overlay_skills = set(inventory["overlay_skills"]) - set(expected_skill_names)
    if unknown_overlay_skills:
        failures.append(f"inventory overlay skills missing from skills inventory: {sorted(unknown_overlay_skills)}")

    unknown_overlay_agents = set(inventory["overlay_agents"]) - set(expected_agent_names)
    if unknown_overlay_agents:
        failures.append(f"inventory overlay agents missing from agents inventory: {sorted(unknown_overlay_agents)}")

    for rel_path in inventory["porting_review_files"]:
        if not (root / rel_path).exists():
            failures.append(f"inventory porting review file missing: {rel_path}")

    if "task_framer" in inventory["core_read_only_agents"]:
        failures.append("inventory: task_framer must not be in core read-only agents")
    if "task_framer" not in inventory["write_capable_agents"]:
        failures.append("inventory: task_framer must stay write-capable")
    if "module_implementer" not in inventory["write_capable_agents"]:
        failures.append("inventory: module_implementer must stay write-capable")

    if failures:
        for failure in failures:
            print(failure, file=sys.stderr)
        return 1

    print("codex governance OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
