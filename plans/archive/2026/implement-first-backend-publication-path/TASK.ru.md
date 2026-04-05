# TASK

## Контекст

После corrective rebasing controller phase-2 больше не закрепляет ложный путь
`payload-registry + mlflow publication`, но живого `source -> backend artifact`
пути пока нет.

У модуля уже есть:

- public API `Model` / `ClusterModel` со `spec.source={HuggingFace|HTTP|Upload}`;
- thin lifecycle controller `modelpublish`;
- durable execution boundary `publicationoperation`;
- module config для object storage (`ai-models.artifacts.*`);
- backend image с Python runtime, `huggingface_hub`, `transformers` и S3 helper
  code.

## Цель

Сделать первый живой phase-2 publication path для `spec.source.type=HuggingFace`
через module-owned backend artifact plane:

1. controller-owned operation создаёт backend publication Job;
2. Job скачивает модель из Hugging Face;
3. Job сохраняет published artifact в module-owned object storage prefix;
4. Job считает базовый technical profile и пишет durable result в operation ConfigMap;
5. lifecycle controller переводит `Model` / `ClusterModel` в `Ready`;
6. delete cleanup удаляет сохранённый artifact через backend cleanup entrypoint.

## Почему это следующий slice

- Это минимальный end-to-end path, который совпадает с ADR и текущими ожиданиями
  пользователя.
- Он не возвращает `mlflow` в phase-2 publication hot path.
- Он использует уже существующий module config и backend image вместо новой
  storage surface.

## Scope

В этом slice разрешено:

- довести `publicationoperation` до live Job-based execution для
  `HuggingFace` source;
- добавить backend worker entrypoint для object-storage publication;
- сделать live backend cleanup entrypoint по уже существующему cleanup handle;
- прокинуть controller/job env для object storage из module templates;
- вычистить текущие runtime/docs/test хвосты старой registry-specific модели.

В этом slice не делаем:

- `Upload` execution;
- `HTTP` execution;
- controller-owned source auth secret projection;
- runtime materializer / agent для PVC;
- payload backend implementation;
- planner-level rich hardware profiling сверх базового technical profile.

## Acceptance criteria

- `publicationoperation` больше не фейлит `HuggingFace` request сразу.
- Для `HuggingFace` request controller создаёт Job в operation namespace и ждёт
  durable worker result.
- Worker пишет artifact result с `kind=ObjectStorage`, `uri`, `digest`,
  `mediaType`, `sizeBytes`, source provenance и resolved profile.
- `modelpublish` проецирует этот result в `status.source`, `status.artifact`,
  `status.resolved`, `phase=Ready`, `conditions`.
- cleanup handle использует backend artifact contract и live cleanup command.
- delete path удаляет saved artifact через backend cleanup Job.
- `README`/docs текущего controller direction больше не ссылаются на удалённые
  `uploadsession`/`payload-registry` semantics как на active surface.

## Validation

Минимум:

- `go test ./...` in `images/controller`
- `python3 -m py_compile` для новых/изменённых backend scripts
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `git diff --check`

Желательно:

- `make verify`

## Rollback point

Если live Job-based publication окажется слишком нестабильным, безопасный
rollback point — оставить:

- neutral `artifactbackend` contract;
- cleanup/current-surface cleanup;
- honest failed operation for non-implemented source types.
