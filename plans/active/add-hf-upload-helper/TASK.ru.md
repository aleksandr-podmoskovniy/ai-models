# Добавить готовый helper для загрузки Hugging Face модели в ai-models

## Контекст

Phase-1 `ai-models` уже поднимает рабочий backend на базе upstream `MLflow`, но
штатного no-code UI workflow для загрузки Hugging Face model weights upstream не
даёт. Пользователю нужен максимально простой путь без ручного написания
собственного кода каждый раз.

Параллельно нужно коротко зафиксировать, какой UX планируется дальше на уровне
наших DKP CRD, чтобы не путать phase-1 helper script и будущий platform API.

## Постановка задачи

Нужно добавить repo-local helper script, который:
- принимает минимальный набор аргументов для Hugging Face модели;
- логирует модель в upstream `MLflow`;
- при необходимости сохраняет pretrained weights в artifact storage;
- регистрирует модель в model registry.

## Scope

- новый helper script под `tools/`;
- короткая self-documented usage header внутри script;
- узкая проверка script и repo-level verify;
- финальное объяснение пользователю по planned CRD UX из phase docs.

## Non-goals

- не проектировать phase-2 CRD schema в этом slice;
- не делать полноценный UI upload wizard;
- не добавлять user-facing module config под этот helper;
- не platformize'ить Hugging Face upload как встроенную runtime capability модуля.

## Затрагиваемые области

- `tools/`
- `plans/active/add-hf-upload-helper/*`

## Критерии приёмки

- в репозитории есть готовый runnable helper `upload_hf_model.py`;
- script не требует от пользователя писать собственный код вокруг basic flow;
- script покрывает logging + persisting weights + registration;
- проверки проходят;
- пользователю дано короткое объяснение, что future UX будет строиться через
  `Model` / `ClusterModel`, а не через прямую работу в backend UI.

## Риски

- нельзя обещать, что helper заменяет будущий DKP-native UX;
- нельзя привязать script к слишком специфичному локальному окружению.
