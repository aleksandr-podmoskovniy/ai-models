## 1. Заголовок

Phase reset: ai-models-owned `ModelPack` publisher и полное удаление `MLflow`
baseline из live repo

## 2. Контекст

Текущий репозиторий всё ещё несёт сильный phase-1 baseline вокруг внутреннего
`MLflow` backend:

- `README*`, `docs/*` и `TZ.ru.md` описывают native `MLflow` auth/workspaces;
- `templates/backend/*`, `images/backend/*`, `openapi/config-values.yaml`,
  `tools/*` и test guards привязаны к `MLflow`;
- phase-2 publication path всё ещё опирается на external brand-specific
  publisher implementation (`KitOps`) вместо ai-models-owned publisher.

Пользовательский запрос меняет baseline жёстко:

- не держать fallback paths;
- перестать обслуживать `MLflow` как центр архитектуры;
- перейти на ai-models-owned publication/runtime path;
- жёстко вычистить repo от `MLflow`-specific live surfaces, а не накапливать
  dual-stack compatibility.

Это уже не incremental cleanup, а явный reset workstream, который меняет
project phase narrative, runtime shell и repo shape.

## 3. Постановка задачи

Нужно планомерно перевести модуль на новый live baseline:

- canonical publication contract остаётся `Model` / `ClusterModel` plus
  controller-owned OCI artifact flow;
- publisher `ModelPack` должен стать ai-models-owned implementation, а не
  `KitOps` wrapper;
- `MLflow` backend, auth/workspace shell, templates, tools, docs и values
  должны быть удалены из live repo;
- migration должна идти forward-only:
  новые slices не оставляют fallback на `MLflow` или `KitOps`, а удаляют старые
  live paths в том же workstream, когда replacement готов.
- первый native publisher cut может сохранять текущий filesystem-based worker
  input (`checkpointDir`), но больше не зависит от external binary и сам пишет
  live OCI manifest/config/blob contract.

## 4. Scope

- зафиксировать новый project baseline в docs/development и user-facing docs;
- спроектировать и внедрить ai-models-owned `ModelPack` publisher;
- перевести publication path off `KitOps`;
- зафиксировать первый native publisher cut как bounded replacement:
  current worker-local checkpoint directory in, controller-owned OCI artifact
  out, а затем добить source-to-registry streaming/object-source publication
  тем же canonical workstream;
- удалить `MLflow`-owned backend shell:
  - `images/backend/*`
  - `templates/backend/*`
  - related values/OpenAPI/tooling/docs/tests;
- удалить retired PostgreSQL metadata shell, который остался только как
  phase-1 config/render contract:
  - `aiModels.postgresql`
  - `templates/database/*`
  - related render fixtures and validation checks;
- пересобрать repo layout и phase narrative вокруг:
  - controller
  - `DMCR`
  - ai-models-owned publish/distribution/runtime surfaces;
- вычистить active docs и repo rules от утверждений, что module baseline
  зависит от `MLflow`.

## 5. Non-goals

- не делать в этом bundle сразу node-local cache daemon, `FUSE`, lazy-loading
  или stream-to-VRAM runtime;
- не делать dual-stack migration, где старый `MLflow` path живёт “на всякий
  случай” рядом с новым baseline;
- не оставлять в active repo `KitOps` как fallback publisher после landing
  ai-models-owned implementation;
- не тянуть новые public spec knobs только ради backend/runtime internals.

## 6. Затрагиваемые области

- `README.md`
- `README.ru.md`
- `docs/README.md`
- `docs/README.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `docs/development/TZ.ru.md`
- `docs/development/PHASES.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/backend/*`
- `templates/database/*`
- `templates/module/*`
- `images/backend/*`
- `images/controller/internal/adapters/modelpack/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `tools/*`
- `tools/helm-tests/*`
- `Makefile`
- CI / werf wiring where backend or `MLflow` still appears

## 7. Критерии приёмки

- Есть отдельный reset bundle, который явно фиксирует отказ от `MLflow`
  baseline и от fallback mindset.
- В active repo не остаётся live runtime/docs/config surfaces, которые
  утверждают, что ai-models module зависит от native `MLflow` auth/workspaces.
- `images/backend/*` и `templates/backend/*` либо полностью удалены, либо
  сведены к нулю в рамках нового baseline; retained stubs without live runtime
  role недопустимы.
- В publication path больше нет `KitOps`-based live publisher; canonical
  publisher — ai-models-owned implementation.
- Первый native publisher cut использует текущий worker-local `checkpointDir`,
  но уже сам публикует OCI artifact в shape, которую без изменений читает live
  `internal/adapters/modelpack/oci` materializer.
- User-facing and engineering docs согласованы с новым baseline:
  - `README*`
  - `docs/README*`
  - `docs/CONFIGURATION*`
  - `docs/development/TZ.ru.md`
  - `docs/development/PHASES.ru.md`
  - `docs/development/REPO_LAYOUT.ru.md`
- OpenAPI/values/templates больше не тащат `MLflow`-specific config contract.
- User-facing module contract больше не содержит `postgresql`, а live renders
  больше не создают `Postgres` / `PostgresClass`.
- Repo verification проходит без backend/`MLflow` artifacts:
  - no stale helm-render expectations
  - no stale scripts/tests/CI references
  - `make verify` green on final state.

## 8. Риски

- это меняет product baseline, а не только implementation detail;
- можно удалить `MLflow` surface быстрее, чем новый ai-models-owned publisher
  станет рабочим, и остаться без publication path;
- можно недочистить docs/openapi/tests и оставить repo в противоречивом
  состоянии;
- можно превратить migration в giant diff без defendable slices, если не держать
  forward-only but bounded execution.
