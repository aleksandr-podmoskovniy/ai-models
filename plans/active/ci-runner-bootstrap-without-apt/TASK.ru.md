# CI Runner Bootstrap Without Apt

## Контекст

`ai-models` workflow в GitHub Actions запускается на ARC runner pool `ai-models-runners`, где базовый runner image не гарантирует наличие `make` и не должен зависеть от runtime `apt-get install`.

Текущий `build.yaml` делает bootstrap toolchain через:

- `sudo apt-get update`
- `sudo apt-get install build-essential`

Этот путь уже ломается на недоступности `archive.ubuntu.com`, из-за чего CI падает до реальных repo-level проверок.

## Постановка задачи

Убрать зависимость `lint` и `verify` jobs от runtime `apt` bootstrap и перевести их на repo-local CI wrapper scripts, которые выполняют эквивалентные проверки без требования `make` в runner image.

## Scope

- добавить repo-local CI wrapper script(s) под `tools/ci/`;
- переключить `.github/workflows/build.yaml` на новые wrapper script(s);
- сохранить текущий semantic набор проверок для `lint` и `verify`, насколько это возможно без `make` bootstrap.

## Non-goals

- не менять cluster-side ARC runner image и AutoscalingRunnerSet template;
- не менять `deploy.yaml`;
- не пересобирать весь verify pipeline в другую архитектуру;
- не трогать runtime/module behavior, values, API или templates.

## Критерии приёмки

- `lint` и `verify` jobs в `.github/workflows/build.yaml` больше не используют `apt-get install build-essential`;
- workflow не требует `make` в runner image для прохождения `lint`/`verify`;
- repo-local wrapper script(s) проходят хотя бы узкие локальные проверки на синтаксис и базовый smoke path;
- чужие незавершённые изменения в `ai-models` не затронуты.
