## 1. Current phase

Governance hardening поверх live `ai-models` baseline. Это не product/runtime
slice, а dedicated workflow-governance continuation bundle.

## 2. Orchestration

`solo`

Причина:

- задача целиком лежит на repo-local workflow surface;
- цель — tightening, transferability и consistency review инструкций, а не
  проектирование нового runtime/API behavior;
- основной риск здесь в качестве границ между reusable core и overlays, а не
  в multi-agent exploration.

## 3. Slices

### Slice 1. Reframe the canonical governance bundle

Цель:

- обновить существующий canonical active bundle под переносимый multi-agent
  baseline и зафиксировать acceptance criteria.

Файлы/каталоги:

- `plans/active/governance-surface-sync/TASK.ru.md`
- `plans/active/governance-surface-sync/PLAN.ru.md`

Проверки:

- manual consistency review against `AGENTS.md` and `.codex/README.md`

Артефакт результата:

- актуальный bundle, который явно описывает reusable core, overlays, touched
  instruction surfaces и governance validations.

### Slice 2. Tighten reusable core and overlay boundaries

Цель:

- переписать instruction surface так, чтобы generic core skills/agents были
  переносимыми, а `ai-models`-specific knowledge оставался в overlays;
- сделать overlay split и porting contract machine-checkable.

Файлы/каталоги:

- `AGENTS.md`
- `.codex/README.md`
- `.codex/governance-inventory.json`
- `.codex/agents/*.toml`
- `.agents/skills/*`
- `tools/check-codex-governance.py`

Проверки:

- manual consistency review of precedence chain
- targeted drift scan for domain-specific wording in core skills/agents

Артефакт результата:

- repo-local governance surface с явным capability split:
  reusable core vs project-specific overlays;
- lintable porting contract for copying the baseline into another module repo.

### Slice 3. Sync workflow docs and review surfaces

Цель:

- выровнять workflow docs с новым baseline так, чтобы task intake, review и
  handoff одинаково трактовали переносимый multi-agent system;
- убрать необходимость “помнить на словах”, что именно нельзя слепо копировать.

Файлы/каталоги:

- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/TASK_TEMPLATE.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`
- `plans/README.md`
- optional `docs/development/REPO_LAYOUT.ru.md`

Проверки:

- manual consistency review across touched docs

Артефакт результата:

- workflow docs, которые не противоречат precedence surface и одинаково
  описывают governance guardrails.

## 4. Rollback point

После Slice 1: canonical bundle already updated, but instruction surfaces ещё не
переписаны.

## 5. Final validation

- `make lint-codex-governance`
- `git diff --check`
- manual review of touched governance layers as one instruction system
