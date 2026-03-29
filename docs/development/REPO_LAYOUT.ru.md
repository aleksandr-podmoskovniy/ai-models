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
- `images/controller/` — канонический корень для будущего controller executable code;
- `images/hooks/` — image-based Go hooks для platform-side effects;
- `images/src-artifact/` — reusable source artifact fetch layer.

Правила:
- controller source, module-local `go.mod` и image build files должны жить под
  `images/controller/`, а не в top-level `controllers/`;
- code Go hooks должен жить под `images/hooks/`, а bundle должен импортировать
  его как `/hooks/go`;
- `images/` не должен превращаться в свалку unrelated tooling или docs.

### `hooks/`

Top-level `hooks/` нужен только для classic shell hooks, если такие когда-либо
понадобятся. Текущий модуль использует image-based Go hooks из `images/hooks`.

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
