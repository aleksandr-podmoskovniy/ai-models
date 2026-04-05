# Review

## Scope check

Срез остался в пределах corrective re-architecture:

- live source publication больше не строится вокруг `batch Job` как основного
  primitive;
- рабочий path ограничен `HuggingFace -> worker Pod -> backend artifact`;
- `HTTP` оставлен в controlled failure до safe pod/session reimplementation;
- `Upload` не был насильно реализован через batch worker;
- controller, RBAC и docs синхронизированы с новым execution primitive.

Публичный API в этом slice не расширялся и не закреплял premature `OCI-only`
contract. Нейтральный `status.artifact` сохранён.

## What changed

- Старый `internal/sourcepublishjob` удалён.
- Новый execution boundary закреплён через
  `internal/sourcepublishpod/{pod.go,service.go}`.
- `publicationoperation` теперь:
  - materializes worker lifecycle через `sourcepublishpod.Service`;
  - ставит controller owner reference на operation `ConfigMap`;
  - watch'ит `Pod` owner events вместо опоры только на fixed timer requeue.
- `templates/controller/rbac.yaml` теперь даёт controller access к `pods` для
  publication path, не ломая cleanup `Job` path.
- `images/controller/README.md` и `docs/CONFIGURATION*` больше не описывают
  phase-2 publication как canonical `Job`/live `HTTP` flow.

## Validations

Выполнено:

- `python3 -m py_compile images/backend/scripts/ai-models-backend-source-publish.py`
- `go test ./internal/sourcepublishpod ./internal/publicationoperation ./internal/app` в `images/controller`
- `go test ./...` в `images/controller`
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

## Findings

Критичных блокеров после corrective slice не осталось.

## Residual risks

- `modelpublish` всё ещё requeue'ится по таймеру, потому что сам `Model` /
  `ClusterModel` пока не watch'ит operation `ConfigMap`. Для этого slice это
  допустимо: timer остаётся только на public lifecycle projection, а не на
  worker runtime orchestration.
- `HTTP` остаётся публично допустимым `spec.source.type`, но live path
  intentionally fails до safe reimplementation. Это честный bounded UX, но не
  финальное поведение phase-2.
- `Upload` по-прежнему требует отдельного session/supplements slice по образцу
  virtualization.
- Live publication всё ещё публикует в object-storage-backed backend artifact
  plane; следующий архитектурный шаг — `Upload session` и затем `KitOps` / OCI
  publication path без возврата к batch semantics.
