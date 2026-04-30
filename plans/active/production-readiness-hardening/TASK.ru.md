# Production readiness hardening

## Контекст

Модуль готовится к продовому rollout. После нескольких крупных slices остались
активные изменения в runtime delivery, observability и chunked materialize.
Нужен повторный проход не по вкусовым рефакторам, а по доказуемым рискам:
security, Kubernetes API conventions, DKP module patterns, HA/replay,
observability noise and delivery correctness.

## Постановка задачи

Планомерно проверить текущий рабочий diff и критичные live paths, устранить
найденные дефекты, не добавляя новых public knobs и не размывая существующие
границы.

## Scope

- Current uncommitted diff around workload delivery, DMCR logging and chunked
  materialize.
- DKP templates and runtime pod/security context conventions.
- API/status/condition conventions for `Model` / `ClusterModel` where touched
  by current tails.
- RBAC/auth/secret exposure and runtime delivery auth boundaries.
- Observability signal/noise contract for controller/runtime/DMCR logs.
- Active plan hygiene: не держать завершённые хвосты как “активную работу”.

## Non-goals

- Не переписывать весь проект или переносить этапы 2/3 в текущий rollout.
- Не менять public spec shape без отдельного API bundle; status schema bounds
  are allowed here only when they match already-controller-owned projection
  limits and do not introduce user-written fields.
- Не добавлять fallback paths вместо целевых решений.
- Не запускать live e2e без отдельной команды пользователя и подтверждённого
  rollout.

## Затрагиваемые области

- `images/controller/**`
- `images/dmcr/**`
- `templates/**`
- `openapi/**` только если найден API/security defect
- `plans/active/**`
- `docs/**` только если меняется эксплуатационный или API contract

## Критерии приёмки

- Все изменения проходят repo gates, релевантные для затронутых областей.
- Нет новых файлов выше file-size gates и нет обхода complexity gates.
- Workload delivery не пишет секреты в пользовательские namespaces без явной
  необходимости и не шумит на стабильном blocked состоянии.
- DMCR cleanup/read/write logs различают ожидаемый miss и реальную ошибку.
- Chunked materialize остаётся resumable, bounded and immutable.
- Security context, service accounts, RBAC and token usage не ухудшаются.
- Status fields fed by external model metadata have explicit bounded schema
  and controller-side projection limits.
- Active plans отражают только executable next work.

## Риски

- Слишком широкий refactor может сломать уже проходящий verify.
- Security hardening может конфликтовать с текущим fallback path.
- API cleanup без отдельной миграции может создать несовместимость для live
  объектов.
