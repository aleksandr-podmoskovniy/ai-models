# ai-models

`ai-models` — модуль DKP для реестра и каталога AI/ML-моделей.

Текущий live baseline модуля — controller-owned publication/runtime path:

- `Model` / `ClusterModel` как DKP API каталога;
- controller publication в канонические OCI `ModelPack` artifacts;
- внутренний `DMCR` как publication backend;
- source mirror / upload staging в S3-compatible storage;
- runtime delivery path для consumer workloads.

Текущий порядок разработки:
1. стабилизировать ai-models-owned publication/runtime baseline;
2. добавить distribution/runtime topologies вроде `DMZ` registry и node-local cache;
3. затем hardening, patching и long-term support.

Что уже входит в репозиторий:
- метаданные DKP-модуля и user-facing документация;
- стабильные `config-values` для controller/runtime shell и object storage wiring;
- runtime templates для `DMCR`, controller shell и module-wide manifests;
- runtime/internal `values` и image-based Go hooks для модульной обвязки;
- `werf`, CI/CD и repo-local workflow для упаковки module-owned runtime images.

Текущий import/publication flow для моделей:
- канонический live путь идёт через `Model` / `ClusterModel`;
- remote source обрабатывается через controller-owned source mirror;
- upload source обрабатывается через controller-owned upload sessions;
- publication публикует OCI `ModelPack` артефакты во внутренний `DMCR`.

Начинать с:
- `AGENTS.md`
- `DEVELOPMENT.md`
- `docs/development/TZ.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `plans/active/`
