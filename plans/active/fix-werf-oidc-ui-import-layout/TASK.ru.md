# Fix Werf OIDC UI Import Layout

## 1. Контекст

Backend image собирает patched `mlflow-oidc-auth` из source и требует prebuilt UI assets при `install-oidc-auth-from-source.sh`. В `Dockerfile.local` этот путь уже есть, но в `werf` final image импорт `/oidc-auth-ui` оказался пропущен, из-за чего CI падает на этапе `backend/install` с сообщением `The provided OIDC auth UI directory does not exist: /oidc-auth-ui`.

## 2. Постановка задачи

Выровнять `werf` final image layout с локальным Docker layout и добавить проверку, которая заранее ловит отсутствие `backend-oidc-auth-ui-build -> /oidc-auth-ui` import в финальном `backend` image.

## 3. Scope

- исправить `images/backend/werf.inc.yaml`;
- усилить локальный verify/guard для этого layout contract;
- прогнать релевантные проверки.

## 4. Non-goals

- не менять patch queue `mlflow-oidc-auth`;
- не менять runtime/auth semantics;
- не менять `Dockerfile.local`, если он уже согласован с нужным layout.

## 5. Затрагиваемые области

- `images/backend/werf.inc.yaml`
- `Makefile`

## 6. Критерии приёмки

- final `backend` image в `werf.inc.yaml` импортирует prebuilt OIDC UI assets;
- локальная verify-проверка ловит отсутствие этого import path;
- `make verify` проходит.

## 7. Риски

- можно починить только текущий symptom и снова оставить `Dockerfile.local` и `werf` в расхождении;
- если guard будет слишком хрупким к форматированию YAML, он даст ложные падения.
