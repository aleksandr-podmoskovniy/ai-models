# Очистка production repo от sample-значений и явных заглушек

## Контекст

Репозиторий уже используется как рабочий DKP module root для `ai-models`, но в нём ещё остались отдельные demo-следы:
- fixture-значения с `example.com`, `ghcr.io/example` и fake docker config;
- wording, который выглядит как временная заготовка, а не как production repo metadata.

Сейчас нужно подчистить это без изменения архитектуры и без переработки runtime logic.

## Постановка задачи

Нужно убрать из репозитория явные sample/placeholder значения и формулировки, которые плохо выглядят в production-проекте, сохранив рабочий render/verify loop.

## Scope

- найти production-facing и repo-facing sample/placeholder значения;
- очистить render fixture от `example/*fake` значений;
- привести тексты в schema/task descriptions к более нормальному production wording там, где они явно выглядят как заглушки;
- сохранить проходящий `make verify`.

## Non-goals

- не менять runtime contract модуля;
- не менять deployment shape или MLflow wiring;
- не убирать технические defaults, если они реально нужны для рендера и шаблонов;
- не переписывать docs и schema шире, чем нужно для cleanup.

## Затрагиваемые области

- `plans/cleanup-production-placeholders/`
- `fixtures/module-values.yaml`
- `openapi/values.yaml`
- `Taskfile.yaml`

## Критерии приёмки

- в рабочем дереве больше нет явных demo-значений вида `example.com`, `ghcr.io/example` и fake docker config в repo fixtures;
- wording в затронутых production-facing файлах больше не выглядит как временная заглушка;
- `make verify` проходит;
- cleanup выглядит как controlled polish, а не как случайный churn.

## Риски

- fixture-значения участвуют в `helm template`, поэтому нельзя просто удалить обязательные поля;
- излишняя чистка может задеть internal schema fields, которые нужны hooks и templates.
