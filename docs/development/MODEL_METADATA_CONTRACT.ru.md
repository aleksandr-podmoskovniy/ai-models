# Контракт metadata модели

`Model` / `ClusterModel` описывают опубликованный model artifact. Они не
являются планировщиком инференса и не должны обещать, сколько GPU, MIG-профилей
или MPS-долей нужно для запуска.

Главное правило: публичный `status.resolved` содержит только короткую сводку
проверяемых фактов модели. Всё, что является оценкой, слабой догадкой,
runtime-совместимостью или будущим scheduler input, остаётся внутренним
контрактом до появления реального consumer requirement.

## Почему так

Один и тот же artifact может запускаться разными способами:

- `1x80GiB`, `2x40GiB`, `4x24GiB` или с offload;
- plain `vLLM` или `vLLM` через KubeRay;
- whole GPU, MIG, MPS или без поддержки конкретной комбинации;
- одна модель в pod или несколько моделей вместе.

Эти решения зависят не только от модели. Нужны runtime mode, service class,
SLO, batching, concurrency, cluster inventory и матрица совместимости
runtime/topology. Поэтому `ai-models` считает metadata и resource factors, а
`ai-inference` позже строит настоящий launch plan.

## Публичный CRD contract

Текущая публичная проекция:

```yaml
status:
  resolved:
    format: Safetensors | GGUF
    architecture: LlamaForCausalLM
    family: llama
    task: text-generation
    parameterCount: 7000000000
    quantization: q4_k_m
    contextWindowTokens: 8192
    supportedEndpointTypes:
      - TextGeneration
      - Chat
```

Смысл полей:

| Поле | Зачем нужно | Когда заполняется |
| --- | --- | --- |
| `format` | Выбор parser/runtime family. | Когда format известен из source selection или artifact inspection. |
| `architecture` | Семейство model class для runtime validation. | Только из config/metadata, не из имени файла. |
| `family` | UX/search и грубая группировка. | Только из надёжного config-derived сигнала. |
| `task` | Базовый endpoint intent. | Только из config/architecture mapping или другого надёжного source signal. |
| `parameterCount` | Capacity hint для будущего расчёта. | Только exact/derived. Estimated bytes-based значение наружу не публикуется. |
| `quantization` | Runtime/parser hint. | Только из metadata/config. Filename-derived GGUF suffix наружу не публикуется как факт. |
| `contextWindowTokens` | Вход для KV-cache расчёта. | Только из config/tokenizer metadata. |
| `supportedEndpointTypes` | Предварительная endpoint capability. | Только из надёжного `task`. Не заменяет runtime validation. |

Публично не публикуются:

- `minimumLaunch`;
- `acceleratorCount`;
- `compatibleRuntimes`;
- `compatibleAcceleratorVendors`;
- `compatiblePrecisions`;
- `framework`;
- `footprint`;
- `evidence`;
- runtime-specific exceptions вроде `KubeRay + vLLM + MIG + MPS`.

Причина простая: эти поля легко принять за готовое scheduler-решение, хотя
`ai-models` не видит cluster inventory и не владеет runtime compatibility
matrix.

## Confidence model

Внутри controller/runtime каждое значение имеет confidence:

| Confidence | Значение |
| --- | --- |
| `Exact` | Прочитано из artifact/config/source metadata как явный факт. |
| `Derived` | Выведено из exact values по стабильному правилу. |
| `Estimated` | Оценено по bytes, dtype или коэффициентам. |
| `Hint` | Взято из слабого сигнала, например имени GGUF-файла. |

В public `status.resolved` попадают только `Exact` и `Derived` значения.
`Estimated` и `Hint` остаются внутри snapshot/profile и могут использоваться
для логов, diagnostics и будущего internal planning contract.

## Internal profile

Внутренний профиль хранится в `publishedsnapshot.ResolvedProfile`. Он нужен для
status projection сейчас и для будущего `ai-inference` handoff позже.

```text
ResolvedProfile
  Task + TaskConfidence
  Family + FamilyConfidence
  License
  Architecture + ArchitectureConfidence
  Format
  ParameterCount + ParameterCountConfidence
  Quantization + QuantizationConfidence
  ContextWindowTokens + ContextWindowTokensConfidence
  SourceRepoID
  SupportedEndpointTypes
  Footprint
    WeightsBytes
    LargestWeightFileBytes
    ShardCount
    EstimatedWorkingSetGiB
```

`Footprint` - это не request и не placement. Это измеренные или оценённые
факторы:

- `WeightsBytes` - сумма весовых файлов;
- `LargestWeightFileBytes` - самый крупный weight file, важен для streaming и
  shard/load strategy;
- `ShardCount` - количество weight shards;
- `EstimatedWorkingSetGiB` - грубая оценка памяти для operator UX и будущего
  planner input, но не guarantee.

## Где живёт расчёт

Граница должна оставаться гексагональной:

```text
images/controller/internal/
  adapters/modelprofile/
    safetensors/      # извлекает факты из config/tokenizer/weights metadata
    gguf/             # пока даёт только слабые filename/size hints
    common/           # format-neutral helpers and estimation primitives
  publishedsnapshot/  # immutable internal profile and confidence contract
  domain/publishstate/# public status projection and conditions
```

Правила:

- `adapters/modelprofile/*` извлекают факты, но не решают placement.
- `publishedsnapshot` хранит immutable result и confidence.
- `domain/publishstate` решает, какие факты безопасно показать наружу.
- `api/core/v1alpha1` остаётся схемой CRD, без provider parsing и hidden
  business logic.
- Kubernetes object shaping, Pods, CSI, node selectors и runtime topology не
  попадают в profile calculation.

## Safetensors

Надёжные public fields возможны, если доступны config/tokenizer metadata:

- `architecture` из `architectures`;
- `family` из `model_type`;
- `contextWindowTokens` из известных config/tokenizer keys;
- `quantization` из quantization config или dtype;
- `parameterCount` из явного metadata или стабильного derived rule;
- `supportedEndpointTypes` из надёжного task.

Если поле можно только оценить по bytes, оно остаётся internal.

## GGUF

Пока нет полноценного GGUF metadata parser, GGUF должен быть честно неполным:

- `format = GGUF` можно публиковать;
- family/quantization из filename остаются `Hint`;
- parameter count из размера файла остаётся `Estimated`;
- endpoint types не публикуются без надёжного task;
- architecture/context не заполняются догадками.

Такой объект может быть опубликован и быть `Ready`, но `MetadataResolved`
получает partial reason/message.

## Conditions

`MetadataResolved=True` означает: каталог смог построить поддержанный metadata
profile и безопасно спроецировать публичную summary.

Причины:

- `ModelMetadataCalculated` - public summary построена без low-confidence
  public gaps.
- `ModelMetadataPartial` - часть internal metadata была `Estimated` или `Hint`,
  поэтому controller не вынес её в public status.

Отдельный `Validated=True` не эмитится, пока нет настоящей policy validation.
Иначе condition создаёт ложное ощущение, что catalog уже проверил runtime и
launch policy.

## Граница с `ai-inference`

`ai-models` отдаёт:

- artifact digest/reference;
- format/layout facts;
- endpoint capability summary;
- model identity facts;
- internal footprint factors.

`ai-inference` считает:

- runtime engine and launcher;
- whole GPU / MIG / MPS / offload topology;
- количество устройств;
- pod resources;
- multi-model composition;
- fail-closed condition, если runtime/topology не подтверждены.

Формула ответственности:

```text
model metadata + artifact facts
+ service class and SLO
+ runtime compatibility matrix
+ cluster inventory
+ multi-model composition
= launch plan
```

Например, знание "`MPS` поверх `MIG` работает с plain `vLLM`, но не работает с
KubeRay+vLLM" не является свойством модели. Оно должно жить в
`ai-inference` compatibility registry, а не в `Model.status.resolved`.

## Что запрещено добавлять в catalog API

- `spec.task`, `spec.runtime`, `spec.endpointTypes` как пользовательские
  обещания совместимости.
- `status.resolved.launchProfiles` с GPU count, MIG profile или MPS share.
- Runtime-specific compatibility matrix.
- Public fields, которые можно заполнить только weak hint'ом.
- Public estimates, которые downstream легко примет за hard resource request.

Любое расширение public metadata contract должно проходить отдельным API slice:

1. назвать конкретного consumer;
2. доказать, что поле нельзя оставить internal;
3. описать confidence/source semantics;
4. обновить RBAC exposure review;
5. покрыть status projection tests.
