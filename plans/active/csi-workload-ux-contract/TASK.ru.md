# CSI workload UX contract

## Задача

Пересмотреть и реализовать целевой UX доставки моделей в workload через
node-cache CSI: пользователь должен объявлять модель, а не вручную собирать
внутренние `volumeAttributes`, registry URI, digest и runtime wiring.

Решение должно оставаться generic: `ai-models` не должен знать KubeRay,
RayService, Stronghold или другие third-party CRD как отдельные сущности.

## Scope

- Сравнить текущий контракт с паттернами Deckhouse, virtualization,
  Stronghold/Vault и CSI-подходом Kubernetes.
- Сформулировать один простой user-facing contract для supported workload
  templates без public CSI attributes escape hatch.
- Реализовать generic mutation path для поддерживаемых PodTemplate workloads.
- Сохранить controller-owned stamping внутренних CSI атрибутов.
- Обновить tests/docs/e2e runbook под новый контракт.

## Non-goals

- Не возвращать Ray/KubeRay-specific reconciliation.
- Не добавлять PVC/materialize fallback.
- Не добавлять namespace Secret fallback для runtime auth.
- Не проектировать полный ai-inference scheduler в этом slice.
- Не менять node-cache CSI protocol сверх needed workload UX contract.

## Acceptance criteria

- Для поддерживаемого workload достаточно аннотации
  `ai.deckhouse.io/model` / `ai.deckhouse.io/clustermodel` /
  `ai.deckhouse.io/model-refs`; controller сам добавляет CSI volume, mounts,
  env и internal volume attributes.
- Пользователь не обязан и не должен задавать CSI volume, artifact URI/digest
  или registry credentials.
- Для third-party controllers нет package-local special cases: они должны
  отдавать PodTemplate через supported Kubernetes workload path либо ждать
  отдельного доверенного delivery ticket/API.
- Controller не выставляет node labels/selectors/affinity за пользователя.
- При отсутствии node-cache/local storage нет silent fallback; failure виден
  как понятный condition/event.
- Tests покрывают single-model, multi-model, existing CSI volume, wrong volume
  source, no nodeSelector injection and cleanup.

## Validation

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`
- `python3 -m unittest tools/helm-tests/validate_renders_test.py`
- `git diff --check`
- `make verify`

## Rollback point

Вернуть предыдущий explicit-volume contract: пользователь сам объявляет
inline CSI volume, а controller только stamps internal attributes. Rollback не
должен возвращать Ray/KubeRay-specific code и PVC fallback.
