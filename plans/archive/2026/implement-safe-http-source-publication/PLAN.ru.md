# PLAN

## Current phase

Этап 2. `Model` / `ClusterModel`, controller publication plane и platform UX.

## Orchestration

- mode: `full`
- read-only review before code changes:
  - backend archive safety audit;
  - controller/API gap audit for `HTTP` on current `pod/session` path.
- final substantial review:
  - `review-gate`
  - reviewer-style read-only pass

## Read-only audit conclusions

- минимальный production-worthy hardening: отказаться от `extractall`,
  запретить symlink/hardlink и специальные entry types, извлекать только
  regular files/directories в заранее проверенные пути;
- `HTTP` можно включить на текущем `sourcepublishpod` path без изменения public
  controller layering;
- narrow live scope на этот slice: `HTTP` без `authSecretRef`, но с optional
  inline `caBundle`;
- `HTTP` должен требовать `spec.runtimeHints.task` до запуска worker pod,
  потому что здесь нет `pipeline_tag`-style fallback;
- reuse archive unpack helper между `HTTP` и `Upload` нужно сохранить, чтобы не
  расколоть два почти одинаковых path.

## Slice 1. Harden Archive Extraction

Цель:

- убрать `extractall` из current backend worker;
- закрыть path/link escape для tar/zip archive extraction.

Файлы/каталоги:

- `images/backend/scripts/ai-models-backend-source-publish.py`
- backend helper tests for this script

Проверки:

- `python3 -m py_compile images/backend/scripts/ai-models-backend-source-publish.py`
- `python3 -m unittest discover -s images/backend/scripts -p 'test_*.py'`

Артефакт:

- safe archive extraction helper с unit tests на traversal/link cases.

## Slice 2. Re-enable HTTP On Pod-Based Publication Path

Цель:

- включить `HTTP` в `sourcepublishpod` и `publicationoperation`;
- передавать `http.url` и optional `http.caBundle`;
- `http.authSecretRef` оставить explicit controlled failure.

Файлы/каталоги:

- `images/controller/internal/sourcepublishpod/*`
- `images/controller/internal/publicationoperation/*`
- affected controller tests

Проверки:

- `go test ./internal/sourcepublishpod ./internal/publicationoperation` в `images/controller`

Артефакт:

- live `HTTP` path на current pod/session controller architecture;
- `http.authSecretRef` остаётся explicit controlled failure;
- `spec.runtimeHints.task` валидируется до запуска worker pod.

## Slice 3. Sync Docs And Validate End State

Цель:

- синхронизировать live scope в docs и bundle;
- прогнать repo-level validations.

Файлы/каталоги:

- `images/controller/README.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `plans/active/implement-safe-http-source-publication/*`

Проверки:

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

Артефакт:

- docs и code одинаково описывают `HTTP` как live path с narrow scope.

## Rollback point

После Slice 1 backend archive hardening уже полезен сам по себе и не ломает
public controller behavior, даже если live `HTTP` path ещё не включён.

## Execution status

- Slice 1 completed.
- Slice 2 completed for unauthenticated `HTTP` plus optional inline `caBundle`.
- Slice 3 completed.
- `http.authSecretRef` intentionally remains a controlled failure for a later
  credential-projection slice.
