## 1. Current phase

Governance follow-up after runtime-baseline reset. Это отдельный workflow task,
а не product/runtime implementation slice.

## 2. Orchestration

`solo`

Причина:

- scope ограничен repo-local governance surfaces;
- runtime/API behavior не проектируется заново;
- основной риск здесь в consistency review, а не в multi-agent exploration.

## 3. Slices

### Slice 1. Audit current governance drift

Цель:

- зафиксировать расхождения между live repo baseline and governance surfaces.

Файлы/каталоги:

- `AGENTS.md`
- `.codex/README.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `plans/active/governance-surface-sync/*`

Проверки:

- manual consistency review

Артефакт результата:

- explicit drift list to fix.

### Slice 2. Rewrite stale phase/workflow narrative

Цель:

- перевести governance surfaces на current publication/runtime-first roadmap.

Файлы/каталоги:

- `AGENTS.md`
- `.codex/README.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- optional related workflow docs if needed

Проверки:

- `make lint-codex-governance`

Артефакт результата:

- repo-local instruction surface aligned to live baseline.

## 4. Rollback point

После Slice 1: drift inventory already captured, but no instruction surface has
changed yet.

## 5. Final validation

- `make lint-codex-governance`
- `git diff --check`
