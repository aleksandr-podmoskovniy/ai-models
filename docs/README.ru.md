---
title: "Обзор"
menuTitle: "Обзор"
weight: 10
---

`ai-models` — модуль DKP для каталога AI/ML-моделей и publication/runtime
delivery path.
На текущем этапе модуль держит hidden managed backend рядом с
platform-facing `ModelPack` catalog/runtime path и не выводит backend internals
в платформенный контракт.

Сейчас в репозитории уже есть:

- модульные метаданные DKP и phase-1 runtime templates;
- короткий стабильный user-facing контракт конфигурации для логирования,
  PostgreSQL и S3-compatible artifacts; общие runtime-настройки берутся
  из platform и global defaults Deckhouse;
- wiring для native MLflow auth/workspaces, ingress/https и managed-postgres;
- phase-2 `Model` / `ClusterModel` API и controller path для source-first
  publication в OCI-backed `ModelPack` artifacts и runtime materialization в
  локальный путь;
- `werf` и CI/CD для сборки и поставки модуля;
- repo-local guidance и skills для следующих шагов по упаковке backend engine
  и разработке DKP API.
