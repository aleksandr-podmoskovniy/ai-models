# Пересборка DMCR image из актуальных исходников

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

Проблема находится в сборочно-публикационной цепочке: DMCR image был собран
нестандартно относительно controller/hooks и мог переиспользоваться старым
digest при свежей публикации module artifact.

## Постановка задачи

Сделать сборку DMCR такой же по форме, как сборка controller/hooks: отдельный
source artifact и отдельный build artifact. Это должно заставить werf учитывать
актуальные исходники DMCR при расчёте сборочных слоёв.

## Scope

- Привести `images/dmcr/werf.inc.yaml` к тому же source/build split-паттерну,
  который уже используется для controller и hooks images.

## Non-goals

- Не деплоить новую версию в кластер.
- Не менять ModuleSource, ModuleRelease или cluster resources.
- Не чистить registry и не удалять уже опубликованные image digests.
- Не менять DMCR runtime-контракт и логику публикации моделей.
- Не добавлять дополнительный payload-check в GitHub Actions.

## Затрагиваемые области

- `images/dmcr/werf.inc.yaml`
- `plans/active/ci-dmcr-image-consistency/*`

## Критерии приёмки

- Изменения в `images/dmcr/**` инвалидируют source artifact DMCR тем же способом,
  что и для controller/hooks.
- GitHub Actions workflow остаются без дополнительного payload-check step.
- `make verify` проходит.

## Риски

- Правка не добавляет отдельную проверку содержимого опубликованного image.
  Если werf снова переиспользует неподходящий digest, это будет видно по
  rollout/registry inspection, а не по отдельному Actions guard.
