# Structure Project Skills And Agent Memory

## 1. Контекст

В репозитории уже есть project-specific skills под `.agents/skills/`, но сейчас
они слишком общие и не удерживают достаточно жёстко именно ту дисциплину,
которая уже была выведена через corrective bundles:

- phase discipline;
- task bundle hygiene;
- archive hygiene для `plans/active`;
- hexagonal/controller boundaries;
- quality gates на complexity/LOC/thin reconcilers;
- запрет на наращивание feature work поверх fat controller.

Из-за этого repo memory слишком сильно живёт в:

- `AGENTS.md`;
- активных planning bundles;
- текущем контексте чата.

Нужно перенести эту память в сами project skills, чтобы дальнейшая работа
автоматически тянула правильные guardrails.

## 2. Постановка задачи

Структурировать project-specific skills и repo memory так, чтобы:

1. controller/API/runtime work автоматически тянул корректную архитектурную
   дисциплину;
2. task intake автоматически требовал plan hygiene и archive hygiene;
3. review/runtime work автоматически помнил про quality gates и thin
   reconciler policy;
4. не плодились лишние skills, а existing skill structure стала более
   намеренной и устойчивой.

## 3. Scope

- `.agents/skills/*`
- при необходимости `AGENTS.md` only if a gap cannot be expressed better in skills
- текущий bundle

## 4. Non-goals

- Не менять runtime/controller code.
- Не вводить сложную систему skill metadata beyond what repo already uses.
- Не дублировать целиком `AGENTS.md` внутрь skills.
- Не плодить множество мелких skills без реальной необходимости.

## 5. Затрагиваемые области

- `.agents/skills/task-intake-and-slicing/SKILL.md`
- `.agents/skills/controller-runtime-implementation/SKILL.md`
- `.agents/skills/model-catalog-api/SKILL.md`
- `.agents/skills/review-gate/SKILL.md`
- possible new repo-specific skill under `.agents/skills/*`
- `plans/active/structure-project-skills-and-agent-memory/*`

## 6. Критерии приёмки

- Есть отдельный bundle с rationale for skill restructuring.
- Existing skills tightened where they already own the right concern.
- Added at most one new repo-specific skill, only if it closes a real missing
  discipline boundary.
- Skills explicitly encode:
  - plan/archive hygiene
  - controller architecture discipline
  - quality-gate expectations
  - “no new feature work on fat controller” policy
- `git diff --check` passes.

## 7. Риски

- Самый опасный риск — продублировать `AGENTS.md` во все skills и получить
  шум вместо памяти.
- Второй риск — создать слишком много skills и сделать trigger surface хуже.
- Третий риск — зафиксировать guidance слишком абстрактно, без привязки к
  current corrective bundles и реальным guardrails.
