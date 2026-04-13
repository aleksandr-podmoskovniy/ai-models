# PLAN

## Current phase

Workflow governance maintenance for the repo-local Codex surface that should
serve as a reusable baseline for DKP module repos.

## Orchestration

- mode: `solo`
- reason:
  - task is repo-local governance/docs refactor;
  - current runtime policy does not allow silent delegation without an explicit
    user request;
  - the right signal here comes from a direct repo review of the instructions.

## Slice 1. Audit current workflow surface

Цель:

- снять текущие gaps и overlaps между:
  - `AGENTS.md`
  - `.codex/README.md`
  - `.agents/skills/*`
  - `.codex/agents/*`

Артефакты:

- findings captured in this bundle

Проверки:

- manual consistency pass over the listed files

## Slice 2. Encode reusable engineering doctrine

Цель:

- жёстко зафиксировать:
  - boundary discipline
  - long-context resilience
  - systematic testing methodology
  - reusable-core vs project-overlay split

Артефакты:

- updated root workflow docs and relevant skills

Проверки:

- repo-local consistency pass after edits

## Slice 3. Tighten skill boundaries and agent profiles

Цель:

- сделать workflow rules более жёсткими и менее двусмысленными;
- выровнять skills по responsibility split;
- убрать или переписать weak/duplicated guidance;
- выровнять agent profiles с обновлённым doctrine.

Артефакты:

- updated `AGENTS.md`
- updated `.codex/README.md`
- updated relevant `SKILL.md`
- updated relevant `.codex/agents/*.toml`

Проверки:

- repo-local consistency pass after edits

## Slice 4. Add a machine-checkable governance guardrail

Цель:

- перестать полагаться только на prose и manual reread при проверке
  repo-local workflow surface;
- зафиксировать inventory для ключевых governance invariants;
- добавить lint в normal verify path.

Артефакты:

- `.codex/governance-inventory.json`
- `tools/check-codex-governance.py`
- updated `Makefile`
- synced root workflow docs if command surface changes

Проверки:

- `python3 ./tools/check-codex-governance.py`
- `make lint-codex-governance`

## Slice 5. Align `docs/development` with the governance baseline

Цель:

- убрать drift между `docs/development` и repo-local instruction surface;
- выровнять workflow docs, bundle template и review checklist с текущим
  governance doctrine;
- расширить machine-checkable guardrail на ключевые workflow docs.

Артефакты:

- updated `docs/development/CODEX_WORKFLOW.ru.md`
- updated `docs/development/TASK_TEMPLATE.ru.md`
- updated `docs/development/REVIEW_CHECKLIST.ru.md`
- updated `docs/development/REPO_LAYOUT.ru.md`
- updated `plans/README.md` if workflow-surface scope expands there
- updated governance inventory/checker if new invariants are encoded

Проверки:

- `python3 ./tools/check-codex-governance.py`
- `make lint-codex-governance`
- repo-local consistency pass across workflow docs and root governance docs

## Slice 6. Final review of the instruction set

Цель:

- провести жёсткий review обновлённого workflow surface как engineering
  artifact, а не как prose.

Артефакты:

- updated bundle notes if implementation drift is found

Проверки:

- `git diff --check`
- targeted manual review against:
  - duplication
  - ambiguity
  - portability as reusable DKP-module baseline
  - missing execution discipline

## Rollback point

Если tightening окажется неудачным:

1. откатить изменения только в workflow docs and skills;
2. оставить product/runtime tree нетронутым;
3. вернуть bundle к audit-only state.

## Final validation

- `make lint-codex-governance`
- `git diff --check`
- manual review of:
  - `AGENTS.md`
  - `.codex/README.md`
  - touched `.agents/skills/*`
  - touched `.codex/agents/*`
