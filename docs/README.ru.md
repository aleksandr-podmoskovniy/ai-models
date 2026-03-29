---
title: "Обзор"
menuTitle: "Обзор"
weight: 10
---

`ai-models` — модуль DKP для реестра и каталога AI/ML-моделей.
На текущем этапе модуль использует собственный managed внутренний registry backend.

Сейчас в репозитории уже есть:

- модульные метаданные DKP и phase-1 runtime templates;
- короткий стабильный user-facing контракт конфигурации для логирования,
  PostgreSQL и S3-compatible artifacts; общие runtime-настройки берутся
  из platform и global defaults Deckhouse;
- wiring для Dex SSO, ingress/https и managed-postgres на базе runtime
  возможностей Deckhouse;
- `werf` и CI/CD для сборки и поставки модуля;
- repo-local guidance и skills для следующих шагов по упаковке backend engine
  и разработке DKP API.
