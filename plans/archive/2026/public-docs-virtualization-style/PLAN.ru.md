# План

## Current phase

Этап 2: Distribution and runtime topology. Задача документирует текущий
runtime/config/API baseline без изменения поведения.

## Orchestration

`solo`.

Сабагенты не используются: задача docs-only, а текущий turn не требует
spawn. Skills/agents не меняются, потому что для документационной проблемы
достаточно public docs; изменение governance surface потребовало бы отдельный
governance bundle.

## Reference patterns

- `virtualization/docs/README*.md`, `ADMIN_GUIDE*.md`, `USER_GUIDE*.md`,
  `CONFIGURATION*.md`, `CR*.md`, `FAQ*.md`.
- `gpu-control-plane/docs/README*.md`, `ADMIN_GUIDE*.md`,
  `CONFIGURATION*.md`, `EXAMPLES*.md`, `FAQ*.md`.

## Slices

### 1. Public docs structure

- Добавить `ADMIN_GUIDE`, `USER_GUIDE`, `CR`, `EXAMPLES`, `FAQ`.
- Обновить `README` как входную страницу и навигацию.
- Статус: выполнено.

### 2. Contract accuracy

- Зафиксировать, что public knobs минимальны.
- Отдельно отметить текущие ограничения: Ollama loader fail-closed,
  `Diffusers` metadata/layout есть, serving/runtime support зависит от
  будущего ai-inference.
- Исправить `CR*.md`: не дублировать CRD schema вручную, оставить generated
  docs entrypoint как в DKP modules (`<!-- SCHEMA -->`).
- Статус: выполнено.

### 3. Validation

- `git diff --check`;
- docs marker check через `make verify` или repo-level `make verify`, если
  время позволяет.
- Статус: выполнено. `git diff --check -- docs
  plans/active/public-docs-virtualization-style` и `make verify` прошли.

## Rollback point

Удалить новые docs pages и вернуть `README` / `CONFIGURATION` к предыдущему
состоянию. Runtime/API/templates не меняются.

## Final validation

- `git diff --check`;
- `make verify`;
- `review-gate`.

## Review gate

- Статус: выполнено.
- Findings: критичных замечаний нет.
- Residual risk: docs описывают upload API на уровне контракта, но не заменяют
  отдельный CLI/SDK для upload-клиента; это осознанно не входит в текущий
  docs slice.
