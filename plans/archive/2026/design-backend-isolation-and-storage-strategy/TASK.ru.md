# TASK

## Контекст
После поднятия phase-1 backend стало видно несколько стратегических вопросов:
- как не отдавать всем пользователям один общий raw MLflow surface;
- как совместить SSO с native MLflow auth/workspaces, не скатившись в ingress-only полумеру;
- как сделать импорт больших HF-моделей эффективнее и ближе к будущему controller flow;
- как соотнести это с возможностями KServe как потребителя/хранилища моделей.

## Постановка задачи
Проверить текущий backend/auth/storage contract против upstream MLflow, Hugging Face, KServe и Deckhouse reference modules и сформировать корректную стратегию для:
- SSO в сам backend через Dex без ложной уверенности в ingress-only auth;
- backend isolation через native MLflow workspaces и app-level authz;
- sync `namespace/group -> workspace` без неявного scraping чужих RBAC-сущностей;
- efficient HF -> S3 import path;
- model storage / consumption boundaries around KServe.

## Scope
- repo-local analysis текущего ai-models runtime;
- upstream-first research по MLflow workspaces/authz/SSO, HF import flows и KServe storage surface;
- сверка с Deckhouse reference modules, где UI закрывается через `user-authn`/`DexAuthenticator`;
- decisions и next steps для phase 1 / phase 2.

## Non-goals
- не внедрять в этом slice новый CRD API;
- не добавлять кастомный auth слой поверх upstream MLflow без отдельной implementation task;
- не реализовывать сразу end-to-end controller.

## Затрагиваемые области
- `plans/active/design-backend-isolation-and-storage-strategy/*`
- при необходимости repo docs, если по итогам потребуется зафиксировать решения отдельным slice.

## Критерии приёмки
- дан честный ответ, как должна выглядеть целевая схема `SSO + native MLflow workspaces + namespace/group sync`;
- дан честный ответ, можно ли и нужно ли включать workspaces для backend isolation уже сейчас;
- описан корректный import path для больших HF-моделей и границы дальнейшего улучшения;
- дан короткий и точный ответ по model storage surface в KServe;
- решения разложены на phase-1 feasible / phase-2 target / non-goals.

## Риски
- легко спутать raw upstream UI surface с тем, что реально должно стать platform contract;
- легко перепутать cluster-level SSO gate с app-native authz model;
- легко предложить более быстрый import path ценой невалидного MLflow artifact layout или потери reuse для будущего controller.
