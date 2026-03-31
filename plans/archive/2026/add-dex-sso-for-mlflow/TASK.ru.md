# TASK

## 1. Заголовок

Добавить вход в MLflow по SSO через Deckhouse Dex без отказа от upstream-native workspaces и direct-to-S3 import path.

## 2. Контекст

`ai-models` уже переведён на upstream-native MLflow auth/workspaces и direct
artifact access, но текущий login flow остался только на bootstrap admin
credentials. Для phase-1 backend это уже даёт internal auth boundary, но UX
плохой: пользователю приходится логиниться вручную локальным MLflow admin, а
не через platform SSO.

Пользователь явно требует:
- заходить в MLflow по SSO;
- делать это корректно и ближе к уже существующим DKP-паттернам, а не через
  ad-hoc workaround.

## 3. Постановка задачи

Нужно найти и реализовать такой SSO path для MLflow, который:
- использует Deckhouse Dex как OIDC provider;
- остаётся максимально близким к upstream MLflow security model;
- не ломает workspaces и machine-oriented import Jobs;
- не вводит непонятную пользователю ручную bootstrap-процедуру после включения
  модуля.

## 4. Scope

В задачу входит:
- анализ upstream MLflow SSO contract и DKP reference-паттернов;
- выбор корректного phase-1 решения для SSO через Dex;
- изменения values/templates/runtime/docs/fixtures/validation под выбранный
  путь;
- проверка, что login path и existing direct import path совместимы.

## 5. Non-goals

- Не проектировать ещё namespace/workspace sync controller.
- Не вводить phase-2 `Model` / `ClusterModel`.
- Не менять direct-to-S3 import path обратно на proxied uploads.
- Не строить отдельный самописный auth service вне оправданного DKP pattern.

## 6. Затрагиваемые области

- `openapi/`
- `templates/backend/`
- `templates/auth/`
- `templates/module/`
- `images/backend/`
- `tools/`
- `fixtures/`
- `docs/`
- `plans/active/add-dex-sso-for-mlflow/`

## 7. Критерии приёмки

- UI login flow для MLflow идёт через SSO, а не через ручной bootstrap password;
- выбранный путь объясним относительно upstream MLflow и DKP references;
- import Jobs и machine-to-machine path не ломаются;
- values/OpenAPI/templates/docs/fixtures согласованы;
- `make verify` проходит.

## 8. Риски

- Upstream MLflow SSO path может потребовать сторонний plugin или bootstrap
  sequence, который сложнее, чем базовый auth app.
- Совмещение SSO и machine-oriented internal auth может легко развалить import
  Jobs и monitoring.
- Неправильный выбор между ingress SSO и app-native OIDC может дать красивый
  UI login, но плохую backend authz story.
