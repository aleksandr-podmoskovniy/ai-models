---
title: "Обзор"
menuTitle: "Обзор"
weight: 10
---

`ai-models` — модуль DKP для каталога AI/ML-моделей и publication/runtime
delivery path.
На текущем этапе platform-facing baseline строится вокруг `Model` /
`ClusterModel`, controller-owned publication и runtime delivery. Историческая
backend-попытка больше не входит в live baseline репозитория.

Сейчас в репозитории уже есть:

- модульные метаданные DKP и runtime templates для module-owned publication
  surfaces;
- короткий стабильный user-facing контракт конфигурации для логирования и
  S3-compatible artifacts; общие runtime-настройки берутся из platform и
  global defaults Deckhouse;
- phase-2 `Model` / `ClusterModel` API и controller path для source-first
  publication в OCI-backed `ModelPack` artifacts; standalone runtime
  materializer и reusable consumer-side K8s wiring для `OCI -> local path`
  уже есть, но concrete runtime integration по-прежнему остаётся отдельным
  workstream;
- `werf` и CI/CD для сборки и поставки модуля;
- repo-local guidance и skills для следующих шагов по publication,
  distribution и разработке DKP API.
