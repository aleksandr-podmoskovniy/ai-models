# План: documentation governance hardening

## Current phase

Governance slice. Не меняет runtime/API behavior, но меняет repo-local Codex
surface, поэтому выполняется отдельно от product/runtime задач.

## Orchestration

`solo`: это tightening existing governance surfaces без проектирования нового
runtime/API behavior. Новые skills/agents не создаются. Subagents не
используются, потому что задача governance/doc-only и цель — consistency review
существующих instruction layers.

## Active bundle disposition

- `capacity-cache-admission-hardening` — keep: отдельный storage/cache
  admission workstream с executable follow-up.
- `live-e2e-ha-validation` — keep: текущий e2e/HA runbook.
- `observability-signal-hardening` — keep: отдельный observability workstream.
- `pre-rollout-defect-closure` — keep: defect closure workstream, не смешивать
  с governance.
- `public-docs-virtualization-style` — keep: публичные docs; текущая задача
  правит governance, не содержимое docs.
- `ray-a30-ai-models-registry-cutover` — keep: отдельный workload cutover.
- `source-capability-taxonomy-ollama` — keep: taxonomy/Ollama source workstream.

## Slices

### 1. Зафиксировать docs source-of-truth в reusable workflow

- `.codex/README.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`

Проверка: ручная consistency review + `git diff --check`.

### 2. Ужесточить skills

- `.agents/skills/dkp-module-shell/SKILL.md`
- `.agents/skills/review-gate/SKILL.md`

Проверка: skill body остаётся concise и module-agnostic.

### 3. Ужесточить agent profiles

- `.codex/agents/module-implementer.toml`
- `.codex/agents/repo-architect.toml`
- `.codex/agents/reviewer.toml`
- `.codex/agents/task-framer.toml`

Проверка: agent profiles не копируют skill text целиком и остаются
role-specific.

### 4. Обновить machine-checkable inventory

- `.codex/governance-inventory.json`

Проверка:

```bash
make lint-codex-governance
make lint-docs
git diff --check
```

## Rollback point

Откатить изменения instruction surfaces и inventory одним diff. Product/runtime
код не меняется.

## Final validation

```bash
make lint-codex-governance
make lint-docs
git diff --check
```

## Evidence

- `python3 -m json.tool .codex/governance-inventory.json >/dev/null` passed.
- `make lint-codex-governance` passed.
- `make lint-docs` passed.
- `make render-docs` passed with current "no generated docs step is wired yet"
  message and marker validation.
- `make verify` passed.
- `git diff --check` passed.
