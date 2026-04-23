# Согласованность DMCR image digest в CI

## Контекст

В рабочем кластере `k8s.apiac.ru` новый pod `dmcr-8955d7b4b-4xfgz` не смог
стартовать после выкладки свежего module image `main`.

Проверка registry показала:

- module image `ghcr.io/aleksandr-podmoskovniy/modules/ai-models:main`
  опубликован свежим артефактом;
- его `images_digests.json` ссылается на `dmcr`
  `sha256:23bff80ee996a2580f59203a7b88233f0b982f6dfb75c77e6ec2acd9078e2627`;
- этот `dmcr` image старее уже работавшего pod image и не содержит регистрацию
  storage driver `sealeds3`;
- этот же image не содержит флаг `dmcr-cleaner --schedule`;
- templates уже ожидают новый DMCR runtime, поэтому pod падает с
  `StorageDriver not registered: sealeds3` и `unknown flag: --schedule`.

Проблема находится в сборочно-публикационной цепочке: module artifact
может быть опубликован с устаревшим digest внутреннего DMCR image.

## Постановка задачи

Сделать так, чтобы GitHub Actions и werf-сборка не могли тихо опубликовать
module artifact, в котором templates ожидают новый DMCR runtime, а
`images_digests.json` указывает на старый или непригодный `dmcr` image.

## Scope

- Привести `images/dmcr/werf.inc.yaml` к тому же source/build split-паттерну,
  который уже используется для controller и hooks images.
- Добавить CI-проверку опубликованного module image:
  - прочитать `images_digests.json`;
  - достать `dmcr` image по digest;
  - проверить наличие обязательных runtime-признаков в бинарях DMCR.
- Включить проверку в build workflow после публикации module image.
- Включить ту же проверку в deploy workflow перед публикацией release.

## Non-goals

- Не деплоить новую версию в кластер.
- Не менять ModuleSource, ModuleRelease или cluster resources.
- Не чистить registry и не удалять уже опубликованные image digests.
- Не менять DMCR runtime-контракт и логику публикации моделей.

## Затрагиваемые области

- `images/dmcr/werf.inc.yaml`
- `.github/workflows/build.yaml`
- `.github/workflows/deploy.yaml`
- `tools/ci/*`
- `plans/active/ci-dmcr-image-consistency/*`

## Критерии приёмки

- Изменения в `images/dmcr/**` инвалидируют source artifact DMCR тем же способом,
  что и для controller/hooks.
- CI-проверка падает, если module artifact ссылается на `dmcr` image без
  регистрации `sealeds3`.
- CI-проверка падает, если `dmcr-cleaner` image payload не содержит поддержку
  `--schedule`.
- CI-проверка падает, если `dmcr-direct-upload` image payload не содержит новый
  проверочный код прямой загрузки.
- Build workflow после публикации module image проверяет фактически
  опубликованный artifact.
- Deploy workflow проверяет artifact перед выпуском release.
- Локальная shell-проверка нового CI script проходит.
- Негативная локальная проверка против текущего сломанного `main` tag
  воспроизводимо падает.
- `make verify` проходит.

## Риски

- Проверка выполняется уже после build/push, поэтому она не отменяет сам push,
  но делает workflow красным и не даёт считать artifact годным.
- Если module image repo отличается от repo внутренних images, понадобится
  явный override через переменную окружения.
- Проверка использует строки из бинарей как практический guardrail; она не
  заменяет полноценные unit/integration tests runtime-кода.
