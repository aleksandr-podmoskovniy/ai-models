# Development

## С чего начинать

1. Прочитать `AGENTS.md`.
2. Прочитать `docs/development/TZ.ru.md`, `docs/development/PHASES.ru.md` и `docs/development/REPO_LAYOUT.ru.md`.
3. Для нетривиальной задачи сначала создать task bundle в `plans/active/<slug>/`.
4. Только после этого переходить к коду, templates и values.

## Toolchain

- Go `1.25.7`
- DMT `0.1.69`
- Module SDK `0.10.0`
- Operator SDK `1.42.2`
- Helm `3.20.1`
- werf `2.63.1`
- Deckhouse lib-helm helper subset `1.70.1` in `templates/deckhouse-lib.tpl`
- golangci-lint `2.11.1`
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

Local render matrix still covers module-owned HTTPS and monitoring branches.
`kubeconform` validates built-in Kubernetes objects strictly and skips only the
custom kinds whose schemas are not vendored locally.

`make helm-template` also runs repo-local semantic assertions over rendered
output, so stale legacy surfaces are caught before cluster startup.

`werf build` требует git checkout с валидным `.git` и committed `werf-giterminism.yaml`.
Для локальной работы из грязного worktree использовать `make werf-build-dev`, а
строгий `werf build` оставлять для committed state и CI.

Для Go-based `werf` stages repo использует default
`GOPROXY=https://proxy.golang.org,direct`. Если нужен другой proxy или
внутренний mirror, его нужно передать через environment variable `GOPROXY`.

## Runtime prerequisites

Phase-1 runtime модуля ожидает платформенные prerequisites:

- `global.modules.publicDomainTemplate`;
- global HTTPS mode `CertManager` или `CustomCertificate`;

Во время локального `helm template` custom resources рендерятся только если
соответствующие API доступны в `.Capabilities` или явно переданы через
`global.discovery.apiVersions`, поэтому базовый repo render остаётся на built-in
Kubernetes ресурсах.

Для `global.modules.https.mode=CustomCertificate` модуль использует
канонический DKP Go hooks flow из `images/hooks`, который собирается в
`go-hooks-artifact` и импортируется в bundle по пути `/hooks/go`.
`copy_custom_certificate` остаётся на стандартном helper из `module-sdk`, но
hooks delivery теперь выровнен с `gpu-control-plane` и `virtualization`.

Для external module bundle `Chart.yaml` сознательно не включается в образ
bundle. Это позволяет Deckhouse `helm3lib` использовать internal synthesized
chart path, который игнорирует каталоги `hooks/` и `images/` при `helm install`
и не упирается в Helm per-file limit для Go hooks binaries.

## Правило по стадиям

Репозиторий развивается поэтапно.

- Сначала ai-models-owned publication/runtime baseline.
- Затем distribution topology и runtime delivery hardening.
- Затем distroless, controlled patching и long-term support.

Не смешивать эти этапы в одном неуправляемом изменении.

## Основные документы

- `docs/development/TZ.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/TASK_TEMPLATE.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`
- `plans/README.md`
