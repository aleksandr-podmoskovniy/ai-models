# Verify Bootstrap Hardening

## Контекст

CI упал не на содержательной валидации kubeconform, а на одноразовом сетевом bootstrap `curl` внутри `tools/kubeconform/kubeconform.sh`.

## Постановка задачи

Сделать verify bootstrap более устойчивым к кратковременным сетевым сбоям без изменения semantics самой kubeconform-проверки.

## Scope

- harden kubeconform binary bootstrap download;
- не менять сами rendered manifests или kubeconform strictness;
- локально прогнать `make kubeconform`, `make verify` и build path.

## Non-goals

- не переделывать весь verify pipeline;
- не менять backend/dmcr/controller runtime behavior.

## Критерии приёмки

- bootstrap kubeconform binary использует retry-aware download;
- локальный `make kubeconform` проходит;
- локальный `make verify` проходит;
- локальный werf build path проходит.
