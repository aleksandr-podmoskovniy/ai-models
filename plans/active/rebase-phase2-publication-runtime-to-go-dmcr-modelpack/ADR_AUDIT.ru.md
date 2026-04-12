# ADR Audit

## Что сравнивалось

- текущий ADR:
  - [/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md)
- текущий public contract:
  - [types.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/api/core/v1alpha1/types.go)
  - [model.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/api/core/v1alpha1/model.go)
  - [clustermodel.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/api/core/v1alpha1/clustermodel.go)
- текущее фактическое заполнение статуса:
  - [conditions.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/domain/publishstate/conditions.go)
  - [support.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/dataplane/publishworker/support.go)
  - [snapshot.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/publishedsnapshot/snapshot.go)

## Совпадения

- Публичные объекты по-прежнему `Model` и `ClusterModel`.
- `spec` хранит желаемое состояние, `status` хранит вычисленное состояние.
- Артефакт модели хранится вне etcd.
- Контроллер пишет короткий `phase` и `conditions`.
- В public status нет внутренних сущностей backend-а.
- В `status` есть separate published artifact и separate calculated technical profile.

## Что в CRD реально живое сейчас

### spec.source

Текущий public contract уже не ADR-старый. Он такой:

- `spec.source.url`
- `spec.source.upload`

Тип источника пользователь больше не задаёт явно. Он определяется внутри:

- `huggingface.co` и `hf.co` -> `HuggingFace`
- `upload` -> `Upload`

Это делается в [source.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/api/core/v1alpha1/source.go).

### spec.inputFormat

Живое поле. Сейчас поддерживаются:

- `Safetensors`
- `GGUF`

Если поле пустое, controller пытается определить формат сам.

### spec.runtimeHints

Живое поле, но узкое:

- `task` реально участвует в publication path;
- `engines` реально участвуют в расчёте `status.resolved.compatibleRuntimes`.

### status.source

Живое поле.

Заполняется как:

- `resolvedType` -> из фактически выбранного source type;
- `resolvedRevision` -> если источник умеет её дать.

### status.upload

Живое поле только для upload path.

Заполняется controller-owned upload session:

- `expiresAt`
- `repository`
- `inClusterURL`
- `externalURL`

### status.artifact

Живое поле.

Заполняется из результата публикации:

- `kind`
- `uri`
- `digest`
- `mediaType`
- `sizeBytes`

### status.resolved

Живое поле.

Сейчас реально заполняются:

- `task`
- `framework`
- `family`
- `architecture`
- `format`
- `parameterCount`
- `quantization`
- `contextWindowTokens`
- `supportedEndpointTypes`
- `compatibleRuntimes`
- `compatibleAcceleratorVendors`
- `compatiblePrecisions`
- `minimumLaunch`

### status.conditions

Живые типы условий сейчас такие:

- `Accepted`
- `UploadReady`
- `ArtifactPublished`
- `MetadataReady`
- `Validated`
- `CleanupCompleted`
- `Ready`

## Главный drift относительно ADR

### 1. ADR-спека уже не совпадает с public API

ADR ожидает такие пользовательские поля:

- `spec.modelType`
- `spec.source.type`
- `spec.source.http.*`
- `spec.source.huggingFace.*`
- `spec.usagePolicy.*`
- `spec.launchPolicy.*`
- `spec.optimization.*`

В текущем CRD этого уже нет. Вместо этого сейчас:

- `spec.source.url | spec.source.upload`
- `spec.inputFormat`
- `spec.runtimeHints`

То есть ADR описывает более старую и более inference-centric форму API.

### 2. ADR описывает source contract иначе

В ADR пользователь сам задаёт тип source.
В текущем CRD пользователь задаёт только:

- ссылку
- либо upload

А `HuggingFace` и `Upload` различаются уже внутри controller по выбранной
ветке `source`.

### 3. В ADR есть inference policy в spec, а в коде её нет

В ADR есть большие пользовательские блоки:

- `usagePolicy`
- `launchPolicy`
- `optimization.speculativeDecoding`

В текущем коде их вообще нет как live контракта. Сейчас controller публикует
фактический технический профиль, но не принимает такой же богатый policy-блок
обратно в `spec`.

### 4. Формы условий не совпадают

ADR приводит:

- `ArtifactResolved`
- `MetadataResolved`
- `Validated`
- `Ready`

Текущий код живёт на:

- `Accepted`
- `UploadReady`
- `ArtifactPublished`
- `MetadataReady`
- `Validated`
- `CleanupCompleted`
- `Ready`

### 5. В ADR часть полей находится не там, где сейчас

- ADR кладёт `artifactDigest` в `status.resolved`
- текущий код хранит digest в `status.artifact.digest`

Текущая раскладка выглядит чище: published artifact и computed profile
разделены.

## Вывод

- По `status` текущий код уже ближе к практической рабочей модели, чем ADR.
- По `spec` текущий CRD заметно ушёл от ADR.
- Самый честный источник правды для текущего состояния сейчас:
  - код
  - CRD schema
  - текущий active bundle
- Сам ADR сейчас устарел и уже не может считаться точным контрактом без
  обновления.
