# TASK

## Контекст
После поднятия phase-1 backend стало видно три стратегических вопроса:
- как не отдавать всем пользователям один общий raw MLflow surface;
- как сделать импорт больших HF-моделей эффективнее и ближе к будущему controller flow;
- как соотнести это с возможностями KServe как потребителя/хранилища моделей.

## Постановка задачи
Проверить текущий backend/auth/storage contract против upstream MLflow, Hugging Face и KServe и сформировать корректную стратегию для:
- backend isolation через workspace/authz;
- efficient HF -> S3 import path;
- model storage / consumption boundaries around KServe.

## Scope
- repo-local analysis текущего ai-models runtime;
- upstream-first research по MLflow workspaces/authz, HF import flows и KServe storage surface;
- decisions и next steps для phase 1 / phase 2.

## Non-goals
- не внедрять в этом slice новый CRD API;
- не добавлять кастомный auth слой поверх upstream MLflow без отдельной implementation task;
- не реализовывать сразу end-to-end controller.

## Затрагиваемые области
- `plans/active/design-backend-isolation-and-storage-strategy/*`
- при необходимости repo docs, если по итогам потребуется зафиксировать решения отдельным slice.

## Критерии приёмки
- дан честный ответ, можно ли и нужно ли включать workspaces для backend isolation уже сейчас;
- описан корректный import path для больших HF-моделей и границы дальнейшего улучшения;
- дан короткий и точный ответ по model storage surface в KServe;
- решения разложены на phase-1 feasible / phase-2 target / non-goals.

## Риски
- легко спутать raw upstream UI surface с тем, что реально должно стать platform contract;
- легко предложить более быстрый import path ценой невалидного MLflow artifact layout или потери reuse для будущего controller.
