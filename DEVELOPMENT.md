# Development

## С чего начинать

1. Прочитать `AGENTS.md`.
2. Прочитать `docs/development/TZ.ru.md`, `docs/development/PHASES.ru.md` и `docs/development/REPO_LAYOUT.ru.md`.
3. Для нетривиальной задачи сначала создать task bundle в `plans/active/<slug>/`.
4. Только после этого переходить к коду, templates и values.

## Toolchain

- Go `1.25.7`
- DMT `0.1.68`
- Module SDK `0.10.3`
- Operator SDK `1.42.2`
- Helm `3.20.1`
- werf `2.63.1`
- Deckhouse lib-helm helper subset `1.70.1` in `templates/deckhouse-lib.tpl`
- Deckhouse base images `v0.5.55`

Полный source-of-truth по Deckhouse base images хранится в
`build/base-images/deckhouse_images.yml`, а `werf` поднимает только реально
используемое подмножество образов во время сборки.

Использовать `make ensure-tools`.

## Базовый цикл проверки

- `make fmt`
- `make lint`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`

`make helm-template` не ограничивается одним happy-path render: базовые values
из `fixtures/module-values.yaml` комбинируются со scenario overlays из
`fixtures/render/*.yaml`, и каждый сценарий рендерится в отдельный
`tools/kubeconform/renders/helm-template-*.yaml`.

Local render matrix also forces key custom resources (`Postgres`,
`PostgresClass`, `DexAuthenticator`, `Certificate`, `ServiceMonitor`) into the
render output. `kubeconform` continues to validate built-in Kubernetes objects
strictly and skips these custom kinds because the repo does not vendor a local
schema catalog for them.

## Internal Backend Upstream Fetch And Build Shell

Для phase-1 runtime внутренний backend тянется как внешний 3p source и живёт
только в build-only каталогах.

`images/backend/upstream.lock` должен фиксировать стабильный upstream release,
а не moving `main` snapshot. Для воспроизводимости в metadata хранится и release
tag, и resolved commit.

Основные команды:

- `make backend-fetch-source`
- `BACKEND_SOURCE_DIR=/path/to/upstream make backend-fetch-source`
- `make backend-shell-check`
- `make backend-build-ui`
- `make backend-build-dist`
- `make backend-build-image`
- `make backend-smoke-image`
- `make backend-build-local`
- `werf build`
- `make werf-build-dev`

Что делает fetch:

- подтягивает pinned upstream revision из internal build metadata или копирует явный `BACKEND_SOURCE_DIR` в build-only cache;
- не тащит `.git`, локальные cache/build артефакты и upstream `docs/`;
- проверяет обязательные upstream paths и locked version.

Для upstream-equivalent build shell используются:

- UI build через upstream frontend tree и локальный Yarn runtime из source tree;
- release distributions upstream backend;
- final image baseline, повторяющий upstream full image semantics, с возможным явным DKP overlay.

Локальный Docker loop не требует host `node`, `python` или `werf`: нужные toolchains поднимаются внутри контейнеров с pinned base images.

`werf build` требует git checkout с валидным `.git` и committed `werf-giterminism.yaml`.
Для локальной работы из грязного worktree использовать `make werf-build-dev`, а
строгий `werf build` оставлять для committed state и CI.

Для Go-based `werf` stages repo использует default
`GOPROXY=https://proxy.golang.org,direct`. Если нужен другой proxy или
внутренний mirror, его нужно передать через environment variable `GOPROXY`.

Для нестабильных сетей локальные Docker stages ставят apt-пакеты через retry-aware helper из internal build scripts.

Локальный `make backend-build-ui` всегда пересобирает throwaway worktree из build-only upstream cache, накладывает patch queue и только потом собирает frontend. `make backend-build-dist` и `make backend-build-image` используют уже подготовленный worktree.

`make backend-build-ui` держит `node_modules` в отдельном Docker volume, чтобы не раздувать build-only worktree.

Размер heap для frontend build управляется через `BACKEND_UI_MAX_OLD_SPACE_SIZE` и по умолчанию снижен до `4096`, чтобы local build не упирался в стандартные лимиты памяти Docker Desktop.

## Runtime prerequisites

Phase-1 runtime модуля ожидает платформенные prerequisites:

- `global.modules.publicDomainTemplate`;
- global HTTPS mode `CertManager` или `CustomCertificate`;
- модуль `user-authn` для module SSO;
- модуль `managed-postgres`, если используется `aiModels.postgresql.mode=Managed`.

Во время локального `helm template` custom resources рендерятся только если
соответствующие API доступны в `.Capabilities` или явно переданы через
`global.discovery.apiVersions`, поэтому базовый repo render остаётся на built-in
Kubernetes ресурсах.

Для `global.modules.https.mode=CustomCertificate` модуль использует
канонический DKP Go hooks flow из `images/hooks`, который собирается в
`go-hooks-artifact` и импортируется в bundle по пути `/hooks/go`.
`copy_custom_certificate` остаётся на стандартном helper из `module-sdk`, но
hooks delivery теперь выровнен с `gpu-control-plane` и `virtualization`.

## Правило по стадиям

Репозиторий развивается поэтапно.

- Сначала рабочий внутренний managed backend.
- Затем публичный API `Model` / `ClusterModel`.
- Затем hardening, distroless и controlled patching.

Не смешивать эти этапы в одном неуправляемом изменении.

## Основные документы

- `docs/development/TZ.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/TASK_TEMPLATE.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`
- `plans/README.md`
