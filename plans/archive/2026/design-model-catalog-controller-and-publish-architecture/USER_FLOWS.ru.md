# USER FLOWS

## 1. HF import

### Пользовательский путь

1. Пользователь создаёт `Model`:

```yaml
apiVersion: ai-models.deckhouse.io/v1alpha1
kind: Model
metadata:
  name: deepseek-r1-0528-qwen3-8b
  namespace: team-a
spec:
  source:
    type: HuggingFace
    huggingFace:
      repoID: deepseek-ai/DeepSeek-R1-0528-Qwen3-8B
      revision: main
      authSecretRef:
        name: hf-token
  package:
    type: ModelPack
    layout: HuggingFaceCheckpoint
  runtimeHints:
    task: text-generation
    engines:
    - kserve
```

2. Controller валидирует spec.
3. Controller создаёт publish job.
4. Worker:
   - скачивает snapshot из HF;
   - пакует его в `ModelKit`;
   - пушит в staging repo;
   - промоутит в final repo;
   - возвращает metadata report.
5. Controller пишет final `status.artifact.*`, grants access и optional backend sync.
6. Объект становится `Ready`.

### Что видит пользователь

- `status.phase=Ready`
- `status.artifact.ociRef`
- `status.metadata.*`
- `conditions`

### Что не видит пользователь

- внутренние job names;
- raw HF cache directories;
- registry staging repo details после завершения.

## 2. Local upload с компьютера

## 2.1. Почему не browser upload

Большие model bytes нельзя тащить через browser/UI gateway модуля.

Правильный path:

- control plane через `Model`;
- data plane — direct registry push в staging path.

## 2.2. Flow

1. Пользователь создаёт `Model` с `source.upload`.
2. Controller:
   - переводит объект в `WaitForUpload`;
   - создаёт временный upload grant на staging repo;
   - пишет в `status.upload`:
     - final upload repo;
     - TTL;
     - готовую команду.
3. Пользователь на своей машине запускает helper:
   - локально пакует модель в `ModelKit`;
   - делает `kit push` в staging repo через Kubernetes token auth.
4. Controller замечает появление staging tag.
5. Controller/worker инспектирует artifact, промоутит в final repo и убирает
   upload grant.
6. Объект становится `Ready`.

## 2.3. Recommended UX

Нужен helper уровня:

```bash
d8 ai-models model upload team-a/deepseek-local --path ./model-dir
```

Под капотом helper:

1. создаёт или ждёт `Model`;
2. читает `status.upload`;
3. запускает `kit pack` / `kit push`.

То есть пользователь не должен вручную собирать registry URL и RBAC.

## 2.4. Аналогия с virtualization

Это тот же orchestration pattern, что и у virtualization:

- user-facing object переходит в `WaitForUserUpload`;
- controller выдаёт upload contract;
- controller затем принимает uploaded artifact и доводит объект до `Ready`.

Главное отличие:

- вместо docker auth secret copies на стороне DVCR здесь используется
  Kubernetes-token auth `payload-registry`.

## 3. External training producer

### Цель

Не заставлять `ai-models` владеть training orchestration.

### Flow

1. External pipeline сам собирает `ModelKit`.
2. Публикует его в allowed staging/external OCI ref.
3. Создаёт `Model` с `source.ociArtifact.ref`.
4. Controller:
   - проверяет artifact;
   - промоутит в final catalog repo;
   - рассчитывает status.

Это минимально достаточный путь для интеграции с JupyterHub/Airflow/training
pipelines.

## 4. KServe consumption

### Target path

`KServe` должен быть первым class-A consumer.

Пользователь или higher-level controller берёт из `status.artifact.ociRef` и
использует KitOps/KServe integration path.

Практический смысл:

- `ai-models` публикует artifact;
- inference plane не знает о MLflow;
- registry auth идёт через cluster-native credentials.

### Что нужно в status

- final immutable OCI ref;
- digest;
- optional runtime profile hint `kserve`.

## 5. KubeRay consumption

### Why special

У `KubeRay` нет такого же чистого first-class `KitOps` path, как у `KServe`.

### Target path

Для одной canonical published формы controller должен поддержать adapter story:

1. init-container тянет `ModelKit` из OCI registry;
2. unpack в shared volume;
3. Ray runtime читает local model path.

Это отдельный consumer adapter, а не отдельная canonical storage model.

### Временный shortcut

Если нужна operational простота до появления adapter path, можно держать direct
S3 path как temporary consumer implementation, но не как public storage contract.

## 6. Delete / cleanup

### Flow

1. Пользователь удаляет `Model` / `ClusterModel`.
2. Controller ставит `Deleting`.
3. Controller:
   - снимает read grants;
   - удаляет upload session/grants;
   - удаляет published tag/repository objects;
   - снимает backend mirror;
   - завершает finalizer.
4. Registry layers чистятся обычным GC `payload-registry`.

### Почему не hard-delete layers inline

Потому что registry GC уже штатно умеет убирать unreferenced layers, и это не
надо пытаться дублировать в reconcile loop.

## 7. User roles and scenarios

## 7.1. Namespace ML engineer

Нужно:

- импортировать модель из HF;
- получить готовый OCI ref;
- отдать его в inference/service team.

Использует:

- `Model`
- HF source
- optional local upload helper

## 7.2. Platform admin

Нужно:

- опубликовать curated shared model;
- выдать доступ нескольким namespace;
- держать cluster-wide catalog.

Использует:

- `ClusterModel`
- explicit `spec.access`
- platform-owned defaults

## 7.3. Inference operator

Нужно:

- взять published artifact и запустить serving.

Использует:

- `status.artifact.ociRef`
- consumer-specific integration (`KServe` first, `KubeRay` adapter later)

Не использует:

- backend UI;
- staging repos;
- source credentials.

## 8. Service decomposition

### Controller responsibilities

- API admission/reconcile
- upload session management
- access grants
- child jobs
- status calculation
- cleanup

### Worker responsibilities

- source fetch
- packaging
- registry push/promote
- artifact inspection
- structured result output

### Registry responsibilities

- OCI storage
- authn/authz
- tag/object listing
- layer GC

### Internal backend responsibilities

- provenance
- evaluation metadata
- secondary research/admin integration

## 9. What the first implementation should not do

- не делать browser upload;
- не делать arbitrary source matrix сразу;
- не обещать direct `KubeRay + KitOps` без adapter;
- не смешивать promotion/version-stream UX с первой publish implementation;
- не давать user push в final repo.
