# REVIEW

## Findings

Критичных блокеров по текущему docs slice не найдено.

ADR в [2026-03-18-ai-models-catalog.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md)
переписан под current phase-2 design:

- старый OCI-only и inference-centric contract убран;
- public API теперь описан через `Model` / `ClusterModel`,
  `spec.source={HuggingFace|Upload|OCIArtifact}` и backend-neutral
  `status.artifact`;
- upload описан как controller-owned handoff со staging/promote semantics без
  прямого протаскивания virtualization/DVCR-specific деталей;
- managed backend оставлен внутренним компонентом;
- runtime delivery описан как local materialization concern, а не как public
  serving API;
- delete cleanup добавлен как controller-owned lifecycle.

## Scope check

- Документ больше не обещает `modelType`, `usagePolicy`, `launchPolicy`,
  `status.resolved.*` и другие поля, которых нет в текущем CRD.
- Документ больше не требует OCI-only `spec.artifact` как единственную форму
  public contract.
- Архитектурное описание осталось достаточно high-level и не скатилось в
  implementation dump про staging RBAC, secret copies или pod wiring.

## Checks

Пройдены:

- manual read-through against current design bundle
- manual read-through against current public API/types
- `git -C /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs diff --check`

## Residual risks

- В самом `ai-models` design bundle ещё остались старые формулировки вокруг
  `status.artifact.ociRef` и более OCI-centric narrative. ADR уже обновлён, но
  bundle docs в repo ещё не полностью нормализованы.
- ADR теперь синхронизирован с current contract лучше, чем часть старых design
  notes. Следующий docs slice должен либо обновить bundle целиком, либо явно
  разделить historical decisions и current source of truth.

## Next step

- Нормализовать оставшиеся design docs в `plans/active/design-model-catalog-controller-and-publish-architecture/*`
  под backend-neutral `status.artifact` и local-materialization runtime story.
