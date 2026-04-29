# Governance: документация, skills и agents

## Контекст

Публичные docs модуля были обновлены вручную, но `docs/CR*.md` не являются
местом для ручного описания CRD. В DKP module docs CR pages должны оставаться
render entrypoint с marker `<!-- SCHEMA -->`, а source of truth для схемы и
описаний должен жить в OpenAPI/API surfaces.

Это не разовая ошибка конкретного файла, а пробел в repo-local Codex
governance: skills и agent profiles недостаточно явно запрещают hand-written
generated docs и не заставляют проверять docs source-of-truth.

## Постановка задачи

Ужесточить существующие skills, agent profiles и workflow docs так, чтобы
следующие задачи по документации:

- сначала определяли source of truth;
- не писали generated CR docs вручную;
- обновляли OpenAPI/API/templates перед rendered docs;
- прогоняли `make lint-docs` / `make render-docs` / `make lint-codex-governance`
  там, где это уместно;
- фиксировали эти правила в reusable governance baseline без создания нового
  лишнего skill.

## Scope

- `.agents/skills/dkp-module-shell/SKILL.md`
- `.agents/skills/review-gate/SKILL.md`
- `.codex/agents/*.toml` для ролей, которые framing/implementation/review docs
  могут пропустить
- `.codex/README.md`
- `.codex/governance-inventory.json`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`
- текущий task bundle

## Non-goals

- Не создавать новый skill, если достаточно tightening существующих.
- Не менять public module docs content в этом slice.
- Не менять OpenAPI/API schemas.
- Не переносить правила конкретного `ai-models` product behavior в reusable
  core.

## Затрагиваемые instruction surfaces

- `.codex/README.md`
- `.agents/skills/dkp-module-shell/SKILL.md`
- `.agents/skills/review-gate/SKILL.md`
- `.codex/agents/module-implementer.toml`
- `.codex/agents/repo-architect.toml`
- `.codex/agents/reviewer.toml`
- `.codex/agents/task-framer.toml`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`
- `.codex/governance-inventory.json`

## Критерии приёмки

- Existing skills/agents explicitly require docs source-of-truth review.
- `docs/CR*.md` rule is durable: frontmatter + `<!-- SCHEMA -->`, no
  hand-written CRD field inventory.
- OpenAPI/API descriptions are documented as canonical source for generated
  schema docs.
- Review gate treats hand-written generated docs and stale rendered docs as
  findings.
- Governance inventory checks the new guardrails.
- `make lint-codex-governance`, `make lint-docs` and `git diff --check` pass.

## Риски

- Слишком project-specific wording может ухудшить переносимость baseline в
  соседний module repo. Поэтому правило формулируется как DKP docs
  source-of-truth discipline, а `ai-models` specifics остаются вне core.
