# PLAN

## Current phase

Этап 2. Это bounded feature slice поверх уже идущего corrective refactor:
добавляем source auth projection, но не выходим за controller-owned publication
path и не перепрыгиваем в runtime materializer.

## Orchestration mode

`light`

Read-only subagent до кодовых изменений:
- auth/integration seam audit по minimal controller-owned projection contract

## Audit conclusions

- `application/publication/PlanSourceWorker` уже является correct seam для
  source-specific planning; здесь нужно разрешать `authSecretRef` и
  нормализовать namespace для namespaced `Model` vs `ClusterModel`, но не
  добавлять туда K8s client logic.
- Для namespaced `Model` explicit `authSecretRef.namespace` не должен указывать
  на чужой namespace: безопасное правило для этого slice — namespace либо пустой
  и тогда резолвится в owner namespace, либо совпадает с namespace объекта.
- `sourcepublishpod.Service` является correct adapter boundary для controller-
  owned projection Secret: именно он уже владеет worker Pod creation и может
  подготовить owned supplements до Pod materialization.
- Prompt cleanup projected auth Secret должен идти через `sourcepublishpod`
  delete path, а owner reference на publication operation остаётся safety net,
  а не единственным механизмом уборки.
- `sourcepublishpod.Build` должен получать только projected Secret name и
  собирать Pod contract:
  - `HuggingFace` -> `HF_TOKEN` env from projected Secret key `token`
  - `HTTP` -> `--http-auth-dir` плюс mounted projected Secret directory
- Worker-side HTTP auth contract уже имеет precedent в старом backend import
  path и должен оставаться минимальным:
  - `authorization`, или
  - `username` + `password`
- Source Secret нужно читать из source namespace и копировать только
  минимально нужные ключи в worker namespace (`d8-ai-models`), без прямого
  монтирования user Secret в worker Pod.

## Architecture acceptance criteria

- auth decision logic остаётся вне fat reconciler packages
- source auth projection реализуется как adapter/service concern around
  `sourcepublishpod`, а не как inline reconcile mutation
- controller копирует только минимально нужные ключи в worker namespace
- worker получает auth material через явный internal contract, без прямой
  зависимости на исходный user Secret
- новые non-test controller files остаются в текущих quality gates:
  `LOC <= 350`, cyclomatic complexity `<= 15`
- lifecycle/state evidence обновляется через targeted tests, а не только через
  happy-path integration test

## Slices

### Slice 1. Зафиксировать bounded task и auth contract

Цель:

- оформить отдельный bundle и зафиксировать минимальный auth contract для
  `HuggingFace` и `HTTP`

Файлы:

- `plans/active/implement-source-auth-secret-projection/TASK.ru.md`
- `plans/active/implement-source-auth-secret-projection/PLAN.ru.md`

Проверки:

- manual consistency check against `AGENTS.md`

Результат:

- есть явный scope, non-goals, acceptance criteria и rollback point

### Slice 2. Read-only audit auth seam

Цель:

- подтвердить minimal seam placement и exact secret-key contract до кода

Файлы:

- read-only inspection `images/controller/internal/sourcepublishpod/*`
- read-only inspection `images/backend/scripts/ai-models-backend-source-publish.py`
- выводы фиксируются в этом bundle

Проверки:

- subagent findings

Результат:

- понятен target shape controller-owned projected Secret и worker contract

### Slice 3. Реализовать auth projection для source worker Pods

Цель:

- добавить controller-owned secret projection для `HuggingFace` и `HTTP`

Файлы:

- `images/controller/internal/application/publication/*`
- `images/controller/internal/sourcepublishpod/*`
- `images/controller/internal/publicationoperation/*` только при минимальном
  runtime wiring
- `images/backend/scripts/ai-models-backend-source-publish.py`
- `templates/controller/rbac.yaml` только если нужен минимальный read access к
  source secrets

Проверки:

- `go test ./internal/application/publication ./internal/sourcepublishpod ./internal/publicationoperation`
- `python3 -m py_compile images/backend/scripts/ai-models-backend-source-publish.py`

Результат:

- live `HF/HTTP authSecretRef` path работает через controller-owned projection

### Slice 4. Закрыть bundle и синхронизировать docs

Цель:

- зафиксировать новый auth contract и убедиться, что repo-level shell не
  сломан

Файлы:

- `images/controller/README.md`
- `plans/active/implement-source-auth-secret-projection/PLAN.ru.md`
- `plans/active/implement-source-auth-secret-projection/REVIEW.ru.md`

Проверки:

- controller quality gates
- `go test ./...` в `images/controller`
- `make verify`
- `git diff --check`

Результат:

- slice закрыт, docs и review синхронизированы

## Rollback point

Если auth projection начинает требовать более широкого redesign, чем текущий
slice, безопасный rollback point: оставить только bundle и read-only выводы, не
менять worker/backend contract.

## Final validation

- `go test ./internal/application/publication ./internal/sourcepublishpod ./internal/publicationoperation`
- `python3 -m py_compile images/backend/scripts/ai-models-backend-source-publish.py`
- controller quality gates
- `go test ./...` в `images/controller`
- `make verify`
- `git diff --check`
