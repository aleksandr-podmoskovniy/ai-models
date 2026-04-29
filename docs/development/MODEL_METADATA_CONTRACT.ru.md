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

Важно: `task` и `supportedEndpointTypes` не являются двумя версиями одного
поля.

- `task` в текущем status - это source/provider task taxonomy: например
  Hugging Face `pipeline_tag` / `model-index.results.task.type` или будущая
  Ollama capability, нормализованная только по строке задачи.
- `supportedEndpointTypes` - это наша provider-neutral serving taxonomy:
  какой тип endpoint вообще имеет смысл предложить будущему `ai-inference`.

Поэтому пример `task: text-ranking` + `supportedEndpointTypes: [Rerank]`
корректен по смыслу, но плох по читаемости: без явного provenance слоя это
выглядит как дублирование. Целевой API slice должен либо переименовать/nest
`task` в source-facing capability, либо оставить его только как compatibility
projection до стабилизации CRD.

Текущая публичная проекция:

```yaml
status:
  resolved:
    format: Safetensors | GGUF | Diffusers
    architecture: LlamaForCausalLM
    family: llama
    task: text-generation
    parameterCount: 7000000000
    quantization: q4_k_m
    contextWindowTokens: 8192
    supportedEndpointTypes:
      - TextGeneration
      - Chat
    supportedFeatures:
      - VisionInput
      - ToolCalling
```

Смысл полей:

| Поле | Зачем нужно | Когда заполняется |
| --- | --- | --- |
| `format` | Выбор parser/runtime family. | Когда format известен из source selection или artifact inspection. |
| `architecture` | Семейство model class для runtime validation. | Только из config/metadata, не из имени файла. |
| `family` | UX/search и грубая группировка. | Только из надёжного config-derived сигнала. |
| `task` | Source/provider task provenance, например HF `pipeline_tag`. | Только из config/architecture mapping или другого надёжного source signal. Не является scheduler input. |
| `parameterCount` | Capacity hint для будущего расчёта. | Только exact/derived. Estimated bytes-based значение наружу не публикуется. |
| `quantization` | Runtime/parser hint. | Только из metadata/config. Filename-derived GGUF suffix наружу не публикуется как факт. |
| `contextWindowTokens` | Вход для KV-cache расчёта. | Только из config/tokenizer metadata. |
| `supportedEndpointTypes` | Предварительная endpoint capability: какой API-тип вообще имеет смысл поднимать. | Только из надёжного `task`. Не заменяет runtime validation. |
| `supportedFeatures` | Сквозные признаки модели, которые не являются отдельным endpoint: modality и tool calling. | Только из надёжного task/source-declared metadata или tokenizer/chat-template evidence. |

Целевой менее шумный shape для следующего CRD slice:

```yaml
status:
  resolved:
    format: GGUF
    architecture: qwen35moe
    family: qwen35moe
    parameterCount: 36000000000
    quantization: Q4_K_M
    contextWindowTokens: 131072
    supportedEndpointTypes:
      - Chat
      - TextGeneration
    supportedFeatures:
      - ToolCalling
    sourceCapabilities:
      provider: Ollama
      tasks:
        - completion
```

Так consumer видит один главный ответ для запуска (`supportedEndpointTypes`) и
отдельную provenance/evidence часть (`sourceCapabilities`), не путая их.

Публично не публикуются:

- `minimumLaunch`;
- `acceleratorCount`;
- `compatibleRuntimes`;
- `compatibleAcceleratorVendors`;
- `compatiblePrecisions`;
- `framework`;
- `footprint`;
- `evidence`;
- `mcpTools`;
- runtime-specific exceptions вроде `KubeRay + vLLM + MIG + MPS`.

Причина простая: эти поля легко принять за готовое scheduler-решение, хотя
`ai-models` не видит cluster inventory и не владеет runtime compatibility
matrix.

Минимальная публичная таксономия:

| Source task | `supportedEndpointTypes` | `supportedFeatures` |
| --- | --- | --- |
| `text-generation`, `text2text-generation`, `conversational` | `Chat`, `TextGeneration` | empty |
| `sentence-similarity`, `feature-extraction`, `embeddings` | `Embeddings` | empty |
| `text-ranking`, `rerank` | `Rerank` | empty |
| `automatic-speech-recognition` | `SpeechToText` | `AudioInput` |
| `text-to-speech` | `TextToSpeech` | `AudioOutput` |
| `text-to-audio`, `text-to-music` | `AudioGeneration` | `AudioOutput` |
| `image-classification` | `ImageClassification` | `VisionInput` |
| `object-detection` | `ObjectDetection` | `VisionInput` |
| `image-segmentation` | `ImageSegmentation` | `VisionInput` |
| `image-to-text`, `image-text-to-text` | `Chat`, `ImageToText` | `VisionInput`, `MultiModalInput` |
| `visual-question-answering` | `VisualQuestionAnswering` | `VisionInput`, `MultiModalInput` |
| `text-to-image` | `ImageGeneration` | `ImageOutput` |
| `image-to-image`, `inpainting` | `ImageGeneration` | `VisionInput`, `ImageOutput` |
| `text-to-video` | `VideoGeneration` | `VideoOutput` |
| `image-to-video` | `VideoGeneration` | `VisionInput`, `VideoOutput` |
| `video-to-video` | `VideoGeneration` | `VideoInput`, `VideoOutput` |

`ToolCalling` добавляется не из task name, а из chat-template evidence
(`tokenizer_config.chat_template` с явной веткой `tools` / `tool_call`) или
будущего source-declared факта такого же качества.

## Как читать внешние каталоги

Hugging Face UI показывает несколько независимых осей: `Tasks`, `Libraries`,
`Apps`, `Inference Providers`, размер параметров и прочие tags. Для
`ai-models` это не один плоский enum:

- `Tasks` (`pipeline_tag`, task tags, `model-index`) - source/provider task
  evidence;
- `Libraries` (`transformers`, `diffusers`, `gguf`, `sentence-transformers`,
  `onnx`) - format/framework evidence, но не serving guarantee;
- `Apps` (`vLLM`, `llama.cpp`, `Ollama`, `LM Studio`) - ecosystem/runtime hint,
  не public runtime compatibility;
- `Inference Providers` - внешний hosted-serving факт, не применимый напрямую
  к нашему кластеру;
- parameter filter - UX/search hint, который нельзя превращать в placement.

Ollama надо читать иначе: public HTML может быть удобен человеку, но
controller должен опираться на машинные контракты:

- registry manifest/layers для byte path и digest;
- config/details для `format`, `family`, `parameter_size`, `quantization`;
- `model_info` для `general.parameter_count`, architecture-specific context
  fields и tokenizer metadata;
- `capabilities` как provider capability evidence, а не как готовый
  `ai-inference` endpoint plan.

## Confidence model

Внутри controller/runtime каждое значение имеет confidence:

| Confidence | Значение |
| --- | --- |
| `Exact` | Прочитано из artifact/config/source metadata как явный факт. |
| `Declared` | Явно объявлено source provider'ом, например `pipeline_tag` в HuggingFace metadata. |
| `Derived` | Выведено из exact values по стабильному правилу. |
| `Estimated` | Оценено по bytes, dtype или коэффициентам. |
| `Hint` | Взято из слабого сигнала, например имени GGUF-файла. |

В public `status.resolved` попадают только `Exact`, `Declared` и `Derived`
значения. `Estimated` и `Hint` остаются внутри snapshot/profile и могут
использоваться для логов, diagnostics и будущего internal planning contract.

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
  SupportedFeatures
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
    diffusers/        # извлекает layout/pipeline facts из model_index.json
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
- `supportedFeatures` из task modality или tokenizer/chat-template evidence.

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

## Diffusers

`Diffusers` - это отдельный layout, а не просто набор `.safetensors` файлов.
Публичный `format=Diffusers` допустим только если artifact содержит:

- root `model_index.json`;
- хотя бы один `.safetensors` weight file;
- component configs/tokenizer/scheduler files, нужные для pipeline.

Что публикуется:

- `architecture` = `_class_name` из `model_index.json`, например
  `StableDiffusionPipeline` или `TextToVideoSDPipeline`;
- `family` только из стабильного pipeline-class mapping;
- `task` из explicit user/runtime option, Hugging Face `pipeline_tag` или
  надёжного pipeline-class mapping;
- endpoint/features только если task имеет `Exact`, `Declared` или `Derived`
  confidence.

Что намеренно не поддерживается в этом slice:

- `.bin` / PyTorch pickle weights;
- custom Python pipeline code;
- runtime-specific launch profiles;
- утверждение, что любой Diffusers artifact уже можно сервить любым выбранным
  runtime.

Поэтому `Diffusers + VideoGeneration` означает: artifact layout и metadata
достаточны для catalog publication и будущего `ai-inference` planner input. Это
не означает, что текущий module уже поднимает production video generation
endpoint.

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
- feature summary: vision/audio/image-output/video-output/multimodal/tool-calling;
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

Также нельзя писать "модель поддерживает MCP" как catalog fact. MCP - это
протокол host/runtime уровня. Каталог может показать только `ToolCalling`, если
есть tokenizer/chat-template evidence или другой надёжный source-declared
signal. Уже `ai-inference` решает, может ли выбранный runtime подать MCP tools
в эту модель.

## Что запрещено добавлять в catalog API

- `spec.task`, `spec.runtime`, `spec.endpointTypes` как пользовательские
  обещания совместимости.
- `status.resolved.launchProfiles` с GPU count, MIG profile или MPS share.
- Runtime-specific compatibility matrix.
- Public fields, которые можно заполнить только weak hint'ом.
- Public estimates, которые downstream легко примет за hard resource request.
- Endpoint values для unsupported artifact layouts. Diffusers-подобный
  layout допустим только как `format=Diffusers`, когда artifact реально
  включает `model_index.json` и `.safetensors` weights; иначе
  `text-to-image`/`text-to-video` не должны появляться только ради badge.

Любое расширение public metadata contract должно проходить отдельным API slice:

1. назвать конкретного consumer;
2. доказать, что поле нельзя оставить internal;
3. описать confidence/source semantics;
4. обновить RBAC exposure review;
5. покрыть status projection tests.
