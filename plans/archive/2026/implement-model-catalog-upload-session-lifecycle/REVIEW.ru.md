# REVIEW

## Findings

Критичных блокеров по текущему slice пока не найдено.

Внешний reviewer pass дал одно medium-level замечание и оно закрыто в этой же
итерации:

- concrete helper command оставлен только для agreed namespaced `Model` UX;
- для `ClusterModel` `status.upload.command` сейчас намеренно не цементирует
  будущий helper CLI surface.

## Scope check

- Public API status для upload доведён до agreed shape через
  `status.upload.{expiresAt,repository,command}`.
- В controller module добавлен bounded `internal/uploadsession` без прыжка в
  live reconcile/runtime layer.
- Publisher upload grant остаётся отдельной веткой от consumer access planning:
  - `internal/accessplan` по-прежнему про published read access;
  - `internal/uploadsession` и `internal/payloadregistry` закрывают temporary
    staging grant planning.
- Slice не протёк в live Role/RoleBinding creation, artifact detection,
  publication worker или metadata inspection.

## Checks

Пройдены:

- `go generate ./...` в `api`
- `go test ./...` в `api`
- `bash scripts/verify-crdgen.sh` в `api`
- `go test ./...` в `images/controller/`
- `make fmt`
- `make test`
- `git diff --check`

## Residual risks

- `status.upload.command` сейчас intentionally остаётся helper-level подсказкой.
  Следующий runtime/helper slice должен сохранить этот уровень абстракции и не
  протащить внутрь public contract raw registry login plumbing.
- Для `ClusterModel` helper UX ещё не зафиксирован. Это сознательно оставлено на
  следующий runtime/helper slice, чтобы не freeze'ить CLI surface раньше
  design-решения.
- `CapabilityPush` в `payloadregistry` сейчас маппится только на verb `create`.
  Если выяснится, что реальный upload client требует дополнительных registry
  verbs, это нужно будет решить отдельным runtime slice, а не размывать текущую
  границу.
- Upload session planner пока только планирует phase/status/conditions и staging
  grant intent. Следующий шаг всё ещё остаётся за runtime slice:
  materialization, expiry rotation, observation uploaded artifact и promotion.
