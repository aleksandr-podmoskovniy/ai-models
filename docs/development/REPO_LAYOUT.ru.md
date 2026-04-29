# Структура репозитория ai-models

## Принцип

Репозиторий остаётся `DKP module root`, а не превращается в отдельный operator
repo. Поэтому layout должен разделять:
- module shell и runtime manifests;
- public API и executable runtime code;
- publication/distribution runtime boundaries;
- Deckhouse hooks.

## Chart shell

### `Chart.yaml`, `Chart.lock`, `charts/`

Module chart остаётся DKP-style chart, как в `gpu-control-plane` и
`virtualization`.

Правила:
- `Chart.yaml` объявляет library dependency на `deckhouse_lib_helm`;
- vendored dependency archive живёт в `charts/` и является частью render
  contract для `helm template`;
- release `bundle` должен забирать `Chart.yaml`, `Chart.lock` и
  vendored `charts/`, иначе release payload начинает расходиться с live render
  path;
- release `bundle` также должен забирать `monitoring/`, если модуль уже
  держит Prometheus rules или Grafana dashboards как часть install contract;
- `.helmignore` не должен исключать `charts/`, иначе dependency physically
  лежит в репо, но не участвует в render path;
- repo-local helper fork в `templates/` не должен становиться primary source of
  truth для `helm_lib`; допустимы только узкие, явно обоснованные overrides.

## Каталоги

### `templates/`

`templates/` хранит только module manifests и helper templates.

Подкаталоги:
- `templates/module/` — namespace, registry secret и прочая module-wide обвязка;
- `templates/dmcr/` — manifests внутреннего publication backend;
- `templates/kube-rbac-proxy/` — shared helper templates для защищённого
  metrics scrape path у module-owned runtime components;
- `templates/controller/` — runtime manifests phase-2 controller shell.

Правила:
- не складывать всё в корень `templates/`;
- не размещать controller code или generated API artifacts в `templates/`;
- manifests phase-2 controller должны жить в отдельном `templates/controller/`,
  а не смешиваться с историческим backend shell;
- если retired template subtree больше не участвует в live render path, его
  надо удалять целиком, а не оставлять как пустой namespace под будущий шум;

### `crds/`

`crds/` хранит generated install artifacts для module-owned CRD rollout.

Правила:
- generated CRD schema для `Model` / `ClusterModel` живёт в корневом `crds/`;
- CRD не рендерятся из `templates/`;
- module hooks доставляют их в cluster lifecycle через ensure-CRDs path.

### `api/`

`api/` зарезервирован под публичные DKP API types модуля.

Здесь появятся:
- `Model`;
- `ClusterModel`;
- общие defaults, validation и generated artifacts.

Сырые сущности внутреннего backend engine сюда не попадают.

### `openapi/`

`openapi/` разделяется так же, как в virtualization:

- `openapi/config-values.yaml` — только стабильный user-facing module contract;
- `openapi/values.yaml` — internal/computed/runtime wiring.

Правила:
- не выносить runtime/materializer adapter specifics в `config-values.yaml`;
- не делать public contract заложником текущей implementation brand;
- derived/internal values должны жить в `values.yaml`, templates/helpers и
  image/runtime code, а не в user-facing config surface.

### `docs/`

`docs/` хранит публичную документацию модуля в DKP docs layout.

Правила:
- hand-written pages (`README`, `USER_GUIDE`, `ADMIN_GUIDE`, `EXAMPLES`,
  `FAQ`, `CONFIGURATION`) объясняют workflows, эксплуатацию, примеры и
  ограничения;
- generated schema entrypoints (`docs/CR.md`, `docs/CR.ru.md`) не являются
  местом для ручного описания CRD fields;
- `docs/CR*.md` должны оставаться frontmatter plus `<!-- SCHEMA -->`;
- если нужно изменить описание полей CRD или module config, менять надо
  canonical OpenAPI/API source, а не rendered docs page;
- для docs changes обязательна явная проверка source of truth: OpenAPI/API,
  templates, hand-written docs или generated entrypoint;
- `make lint-docs` проверяет docs markers; при изменении generated docs source
  нужен соответствующий render/codegen check.

### `images/`

`images/` хранит image definitions и executable runtime code модуля.

Текущее разделение:
- `images/dmcr/` — module-local internal publication backend image и исходники
  собственного `dmcr` binary;
- `images/distroless/` — module-local distroless relocation layer для
  собственного runtime кода;
- `images/hooks/` — Deckhouse Go hooks, доставляемые в bundle как `/hooks/go`;
- `images/controller/` — канонический корень для controller executable code.

Правила:
- controller source, module-local `go.mod` и image build files должны жить под
  `images/controller/`, а не в top-level `controllers/`;
- собственные runtime images модуля должны строиться от module-local
  `images/distroless/`, а не тянуть `base/distroless` напрямую в конечные
  controller/runtime images;
- Go hooks source, module-local `go.mod` и werf wiring для них должны жить под
  `images/hooks/`, а не в top-level `hooks/batch`;
- если image-stage не несёт отдельной runtime/build boundary, его нужно
  встраивать обратно в owning image definition, а не держать как пустой alias
  каталог под `images/`;
- mapped Deckhouse base images из `build/base-images/deckhouse_images.yml`
  должны использоваться везде, где для stage уже есть подходящий builder/runtime
  image; raw external `from:` допустим только как явный временный debt, если в
  base-image map пока нет эквивалента;
- `images/` не должен превращаться в свалку unrelated tooling или docs.

### `monitoring/`

`monitoring/` хранит module-owned monitoring payload, который должен ехать в
release bundle так же, как в `virtualization` и `gpu-control-plane`.

Правила:
- `monitoring/prometheus-rules/` и `monitoring/grafana-dashboards/` считаются
  install/release contract, а не локальными dev-only артефактами;
- если monitoring assets живут в репозитории, `werf` bundle не должен их
  терять;
- build-only metadata вроде `build/components/versions.yml` не надо по
  инерции тащить в bundle, если это не install-time contract модуля.

### `docs/development/`

`docs/development/` хранит durable engineering baseline репозитория:
- `TZ.ru.md` — product and phase baseline;
- `PHASES.ru.md` — stage boundaries;
- `REPO_LAYOUT.ru.md` — layout discipline;
- `CODEX_WORKFLOW.ru.md` — рабочий цикл;
- `TASK_TEMPLATE.ru.md` — canonical bundle template;
- `REVIEW_CHECKLIST.ru.md` — final review checklist.

Правила:
- workflow docs не должны противоречить `AGENTS.md`, `.codex/README.md`,
  skills и agent profiles;
- если меняются `CODEX_WORKFLOW.ru.md`, `TASK_TEMPLATE.ru.md` или
  `REVIEW_CHECKLIST.ru.md`, это governance task, а не incidental docs cleanup;
- для такого изменения обязателен `make lint-codex-governance`.

## Werf shell

Root `werf` должен оставаться module-oriented, как в `virtualization` и
`gpu-control-plane`, а не держать ad-hoc mirror/proxy logic по отдельным image
stage files.

Правила:
- root `werf.yaml` должен объявлять общий build-shell context:
  `SOURCE_REPO`, `SOURCE_REPO_GIT`, `GOPROXY`, `DistroPackagesProxy`;
- reusable package-manager helpers должны жить в `.werf/` и подключаться как
  shared templates, а не копироваться вручную по каждому `werf.inc.yaml`;
- git source fetches и package installs должны использовать общий mirror/proxy
  discipline, а не локальные hardcoded `github.com` / distro mirror paths там,
  где модуль уже умеет принимать общий proxy/mirror context.

### `hooks/`

Top-level `hooks/` зарезервирован только для редких classic/shell hook
сценариев. В phase-1 у `ai-models` собственных top-level hooks нет: module
hooks доставляются через `images/hooks` в `/hooks/go`.

Правила:
- не смешивать top-level shell/classic hooks и Go hooks в одном механизме
  delivery;
- не складывать module-local Go hooks source в корень `hooks/`;
- не держать временные workaround paths вроде `batchhooks` в корне chart.

## CI shell

CI должен следовать module-oriented паттерну и не выглядеть как временная
обвязка.

Правила:
- GitHub Actions для `ai-models` должны быть выровнены по паттерну
  `gpu-control-plane`: основной workflow в `build.yaml`, ручной publish в
  `deploy.yaml`;
- GitLab CI должен оставаться stage-oriented: `lint`, `verify`, `build`,
  `deploy_dev`, `cleanup`;
- и GitHub, и GitLab должны использовать repo-local `make` commands как
  канонический entrypoint для lint/verify.
