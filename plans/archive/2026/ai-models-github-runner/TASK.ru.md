# Настроить отдельный GitHub ARC runner для ai-models

## Контекст

В репозитории `ai-models` workflow уже ориентированы на label `ai-models-runners`, но в кластере нет отдельного ARC runner scale set для этого репозитория. Использовать существующий runner pool другого проекта не нужно: scope должен оставаться repo-level и не влиять на `gpu-control-plane` и `dkp-ai-strategy`.

## Постановка задачи

Нужно поднять в том же кластере отдельный GitHub Actions runner для репозитория `ai-models`, используя ARC и тот же GitHub auth secret. Runner должен быть достаточно скромным по ресурсам, поднимать worker pods только по запросу и не держать лишние idle runners.

## Scope

- создать отдельный ARC runner scale set для repo `ai-models`;
- создать отдельный cache PVC для runner pool;
- убедиться, что workflow реально совместимы с self-hosted runner;
- при необходимости минимально поправить `.github/workflows/*`, не меняя CI semantics;
- задокументировать план и результат в task bundle.

## Non-goals

- не переводить runner на organization-level scope;
- не трогать рабочие runner pools других репозиториев;
- не менять бизнес-логику CI beyond compatibility fixes for self-hosted runner;
- не перерабатывать build/deploy pipeline целиком.

## Затрагиваемые области

- Kubernetes namespace `arc-runners`
- GitHub Actions ARC resources
- `.github/workflows/build.yaml`
- `.github/workflows/deploy.yaml`
- `plans/ai-models-github-runner/*`

## Критерии приёмки

- в кластере есть отдельный repo-level scale set `ai-models-runners`;
- scale set смотрит на `https://github.com/aleksandr-podmoskovniy/ai-models`;
- есть отдельный cache PVC для этого runner pool;
- listener стабильно работает без restart loop;
- workflow используют label `ai-models-runners` и не падают на базовых зависимостях self-hosted runner;
- существующие runner pools других репозиториев не затронуты.

## Риски

- self-hosted runner может не содержать базовые утилиты вроде `make`, что приводит к падению `verify`;
- слишком агрессивные размеры cache PVC или `minRunners` приведут к лишнему потреблению ресурсов;
- прямое вмешательство в общий runner pool может сломать CI других репозиториев, поэтому нужен отдельный scale set.
