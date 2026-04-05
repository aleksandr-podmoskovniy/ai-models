# Rebase Publication To Pod Session KitOps OCI

## Контекст

Текущий phase-2 publication path ушёл в сторону batch-style `Job` execution и
object-storage final artifact. Это расходится сразу с несколькими уже
зафиксированными ожиданиями:

- source-first UX должен оставаться по аналогии с virtualization;
- upload должен жить как controller-owned session, а не как batch job;
- worker execution boundary должен быть ближе к virtualization importer/uploader
  pod pattern, а не к разовым batch jobs;
- canonical published artifact должен идти в сторону `KitOps` / OCI, а не
  закрепляться на `ObjectStorage` как platform contract.

Дополнительно текущий `HTTP` live path уже показал security gap, так что
наращивать нынешнюю `Job + object storage` схему дальше нельзя.

## Постановка задачи

Начать corrective re-architecture phase-2 publication path:

1. остановить дальнейшее развитие `Job`-based publication execution;
2. перейти на controller-owned worker `Pod` / session direction по образцу
   virtualization;
3. подготовить controller/backend contracts к будущему `KitOps -> OCI`
   publication flow;
4. зафиксировать `Upload` как session-based direction, а не batch import.

## Scope

- Завести новый task bundle под corrective re-architecture.
- Выполнить первый bounded slice:
  - заменить live `source publish` execution boundary c `Job` на worker `Pod`;
  - rebased internal naming/contracts так, чтобы current execution уже не
    навязывал batch semantics;
  - временно выключить небезопасный live `HTTP` path вместо дальнейшего
    развития уязвимого flow;
  - сохранить working `HuggingFace` path на новой pod-based execution boundary;
  - подготовить bundle/docs/controller wording к следующему `Upload session` и
    `KitOps/OCI` slice.
- Зафиксировать в active bundle, что следующий publication target должен
  строиться вокруг `KitOps` packaging и OCI artifact refs.

## Non-goals

- Не реализовывать в этом slice весь `Upload` flow целиком.
- Не реализовывать в этом slice полноценный `KitOps` packaging/push.
- Не переделывать сейчас весь public API contract под final OCI-only shape.
- Не трогать phase-1 backend rollout, кроме тех частей, которые напрямую связаны
  с publication worker runtime.

## Затрагиваемые области

- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/app/*`
- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/modelpublish/*`
- `images/controller/internal/sourcepublish*`
- `images/backend/scripts/*source-publish*`
- `templates/controller/*`
- `images/controller/README.md`
- `docs/CONFIGURATION*`
- `plans/active/rebase-publication-to-pod-session-kitops-oci/*`

## Критерии приёмки

- В репозитории есть новый corrective bundle под `pod/session + KitOps/OCI`
  direction.
- Phase-2 publication execution больше не строится вокруг `batch Job` как
  основного primitive.
- Live `HuggingFace` path работает через controller-owned worker `Pod`.
- Небезопасный live `HTTP` path больше не остаётся включённым как working
  baseline.
- Controller/docs больше не описывают batch jobs как canonical direction для
  source publication.
- Узкие controller tests проходят.

## Риски

- Это corrective slice, поэтому часть уже написанного `Job`-based кода придётся
  выбросить или переложить.
- Если пытаться одновременно сделать и `Upload session`, и `KitOps/OCI`, и
  public API rebase, задача расползётся и зависнет без рабочего rollback point.
- После этого slice всё ещё останутся обязательные следующие шаги:
  `Upload session`, safe `HTTP`, `KitOps/OCI` publication, runtime materializer.
