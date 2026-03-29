# План работ: отдельный ARC runner для ai-models

## Slice 1. Поднять отдельный repo-level runner pool

### Цель

Создать отдельный ARC runner scale set и cache PVC для `ai-models`, не затрагивая другие репозитории.

### Изменяемые области

- Kubernetes namespace `arc-runners`
- Helm release `gha-runner-scale-set`

### Решение

- отдельный release: `ai-models-runners`
- `githubConfigUrl`: `https://github.com/aleksandr-podmoskovniy/ai-models`
- `minRunners=0`
- `maxRunners=2`
- отдельный `RWX` PVC умеренного размера для cache

### Проверки

- `kubectl get autoscalingrunnersets.actions.github.com -n arc-runners`
- `kubectl get autoscalinglisteners.actions.github.com -n arc-system`
- listener работает без restart loop

### Артефакт

В кластере есть отдельный repo-level runner pool для `ai-models`.

## Slice 2. Довести workflow до self-hosted compatibility

### Цель

Убедиться, что `build.yaml` и `deploy.yaml` не полагаются на GitHub-hosted environment assumptions.

### Изменяемые области

- `.github/workflows/build.yaml`
- `.github/workflows/deploy.yaml`

### Решение

- оставить label `ai-models-runners`
- при необходимости явно готовить cache dirs
- установить минимальный host toolchain там, где self-hosted runner его не гарантирует

### Проверки

- локальная YAML validation workflow files
- review `Makefile` targets, которые реально вызываются в CI

### Артефакт

Workflow совместимы с ARC+dind runner и не требуют GitHub-hosted runner image.

## Rollback point

После Slice 1. Если workflow-часть окажется лишней, cluster-side runner pool можно оставить отдельно, не трогая repo.

## Final validation

- `kubectl get autoscalingrunnersets.actions.github.com -n arc-runners`
- `kubectl get ephemeralrunnersets.actions.github.com -n arc-runners`
- `kubectl get pods -n arc-system`
- локальная YAML validation `.github/workflows/build.yaml` и `.github/workflows/deploy.yaml`
