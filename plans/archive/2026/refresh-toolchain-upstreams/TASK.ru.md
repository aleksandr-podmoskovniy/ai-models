# Обновление toolchain и upstream version pins для ai-models

## Контекст

В репозитории уже есть пины для `dmt`, `module-sdk`, `operator-sdk`, `helm`, `werf` и Deckhouse base images, но они собирались по частям и уже частично разошлись:
- с соседними DKP-проектами;
- с текущим локальным Deckhouse baseline;
- с официальными upstream release channels.

Сейчас нужно выровнять эти версии осознанно, до того как publish/deploy flow пойдёт дальше.

## Постановка задачи

Нужно пройтись по текущим version pins модуля, сверить их:
- с реальным usage внутри `ai-models`;
- с соседними проектами (`n8n-d8`, `gpu-control-plane`, `gpu`, локальный `deckhouse`);
- с официальными upstream releases по первичным источникам;

и затем обновить repo pins, CI и docs до актуального совместимого baseline.

## Scope

- зафиксировать, какие компоненты считаются частью repo toolchain/runtime baseline;
- проверить актуальные версии `dmt`, `module-sdk`, `operator-sdk`, `helm`, `werf`;
- проверить актуальный локальный Deckhouse baseline для `base_images` и `deckhouse_lib_helm`;
- обновить version pins в `Makefile`, `build/components/versions.yml`, install scripts, workflows, docs и связанных служебных файлах;
- явно оставить без изменений те пины, которые уже актуальны или не должны обновляться по совместимости.

## Non-goals

- не переписывать runtime templates или MLflow deployment shape;
- не тащить в репозиторий настоящий dependency на `deckhouse_lib_helm`, если модуль всё ещё использует локально зафиксированные helper templates;
- не делать upgrade ради major-version скачка без проверки совместимости;
- не менять Go/module dependency graph для runtime-кода кроме того, что прямо нужно для toolchain refresh.

## Затрагиваемые области

- `plans/refresh-toolchain-upstreams/`
- `build/components/versions.yml`
- `Makefile`
- `tools/install-*.sh`
- `tools/module-sdk-wrapper.sh`
- `.github/workflows/`
- `.gitlab-ci.yml`
- `DEVELOPMENT.md`
- при необходимости `base_images.yml`

## Критерии приёмки

- repo version pins сверены с upstream и приведены к одному baseline;
- `dmt`, `module-sdk`, `operator-sdk`, `helm`, `werf` больше не расходятся между основными точками конфигурации репозитория;
- если какой-то компонент не обновлён до абсолютного `latest`, в репозитории есть явное объяснение по совместимости или по источнику baseline;
- `make verify` проходит после обновления;
- из diff видно, что обновление носит характер controlled refresh, а не случайного churn.

## Риски

- для `helm` абсолютный latest может означать уже major v4, что не равно безопасному baseline для DKP module workflow;
- `werf` и CI action versions могут обновляться быстрее, чем локальные привычные workflow;
- часть upstream сайтов может быть недоступна, поэтому важно отделять подтверждённые official releases от inference по соседним проектам.
