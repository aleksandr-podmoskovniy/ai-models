# Перенос upstream MLflow packaging и phase-1 runtime в модуль ai-models

## Контекст

Phase-1 shell для `ai-models` уже выровнен под managed `MLflow`: в репозитории есть базовый DKP module root, разделение `config-values` и runtime-only `values`, shell для `werf` и заготовка patch bundle в `images/mlflow/patches/README.md`.

Следующий крупный шаг: перестать думать про `MLflow` как про абстракцию и определить реальный reproducible path, по которому модуль будет:
- тянуть upstream source как внешний 3p artifact, а не хранить snapshot в git tree модуля;
- собирать собственный `MLflow` image из upstream;
- маппить stage-1 values в реальные secrets, env, args и templates;
- удерживать controlled patching и rebase discipline.

Локальный upstream source сейчас лежит в `/Users/myskat_90/flant/aleksandr-podmoskovniy/mlflow` на commit `7e61730de0ddba0373186bfb67ce946af1371f35` и содержит версию `3.11.1.dev0`. Локальный donor chart лежит в `/Users/myskat_90/flant/aleksandr-podmoskovniy/charts/bitnami/mlflow` и сейчас зафиксирован на chart version `5.2.1` с `appVersion: 3.3.2`.

Предварительная проверка показала важные ограничения и опорные правила:
- upstream release wheel собирается через `dev/build.py --package-type release` после сборки UI;
- upstream image release публикует как минимум `mlflow:<version>` и `mlflow:<version>-full`, где `-full` строится по модели `mlflow[extras,azure,db,gateway,genai]`;
- upstream `dev/build.py` ожидает `.git`, инициализированные submodules и уже собранные UI assets;
- в source tree нет `mlflow/server/js/build`, значит frontend build обязателен;
- UI build требует Node.js `^22.19.0`;
- Bitnami chart полезен как donor по runtime semantics, но не подходит как runtime artifact или platform contract для DKP module;
- если DKP-модулю понадобится отклонение от upstream `-full` image, например отдельный `auth` overlay для basic auth, это должно быть оформлено как явный и минимальный delta layer, а не как новая кастомная сборка `MLflow`.

## Постановка задачи

Нужно спланировать и затем реализовать воспроизводимый phase-1 путь для `MLflow registry backend` внутри `ai-models`: от fetch pinned upstream 3p source и controlled patch queue до upstream-equivalent release build с UI и штатными компонентами, а затем до DKP-native runtime wiring для tracking/UI, PostgreSQL, S3-compatible artifacts, ingress/https, auth и observability.

## Scope

- зафиксировать, какой upstream baseline мы тянем как внешний 3p source и как обновляем его дальше;
- определить repo-local layout для pinned upstream metadata, source-artifact stages, build scripts и patch queue по паттернам, совместимым с `gpu-control-plane`;
- определить reproducible build flow из upstream source, эквивалентный upstream release process: UI build, release wheel, runtime image, smoke checks;
- определить, какие semantics берем из Bitnami chart и какие сознательно не переносим;
- определить phase-1 runtime shape для DKP-модуля: tracking/UI first, PostgreSQL, S3-compatible storage, ingress/https, basic auth, metrics, без урезания самого `MLflow` относительно upstream release image;
- описать маппинг между текущими `config-values` / `values` и будущими runtime templates/secrets/env/args;
- определить validation loop, который будет подтверждать source import, patch application, image build и module template wiring.

## Non-goals

- не внедрять `Model`, `ClusterModel`, контроллеры публикации и synchronization semantics из phase 2;
- не использовать Bitnami chart как runtime dependency, subchart или deployable artifact;
- не использовать Bitnami image как production image модуля;
- не уводить задачу в distroless, supply-chain hardening и глубокие security-гейты phase 3;
- не хранить upstream source snapshot в git tree модуля;
- не патчить upstream ad-hoc вне repo-local patch queue;
- не придумывать собственный "облегчённый" вариант `MLflow`, который расходится с upstream release/full build без отдельного решения и документации;
- не пытаться превращать phase-1 runtime contract DKP-модуля в копию всего upstream deployment surface.

## Затрагиваемые области

- `plans/mlflow-upstream-packaging/`
- `images/src-artifact/`
- `images/mlflow/`
- `.werf/stages/`
- `base_images.yml`
- `build/components/versions.yml`
- `openapi/`
- `templates/`
- `docs/`
- `DEVELOPMENT.md`
- `.github/workflows/`
- `.gitlab-ci.yml`
- `Makefile`
- `Taskfile.yaml`

## Критерии приёмки

- есть явное решение, откуда и в каком формате подтягивается upstream `MLflow`, с pinned tag/commit и описанным fetch/update path;
- repo layout для upstream metadata, source-artifact, build scripts и patch queue не смешивает upstream code, DKP templates и docs;
- git tree модуля не содержит vendored `src/mlflow` или другой полной копии upstream source;
- build strategy даёт upstream-equivalent результат: release wheel с UI assets и runtime image, повторяющий upstream release/full semantics;
- phase-1 runtime shape явно ограничен tracking/UI backend-ом и не тащит Bitnami `run` component как обязательную часть baseline;
- есть согласованный mapping для PostgreSQL, S3-compatible artifacts, ingress/https, auth и metrics между module values и runtime manifests;
- любое отклонение от upstream `-full` image явно выделено как отдельный overlay и объяснено по назначению;
- patch/rebase discipline описан так, чтобы upstream edits были воспроизводимыми и проверяемыми;
- определен validation loop для source import, patch apply, image smoke и repo-level checks.

## Риски

- локальный upstream baseline сейчас dev-oriented (`3.11.1.dev0`), поэтому до реализации может понадобиться перепин на стабильный tag;
- если взять слишком много из Bitnami chart, модуль начнет повторять чужую архитектуру вместо DKP-native runtime;
- если взять слишком мало из donor chart, можно заново изобрести уже известные runtime детали по auth, artifact serving и DB upgrades;
- UI build и submodule-dependent части upstream могут сделать packaging неhermetic, если sync/build flow будет описан расплывчато;
- незаметное расхождение с upstream release/full build приведет к тому, что модуль будет тестироваться и эксплуатироваться не на том `MLflow`, который ожидается пользователями;
- неправильное обращение с `auth` и другими DKP-specific additions раздует delta относительно апстрима и осложнит patch/rebase дальше.
