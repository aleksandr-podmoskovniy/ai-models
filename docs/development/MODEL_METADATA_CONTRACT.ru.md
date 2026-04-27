# Контракт metadata модели

## Зачем нужен отдельный контракт

`status.resolved` у `Model` / `ClusterModel` должен быть полезен будущему
`ai-inference`, но не должен притворяться, что все характеристики модели
известны одинаково точно.

Контроллер получает metadata из разных источников:

- фактическое содержимое модели и config files;
- metadata удалённого источника, например Hugging Face `pipeline_tag`;
- весовые файлы и их размер;
- имя файла, особенно для `GGUF`;
- платформенные правила про endpoint types, precision и minimum launch.

Эти сигналы не равнозначны. Значение, прочитанное из `config.json`, можно
использовать как факт. Значение, угаданное по имени файла, можно использовать
только как подсказку. Отсутствующее значение лучше оставить пустым, чем
заполнить догадкой и сломать downstream planner.

## Базовое правило

`status.resolved` — это краткая consumer summary. Она отвечает на вопрос
"что каталог знает о модели сейчас".

Metadata evidence/provenance — это доказательная модель. Она отвечает на вопрос
"почему каталог считает это значение достаточно надёжным".

До появления публичного consumer requirement evidence должна жить во внутреннем
runtime contract: `publishedsnapshot`, profilers и status projection. Публичную
schema надо расширять только отдельным API slice, когда станет ясно, какие
доказательства реально нужны `ai-inference`.

## Уровни честности

### Extracted

Значение прочитано из фактического model artifact или его config metadata.

Примеры:

- `architecture` из `config.json.architectures[0]`;
- `contextWindowTokens` из известного config key;
- quantization из явного quantization config.

Такое значение можно использовать как hard input, если format/parser
поддержаны.

### Derived

Значение выведено из extracted значений по стабильному правилу.

Примеры:

- `family` из `model_type`;
- endpoint types из надёжного `task`;
- precision из явной quantization.

Такое значение можно использовать как hard input только вместе с исходным
evidence.

### Estimated

Значение посчитано приблизительно по размеру, precision или quantization.

Примеры:

- `parameterCount` из bytes / bytes-per-parameter;
- `minimumLaunch` из model bytes с overhead factor.

Такое значение является capacity/UX hint. Оно не является scheduler guarantee и
не должно быть единственным основанием для отказа в запуске.

### Projected

Значение является платформенной проекцией из уже известных характеристик.

Примеры:

- `supportedEndpointTypes` из `task`;
- `compatibleAcceleratorVendors` из наличия GPU minimum launch.

Такое значение полезно для admission и UX, но downstream controller обязан
понимать исходное допущение.

### Hint

Значение взято из слабого сигнала.

Примеры:

- `family` из stem имени GGUF-файла;
- `quantization` из suffix имени GGUF-файла до появления настоящего GGUF
  metadata parser;
- `task` из source metadata без подтверждения содержимым.

Hint нельзя использовать как hard deny/allow decision без дополнительной
проверки runtime adapter'ом.

### Unknown

Надёжного сигнала нет. Поле остаётся пустым.

Unknown не является ошибкой публикации, если модель опубликована и format
поддержан. Ошибкой является только ситуация, когда metadata нужна для
публикации или safety decision и её нельзя получить.

## Поля `status.resolved`

| Поле | Целевой источник | Уровень | Consumer semantics |
| --- | --- | --- | --- |
| `format` | format detector / input selection | Extracted | Hard field. `ai-inference` может выбирать parser/runtime family. |
| `framework` | profiler path by format | Derived | Label пути профилирования, не engine compatibility. |
| `architecture` | model config | Extracted | Hard field, если прочитано из artifact metadata. Для GGUF пусто до parser support. |
| `family` | `model_type`, architecture normalization, GGUF filename fallback | Derived или Hint | Hard только при config-derived source. Filename-derived value только для UX/search. |
| `task` | source metadata, model config, architecture mapping | Extracted/Derived или Hint | Используется для endpoint projection только при достаточном confidence. |
| `parameterCount` | explicit metadata or bytes estimate | Extracted или Estimated | Exact можно использовать для planning. Estimate только capacity hint. |
| `quantization` | quantization config, dtype, GGUF metadata, filename fallback | Extracted/Derived или Hint | Hard только при metadata/config source. Filename fallback не должен быть hard input. |
| `contextWindowTokens` | model config / tokenizer config | Extracted | Hard field, если прочитано. Иначе пусто. |
| `supportedEndpointTypes` | platform mapping from reliable task | Projected | Admission hint. Chat/TextGeneration требуют runtime-specific validation. |
| `compatibleRuntimes` | future engine compatibility registry | Projected | Пока не заполнять. Empty не значит deny и не значит allow. |
| `compatibleAcceleratorVendors` | future hardware/runtime compatibility proof | Projected | Текущий GPU vendor list не должен восприниматься как точный hardware proof. |
| `compatiblePrecisions` | quantization/dtype interpretation | Derived или Hint | Hard только при надёжной quantization/dtype source. |
| `minimumLaunch` | model bytes / parameter count / overhead rule | Estimated | Capacity hint. Не scheduler request, не SLA и не proof для MIG/MPS. |

## Что модель должна говорить о запуске

Правильная граница звучит так: модель говорит не "запусти меня через
KubeRay+vLLM на `MIG`+`MPS`", а "я могу обслуживаться такими launch profiles,
если runtime умеет нужные features и кластер даёт подходящую ёмкость".

`Model` / `ClusterModel` должен давать будущему `ai-inference` не готовый
план запуска, а набор model-derived launch profiles:

- какие endpoint types допустимы для этого artifact'а;
- какие runtime features обязательны для запуска;
- какой artifact format и layout надо уметь читать;
- какие baseline memory coefficients известны или оценены;
- какие accelerator requirements следуют из размера/формата модели;
- какие значения являются фактами, оценками или hints.

Это промежуточный контракт между "сырая metadata модели" и "готовый workload".
Он нужен, чтобы планировщик принимал решение по явному пересечению трёх вещей:

- что умеет и требует модель;
- что разрешает класс сервиса и runtime policy;
- что реально доступно в кластере.

Минимальная внутренняя форма может выглядеть так:

```text
ResolvedPlanningProfile
  Artifact
    Digest
    Format
    Layout
  Capabilities[]
    EndpointType
    RequiredModelFeatures[]
    Evidence
  Footprint
    WeightsBytes
    ParameterCount
    Precision
    Quantization
    ContextWindowTokens
    KVCacheBytesPerTokenPerSequence
    Evidence
  LaunchProfiles[]
    Name
    EndpointTypes[]
    AcceleratorClass: CPU | GPU | Unknown
    MinAcceleratorMemoryBytes
    MinAcceleratorCount
    RequiredRuntimeFeatures[]
    Evidence
```

Для LLM это может означать:

- `Chat` допустим только если есть надёжный task/profile и runtime потом
  подтвердит chat template;
- `TextGeneration` допустим для decoder-only causal LM profile;
- `GPU` означает accelerator-backed path, если весовые файлы и baseline
  memory envelope явно не укладываются в CPU policy;
- `KVCacheBytesPerTokenPerSequence` является coefficient для planner'а, а не
  готовым request'ом;
- tensor parallel, distributed launch, `MIG`, `MPS` и KubeRay остаются
  runtime-specific planning decisions.

Такой profile не обязан сразу становиться публичным CRD-полем. Сначала он
должен жить как internal contract рядом с `publishedsnapshot.ResolvedProfile`
и evidence model. Публичную проекцию `status.resolved.launchProfiles` можно
добавлять только отдельным API slice, когда `ai-inference` докажет, что ему
нужен именно такой Kubernetes-level consumer contract.

## Граница `minimumLaunch` и планировщика инференса

`status.resolved.minimumLaunch.placementType=GPU` в `Model` / `ClusterModel`
не совпадает по смыслу с `InferenceService.status.resolved.placementType`.

В каталоге моделей `GPU` означает только одно: модель, по грубой оценке,
требует accelerator-backed launch path. Это не выбор между:

- целым GPU;
- `MIG`-партицией;
- `MPS`-долей;
- `MPS` поверх `MIG`;
- несколькими GPU на одной ноде;
- распределённым запуском на нескольких нодах.

Каталог не может честно выбрать эти варианты по одному artifact profile. Для
этого нужны данные, которых в `ModelPack` нет:

- какой runtime выбран: plain `vLLM`, `vLLM` через KubeRay, `Ollama`, `TGI` или
  другой engine;
- как runtime работает с выбранным способом изоляции GPU;
- есть ли у конкретного runtime mode подтверждённая совместимость с `MIG`,
  `MPS` и их комбинациями;
- какой endpoint type, batching, concurrency и latency/throughput SLO нужны;
- какие GPU, `MIG` profiles, device-plugin resources и sharing mechanisms
  реально доступны в кластере;
- сколько моделей нужно запустить в одном workload и как суммируется их memory
  envelope.

Например, наблюдение вида "`MPS` поверх `MIG` не работает через KubeRay+vLLM,
но работает для plain vLLM" не должно попадать в `Model.status.resolved`.
Это не свойство модели. Это свойство пары runtime-mode и accelerator
topology. Такое знание должно жить в `ai-inference` compatibility/planning
layer.

Следствие для будущего `ai-inference`:

- `Model.status.resolved.minimumLaunch` даёт нижнюю оценку памяти и количества
  условных accelerator units;
- `InferenceServiceClass` задаёт допустимые политики: dedicated/shared,
  allowed placement types, endpoint type и эксплуатационные ограничения;
- inventory слой даёт фактические GPU, `MIG` profiles, allocatable resources и
  sharing capabilities;
- runtime compatibility registry говорит, какие комбинации runtime mode,
  engine, accelerator vendor, partitioning и sharing реально поддержаны;
- planner выбирает итоговый `InferenceService.status.resolved.placementType`,
  `sharingMode`, vendor и count как пересечение этих четырёх источников.

Если хотя бы один источник не подтверждает комбинацию, planner должен
fail-closed с объяснимым condition, а не пытаться вывести совместимость из
`minimumLaunch`.

Минимальный compatibility registry для `ai-inference` должен описывать не
модели, а runtime modes:

```text
engine: vLLM
launcher: Plain | KubeRay
acceleratorVendor: NVIDIA
placementType: WholeDevice | Partition
sharingMode: Dedicated | Shared
partitioning: None | MIG
sharingMechanism: None | MPS
supported: true | false
reason: short stable reason
```

На таком уровне можно честно записать правило:

- `vLLM + Plain + MIG + MPS` supported, если это подтверждено тестами;
- `vLLM + KubeRay + MIG + MPS` unsupported, если это подтверждённый дефект или
  ограничение текущей интеграции.

Каталог моделей при этом остаётся стабильным и не начинает кодировать
runtime-specific исключения.

## Правила для `ai-inference`

`ai-inference` может использовать `status.resolved` и будущие launch profiles
как вход для выбора кандидатного runtime path, но обязан различать hard facts и
hints.

Жёсткие правила:

- `status.artifact.digest` и OCI reference являются source of truth для
  запуска, а не upstream URL или repo id.
- Empty `compatibleRuntimes` не запрещает запуск и не подтверждает
  совместимость.
- `minimumLaunch` задаёт стартовую оценку ёмкости, но runtime planner должен
  сам учитывать batching, concurrency, MIG/MPS, vendor-specific partitioning и
  SLO.
- `minimumLaunch.placementType=GPU` не должен напрямую маппиться в
  `WholeDevice` или `Partition`. Это только требование accelerator-backed path.
- `supportedEndpointTypes` не заменяет runtime validation. Например, наличие
  `Chat` не доказывает, что найден корректный chat template для конкретного
  engine.
- Multi-model workload должен читать независимый resolved profile для каждой
  модели. Общий pod-level план строится как композиция нескольких profiles, а
  не как новая aggregate metadata в `Model`.

Алгоритм планирования должен быть явным:

1. Получить resolved/planning profile для каждой модели.
2. Отфильтровать launch profiles по endpoint type из `InferenceServiceClass`.
3. Развернуть кандидатов runtime matrix: engine, launcher, delivery mode,
   accelerator vendor, partitioning и sharing mechanism.
4. Отбросить runtime candidates, которые не умеют обязательные model features.
5. Посчитать resource envelope по runtime-specific коэффициентам, SLO,
   concurrency, batching и количеству моделей.
6. Пересечь результат с cluster inventory и class policy.
7. Если остался один или несколько вариантов, выбрать deterministic launch
   plan. Если не осталось ни одного, выставить condition с причиной отказа.

В итоге модель говорит: "меня можно обслуживать как `Chat` / `TextGeneration`
при наличии runtime с такими features и такой нижней оценкой памяти". Кластер
говорит: "вот доступная ёмкость и topology". Compatibility matrix говорит:
"такая связка engine, launcher и GPU topology поддержана или запрещена".
Только после этого planner выбирает конкретный workload, параметры runtime и
resources.

## Форматные правила

### Safetensors

Safetensors path может давать надёжные поля, если доступны config files:

- `architecture` из `architectures`;
- `family` из `model_type`;
- `contextWindowTokens` из config/tokenizer metadata;
- quantization/precision из quantization config или dtype;
- task из source metadata или architecture mapping.

Если поле не найдено в config, его не надо заполнять слабой догадкой только
ради красивого status.

### GGUF

До появления настоящего GGUF metadata parser текущий GGUF profile не должен
выглядеть как точный.

Допустимо:

- определить `format = GGUF`;
- использовать filename-derived `family` и `quantization` как Hint;
- оценить `parameterCount` и `minimumLaunch` как Estimated, если есть размер.

Недопустимо:

- считать filename-derived family/quantization hard input;
- публиковать architecture/context как будто они прочитаны из модели;
- делать hard endpoint/runtimes decision только по имени файла.

## Internal evidence model

Следующий code slice должен добавить внутреннюю модель evidence без публичной
schema migration.

Минимальная форма:

```text
ResolvedProfileEvidence
  Field
  Value
  Source
  Confidence
  Message
```

Где:

- `Field` — имя поля summary;
- `Source` — `model-config`, `source-metadata`, `artifact-bytes`,
  `filename`, `platform-policy`, `runtime-estimate`;
- `Confidence` — `Extracted`, `Derived`, `Estimated`, `Projected`, `Hint`;
- `Message` — короткое объяснение для logs/tests/status message.

Эта модель должна жить рядом с `publishedsnapshot.ResolvedProfile` и
заполняться профилировщиками. `domain/publishstate` должен использовать её при
projection в public status и conditions.

## Conditions

`MetadataResolved=True` не должен означать "все поля известны точно".

Целевая семантика:

- True: каталог получил поддержанный profile и может опубликовать summary;
- message перечисляет important unknown/estimated fields;
- reason может отличать full exact profile от partial/estimated profile, если
  это нужно для operator UX;
- unsupported/malformed metadata остаётся failure только если модель нельзя
  безопасно опубликовать или запустить через поддержанный baseline.

## Порядок реализации

1. Добавить internal evidence model и tests вокруг Safetensors/GGUF profilers.
2. Изменить status projection так, чтобы low-confidence значения не выглядели
   как факты.
3. Уточнить condition messages и logs publication runtime.
4. После первого consumer в `ai-inference` принять отдельное API-решение:
   нужна ли публичная `status.resolved.evidence` или достаточно summary +
   condition/log evidence.

## Что нельзя делать

- Нельзя добавлять `spec.task`, `spec.runtime`, `spec.endpointTypes` как
  пользовательские обещания совместимости в текущем catalog API.
- Нельзя заполнять `compatibleRuntimes` догадками.
- Нельзя использовать source repo id как `family`.
- Нельзя считать `minimumLaunch` hard scheduler contract.
- Нельзя прятать provenance только в logs: metadata decisions должны быть
  проверяемы тестами.
