# План работ: refresh upstream version pins

## Current phase

Этап 1: managed MLflow inside the DKP module.

## Slice 1. Собрать текущий baseline и источники правды

### Цель

Понять, какие версии реально используются в `ai-models`, соседних DKP-проектах и локальном Deckhouse.

### Изменяемые области

- `plans/refresh-toolchain-upstreams/`

### Проверки

- ручная сверка `Makefile`, `build/components/versions.yml`, CI файлов
- ручная сверка соседних проектов и локального Deckhouse

### Артефакт

Есть понятный список компонентов и источников baseline.

## Slice 2. Подтвердить upstream versions

### Цель

Сверить каждый version pin с официальным upstream release source и отделить safe-compatible updates от major jumps.

### Изменяемые области

- `plans/refresh-toolchain-upstreams/`

### Проверки

- official release pages / tags для `dmt`, `module-sdk`, `operator-sdk`, `helm`, `werf`

### Артефакт

Есть решение, какие версии обновляем, какие оставляем и почему.

## Slice 3. Обновить repo pins

### Цель

Привести версии в коде, install scripts, workflows и docs к одному baseline.

### Изменяемые области

- `build/components/versions.yml`
- `Makefile`
- `tools/install-*.sh`
- `tools/module-sdk-wrapper.sh`
- `.github/workflows/`
- `.gitlab-ci.yml`
- `DEVELOPMENT.md`
- при необходимости `base_images.yml`

### Проверки

- `make fmt`
- `make verify`

### Артефакт

Пины обновлены и согласованы между всеми основными точками конфигурации.

## Rollback point

После Slice 2. На этом шаге уже известно, что именно нужно обновлять, но репозиторий ещё не изменён.

## Final validation

- `make verify`

## Фактические ограничения

- `hooks/batch` пока остаётся на library dependency `github.com/deckhouse/module-sdk v0.10.0`, хотя repo-level tool baseline уже `v0.10.3`.
- Причина не архитектурная, а средовая: в текущем окружении `proxy.golang.org` не резолвится, поэтому нельзя честно обновить `go.sum` для hook module без внешнего GOPROXY или доступного upstream cache.
- `.gitlab-ci.yml` сознательно сохраняет `WERF_VERSION: "2 stable"` по паттерну `modules-gitlab-ci`; текущий фиксируемый resolved baseline для repo metadata и GitHub Actions — `v2.63.1`.
