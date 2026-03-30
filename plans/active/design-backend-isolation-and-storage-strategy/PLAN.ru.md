# PLAN

## Current phase
Этап 1. Внутренний managed backend inside the module.

## Режим orchestration
`solo`

## Slice 1. Зафиксировать текущие repo boundaries
- проверить, как сейчас устроены auth ingress, Dex, backend runtime и import path;
- зафиксировать, что именно уже wired, а что нет.

## Slice 2. Сверить upstream MLflow / HF / KServe
- MLflow: workspaces, authz, lifecycle удаления, artifact cleanup;
- Hugging Face: efficient download / snapshot semantics;
- KServe: model storage backends и serving sources.

## Slice 3. Свести решения
- phase-1 feasible;
- phase-2 target;
- что не делать без отдельной task/implementation.

## Rollback point
- этот bundle analysis-only; rollback не нужен за пределами bundle.

## Final validation
- проверить связность выводов по repo-local и upstream источникам;
- если код не меняется, repo-level verify не обязателен.
