# ai-models

`ai-models` — модуль DKP для реестра и каталога AI/ML-моделей.

Текущая runtime-реализация модуля уже покрывает базовые сервисы registry
внутри DKP: хранение метаданных в PostgreSQL, S3-compatible artifact storage,
UI/API вход, Deckhouse ingress/https, Dex SSO и воспроизводимую сборку образов.

Текущий порядок разработки:
1. сначала рабочий внутренний managed backend внутри модуля;
2. затем `Model` / `ClusterModel` и контроллер публикации;
3. затем hardening, patching и long-term support.

Что уже входит в репозиторий:
- метаданные DKP-модуля и user-facing документация;
- стабильные `config-values` для логирования ai-models, PostgreSQL и S3-compatible artifacts;
- runtime templates для внутреннего backend, `Ingress`, `DexAuthenticator` и managed-postgres wiring, при этом общая runtime-обвязка берётся из platform/global settings;
- runtime/internal `values` и image-based Go hooks для модульной обвязки;
- `werf`, CI/CD и repo-local workflow для упаковки внутреннего backend engine.

Начинать с:
- `AGENTS.md`
- `DEVELOPMENT.md`
- `docs/development/TZ.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `plans/active/`
