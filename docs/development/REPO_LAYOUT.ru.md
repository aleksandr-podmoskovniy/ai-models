# Структура репозитория ai-models

## Принцип

Репозиторий остаётся `DKP module root`, а не превращается в отдельный operator
repo. Поэтому layout должен разделять:
- module shell и runtime manifests;
- public API и executable runtime code;
- internal backend packaging;
- Deckhouse hooks.

## Каталоги

### `templates/`

`templates/` хранит только module manifests и helper templates.

Подкаталоги:
- `templates/module/` — namespace, registry secret и прочая module-wide обвязка;
- `templates/backend/` — runtime manifests внутреннего backend;
- `templates/database/` — declarative managed-postgres resources;
- `templates/auth/` — auth/SSO integration resources.

Правила:
- не складывать всё в корень `templates/`;
- не размещать controller code или generated API artifacts в `templates/`;
- runtime manifests backend должны жить вместе, чтобы checksum/include paths были локальны и понятны.

### `api/`

`api/` зарезервирован под публичные DKP API types модуля.

Здесь появятся:
- `Model`;
- `ClusterModel`;
- общие defaults, validation и generated artifacts.

Сырые сущности внутреннего backend engine сюда не попадают.

### `images/`

`images/` хранит image definitions и executable runtime code модуля.

Текущее разделение:
- `images/backend/` — internal backend engine packaging;
- `images/hooks/` — Deckhouse Go hooks, доставляемые в bundle как `/hooks/go`;
- `images/controller/` — канонический корень для будущего controller executable code;
- `images/src-artifact/` — reusable source artifact fetch layer.

Правила:
- controller source, module-local `go.mod` и image build files должны жить под
  `images/controller/`, а не в top-level `controllers/`;
- Go hooks source, module-local `go.mod` и werf wiring для них должны жить под
  `images/hooks/`, а не в top-level `hooks/batch`;
- `images/` не должен превращаться в свалку unrelated tooling или docs.

### `hooks/`

Top-level `hooks/` зарезервирован только для редких classic/shell hook
сценариев. В phase-1 у `ai-models` собственных top-level hooks нет: module
hooks доставляются через `images/hooks` в `/hooks/go`.

Правила:
- не смешивать top-level shell/classic hooks и Go hooks в одном механизме
  delivery;
- не складывать module-local Go hooks source в корень `hooks/`;
- не держать временные workaround paths вроде `batchhooks` в корне chart.

Правило по database bootstrap:
- если используется `managed-postgres`, создание database/user должно оставаться
  declarative через ресурсы в `templates/database/`;
- imperative hook добавляется только тогда, когда platform-side effect нельзя
  выразить через declarative module manifests.

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
