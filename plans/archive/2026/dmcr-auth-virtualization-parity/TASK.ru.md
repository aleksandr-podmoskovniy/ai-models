# Выровнять DMCR auth по virtualization DVCR pattern

## Контекст

После перевода TLS на hook-owned internal values следующий stateful Helm drift
остался в DMCR auth: `templates/_helpers.tpl` генерирует password/salt через
`randAlphaNum`, читает live Secrets через `lookup`, вызывает Helm `htpasswd` и
собирает runtime auth state прямо во время render.

В `virtualization` DVCR auth state создаётся hook-ом: hook читает существующий
Secret, валидирует password/salt/htpasswd, генерирует недостающие значения и
пишет их в internal values. Templates только проецируют готовый state.

## Постановка задачи

Перенести DMCR write/read passwords, htpasswd entries и salt из Helm render
logic в hook-owned `aiModels.internal.dmcr.auth`, сохранив текущий DMCR auth
contract для runtime Secret и dockerconfig Secrets.

## Scope

- Добавить hook для DMCR auth state.
- Добавить schema/fixtures для `aiModels.internal.dmcr.auth`.
- Перевести `templates/dmcr/secret.yaml` на values-backed auth state.
- Удалить Helm helpers, которые делают `lookup`, `randAlphaNum` и `htpasswd`
  для DMCR auth.
- Обновить render validation, чтобы DMCR auth stateful render path не вернулся.

## Non-goals

- Не менять DMCR usernames.
- Не менять Secret names, mount paths, ports или registry protocol.
- Не менять TLS/rootCA в этом slice.
- Не менять public `Model` / `ClusterModel` API и user-facing RBAC.

## Критерии приёмки

- В DMCR auth path нет Helm `randAlphaNum` и Helm `htpasswd`.
- `templates/dmcr/secret.yaml` читает `aiModels.internal.dmcr.auth`.
- Existing Secret migration покрыта hook-ом: если Secret есть, значения
  сохраняются; если повреждён/пустой, генерируются новые.
- Render validation проверяет, что DMCR auth helpers не вернули stateful render.
- `make helm-template`, `make kubeconform`, hooks tests и `make verify` проходят.
