# Multi-model workload delivery

## Контекст

Текущий workload delivery contract умеет подключать только одну модель:

- workload задаёт `ai.deckhouse.io/model` или `ai.deckhouse.io/clustermodel`;
- controller резолвит один artifact;
- pod template получает один materializer / один CSI volume;
- runtime env содержит только `AI_MODELS_MODEL_PATH`,
  `AI_MODELS_MODEL_DIGEST`, `AI_MODELS_MODEL_FAMILY`.

Для inference workloads этого недостаточно: один pod может обслуживать базовую
модель, draft model, reranker, embedding model или LoRA/model companion. Такие
модели должны быть подключены одновременно, иметь стабильные readable paths и
не конфликтовать между собой в общем cache root.

## Постановка задачи

Добавить backward-compatible multi-model delivery contract для workload
annotations и runtime mutation.

Новый контракт должен:

- сохранять старые single-model annotations без изменения поведения;
- добавить multi-model annotation с alias-based references;
- резолвить несколько `Model` / `ClusterModel`;
- монтировать/материализовать каждую модель в отдельный стабильный путь;
- отдавать workload понятные env variables для primary и named models;
- не смешивать policy резолва, K8s mutation и node-cache layout.

## Scope

- `internal/controllers/workloaddelivery`: parsing, resolve, reconcile logging
  and delivery signal for multi-model references.
- `internal/adapters/k8s/modeldelivery`: render/apply нескольких bindings,
  init containers, CSI volumes, env contract and cleanup.
- `internal/nodecache` and `materialize-artifact`: stable alias link under
  `/data/modelcache/models/<alias>` for bridge paths.
- `templates/controller/webhook.yaml`: match condition for new annotation.
- Tests and docs for the new workload-facing contract.

## Non-goals

- Не добавлять новый CRD для workload binding в этом slice.
- Не менять `Model` / `ClusterModel` spec/status.
- Не делать vLLM-specific command generation: модуль даёт стабильные пути и
  env, а конкретный inference container сам выбирает аргументы.
- Не ломать старый `AI_MODELS_MODEL_*` contract для одного model reference.
- Не запускать live chaos/E2E в этом slice.

## Затрагиваемые области

- `images/controller/internal/controllers/workloaddelivery`
- `images/controller/internal/adapters/k8s/modeldelivery`
- `images/controller/internal/nodecache`
- `images/controller/cmd/ai-models-artifact-runtime`
- `templates/controller/webhook.yaml`
- `docs/CONFIGURATION*.md`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

## RBAC coverage

User-facing RBAC не расширяется:

- controller уже читает `Model` / `ClusterModel` для workload delivery;
- workload annotations не дают пользователю доступ к Secret, pod logs, exec,
  port-forward, status/finalizers или internal runtime resources;
- cluster-wide `ClusterModel` semantics остаются прежними: если persona не
  должна использовать `ClusterModel`, это проверяется существующей RBAC
  моделью доступа к объекту и admission/операторским policy, а не выдачей новых
  runtime прав workload namespace.

Проверки: targeted Go tests, helm template for webhook match condition.

## Критерии приёмки

- Старые annotations `ai.deckhouse.io/model` и
  `ai.deckhouse.io/clustermodel` продолжают работать как single primary model.
- Новая annotation `ai.deckhouse.io/model-refs` принимает список вида
  `main=Model/gemma,embed=ClusterModel/bge`.
- Alias является path/env-safe и становится директорией
  `/data/modelcache/models/<alias>`.
- Для multi-model workload выставляются:
  `AI_MODELS_MODELS_DIR`, `AI_MODELS_MODELS`,
  `AI_MODELS_MODEL_<ALIAS>_PATH`,
  `AI_MODELS_MODEL_<ALIAS>_DIGEST`,
  `AI_MODELS_MODEL_<ALIAS>_FAMILY`.
- Для первого/primary binding сохраняются `AI_MODELS_MODEL_PATH`,
  `AI_MODELS_MODEL_DIGEST`, `AI_MODELS_MODEL_FAMILY`.
- Bridge topology использует несколько init containers и shared-store layout,
  не перетирая `/data/modelcache/model`.
- SharedDirect topology использует отдельные inline CSI volumes на alias path.
- Cleanup удаляет все managed multi-model env, init containers, volumes and
  resolved annotations.
- Tests покрывают parser, render/apply, cleanup and reconcile apply path.

## Риски

- Annotation string может стать слишком длинной для десятков моделей; текущий
  slice целится в небольшой список прикладных моделей, не в сотни artifacts.
- Multi-model SharedDirect требует несколько CSI inline volumes; kubelet и CSI
  driver должны корректно обслуживать несколько NodePublishVolume для одного
  pod.
- Для vLLM всё равно нужен корректный container args/config; модуль не должен
  генерировать vLLM-specific CLI.

