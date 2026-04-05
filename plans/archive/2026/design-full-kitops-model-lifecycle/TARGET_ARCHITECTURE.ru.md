# Target Architecture

## 1. Canonical lifecycle

```text
User / CI
  |
  | create Model / ClusterModel with spec.source
  v
Catalog controller
  |
  | source fetch or upload session
  v
Normalized model workspace
  |
  | ModelPack manifest generation + policy checks
  | current implementation adapter packs/pushes/inspects
  v
OCI artifact (ModelPack/ModelKit)
  |
  | status.artifact + status.resolved
  v
Runtime delivery adapter
  |
  | kit init -> pull/verify/unpack into PVC/shared volume
  v
Inference runtime
  |
  | local model path only
  v
Delete guards + cleanup
```

## 2. Public platform contract

### `spec`

Пользователь задаёт только:

- `spec.source`;
- `spec.package`;
- `spec.runtimeHints`;
- `spec.access`.

Пользователь не задаёт:

- final OCI ref;
- registry credentials;
- runtime materialization internals;
- cleanup semantics;
- raw implementation-specific entities.

### `status`

Платформа отдаёт только:

- `status.phase`;
- `status.source`;
- `status.upload` при `Upload`;
- `status.artifact` (`kind=OCI`, `uri`, `digest`, `mediaType`, `sizeBytes`);
- `status.resolved`;
- `conditions`.

## 3. Internal bounded components

### Publication controller

Отвечает за:

- acceptance `spec.source`;
- orchestration upload/source acquisition;
- normalization workspace;
- policy checks перед publication;
- `ModelPack` pack/push/inspect through a replaceable implementation adapter;
- projection public `status`;
- internal cleanup handle.

### Runtime delivery adapter

Отвечает за:

- взять уже опубликованный OCI artifact;
- проверить integrity/signature policy;
- unpack into shared volume / PVC;
- передать runtime только локальный путь.

Для v0 это upstream `kit init` container.

Для v1+ это module-owned materializer image/agent.

### Target implementation boundaries

Чтобы не повторить fat-controller drift, implementation должен резаться так:

- domain:
  - publication state;
  - runtime materialization state;
  - delete guards and leases;
- application/use cases:
  - `AcceptSource`;
  - `StartPublication`;
  - `ObservePublication`;
  - `ProjectReadyStatus`;
  - `StartMaterialization`;
  - `ObserveMaterialization`;
  - `CheckDeleteGuards`;
  - `CleanupArtifactAndMaterialization`;
- ports:
  - publication operation store;
  - worker runtime;
  - `ModelPack` publisher;
  - artifact verifier;
  - materializer runtime;
  - lease/reference store;
- adapters:
  - Kubernetes reconcilers;
  - ConfigMap/Pod/Secret/PVC materializers;
  - current init/materializer wrapper.

То есть concrete tools such as upstream `kitops-init`, `KitOps`, `Modctl` or a
future module-owned implementation не должны быть точкой, вокруг которой
строится public or domain contract. Это только adapters.

### Cleanup controller

Отвечает за:

- delete guards;
- cleanup published OCI artifact;
- cleanup local materialization state;
- finalizer release только после успешной очистки.

## 4. V0 runtime path

### Decision

В v0 используем текущий upstream init-container adapter как временную
implementation.

### Почему это приемлемо

- upstream already documents it exactly for Kubernetes init-container flow;
- он уже умеет pull + optional Cosign verification + unpack;
- нам не нужно сейчас писать свой unpack/runtime image раньше времени;
- это keeps scope bounded while we still build the right platform contract.

### Как используем его у нас

Runtime Pod / Deployment получает:

- shared PVC or shared volume;
- `initContainer` с pinned image digest текущего implementation adapter;
- `MODELKIT_REF=<immutable digest ref from status.artifact.uri>`;
- `UNPACK_PATH=<shared volume path>`;
- `UNPACK_FILTER=<platform-chosen filter>` with explicit default instead of
  upstream implicit `model`;
- optional `EXTRA_FLAGS`, preserving explicit controller-owned policy instead of
  relying on implicit defaults;
- optional Cosign verification envs:
  - `COSIGN_KEY`; or
  - `COSIGN_CERT_IDENTITY` + `COSIGN_CERT_OIDC_ISSUER`;
- registry auth mounted through standard OCI secret path.

Main runtime container получает только:

- volume mount;
- local model path;
- no direct registry credentials.

## 5. V1+ runtime path

После рабочего v0 путь развивается не через усложнение public API, а через
замену runtime adapter:

- module-owned init image instead of raw upstream image;
- distroless/hardened runtime;
- cache reuse across restarts;
- materialization leases;
- prewarming and GC;
- later, optional node/PVC cache plane or dedicated agent.

Public contract при этом не меняется.

## 6. Delete path

### Publication artifact cleanup

При удалении `Model` / `ClusterModel` controller:

1. проверяет, можно ли удалять published artifact;
2. удаляет OCI artifact by saved internal cleanup handle;
3. очищает local materialization state;
4. снимает finalizer.

### Guard model

Нужны два уровня guard:

- publication guard:
  - есть ли active runtime leases / bindings на модель;
- storage/materialization guard:
  - остались ли связанные PVC/cache objects.

Если runtime reference model ещё не оформлен как отдельный DKP object,
минимальный v0 guard может быть слабым. Но архитектурно delete path должен быть
готов к последующему введению explicit runtime references.
