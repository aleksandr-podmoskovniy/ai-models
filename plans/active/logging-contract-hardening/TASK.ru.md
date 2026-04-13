# Logging Contract Hardening

## Контекст

В phase-2 runtime у `ai-models` уже есть несколько собственных Go-owned
компонентов:

- controller manager;
- upload-gateway;
- publish-worker;
- artifact-cleanup;
- materialize-artifact.

Сейчас они пишут логи через stock `slog` handlers и по умолчанию стартуют в
`text`-режиме. Это расходится с platform-style structured logging, который уже
используется managed services и другими DKP-модулями.

Пользовательский operational запрос сейчас конкретный:

- перейти на JSON-by-default для наших компонентов;
- нормализовать поля под platform style;
- явно закрепить logging mode в deployment manifests;
- не смешивать этот срез с reconfiguration backend или `dmcr`.

## Постановка задачи

Сделать отдельный cross-cutting hardening slice для logging contract наших
Go-owned runtime components.

## Scope

- ввести custom JSON logging formatter в `images/controller/internal/cmdsupport`;
- сделать `json` дефолтом вместо `text` для controller и artifact-runtime;
- нормализовать envelope логов:
  - `level`
  - `ts`
  - `msg`
  - stable contextual attrs without stock `slog` drift;
- явно прокинуть `LOG_FORMAT=json` в live deployment surface, где это нужно для
  controller и runtime pods;
- зафиксировать и проверить severity vocabulary в наших собственных логах;
- покрыть formatter/defaults/env wiring focused tests;
- обновить active bundle notes и review notes по этому workstream.

## Non-goals

- не менять logging configuration внутреннего backend;
- не менять logging configuration `dmcr`;
- не делать общий observability redesign;
- не менять public API, values contract или current publication semantics;
- не тащить в этот срез unrelated controller structural cleanup.

## Затрагиваемые области

- `plans/active/logging-contract-hardening/*`
- `images/controller/internal/cmdsupport/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/internal/controllers/catalogcleanup/*`
- `templates/controller/deployment.yaml`

## Критерии приёмки

- наши Go-owned runtime components стартуют в `json` по умолчанию;
- `cmdsupport` выдаёт normalized JSON envelope с `ts`, lowercase `level` и
  сохранённым `msg`;
- controller, upload-gateway и spawned runtime paths не теряют `LOG_FORMAT`
  contract;
- deployment manifest явно задаёт `LOG_FORMAT=json` для live controller surface;
- focused tests покрывают:
  - formatter normalization;
  - bridge into `controller-runtime` / `klog`;
  - env propagation for runtime workers/cleanup;
- `backend` и `dmcr` не затронуты;
- пройдены focused checks и `make verify`.

## Риски

- можно незаметно сломать bridge между `slog`, `controller-runtime` и `klog`;
- можно выставить `json` только controller container и забыть spawned runtime;
- можно смешать formatter contract с отдельной политикой backend/`dmcr`, хотя
  это другой workstream.
