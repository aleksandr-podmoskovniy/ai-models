# Разобрать живой startup ai-models на кластере

## Контекст

Репозитарные проверки проходят, но на живом кластере `ai-models` продолжает
ломаться на startup. Пользователь просит смотреть не на локальный render, а на
реальные cluster logs и runtime state.

## Постановка задачи

Нужно снять и интерпретировать фактическое состояние модуля на кластере:
`ModuleConfig`, namespace `d8-ai-models`, события, ресурсы, логи Deckhouse и
ошибки startup sequence, затем при необходимости внести минимальный fix в модуль.

## Scope

- чтение live state из кластера через `kubectl`;
- анализ `ai-models` startup path;
- при необходимости правка repo и повторный repo-level verify.

## Non-goals

- не делать unrelated cleanup;
- не менять phase-2 API или controller design;
- не трогать внешние сервисы вне необходимости, если проблема в module shell.

## Затрагиваемые области

- cluster state через `/Users/myskat_90/.kube/k8s-config`
- при необходимости `templates/*`, `openapi/*`, docs и verify tooling
- `plans/active/debug-live-cluster-startup/*`

## Критерии приёмки

- собрана понятная картина текущего cluster failure;
- найден ближайший реальный blocker startup path;
- если blocker в модуле, внесён минимальный fix и `make verify` проходит;
- остаточные риски явно сформулированы.

## Риски

- на кластере могут одновременно существовать несколько разных blocker'ов;
- cluster state может отставать от текущего repo diff и это нужно отличать от
  проблем модульного контракта.
