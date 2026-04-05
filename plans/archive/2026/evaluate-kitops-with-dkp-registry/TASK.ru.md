# Оценка KitOps/ModelPack в связке с dkp-registry для ai-models v0

## Контекст

В текущем `ai-models` phase-1 внутренняя backend-линия строится вокруг `MLflow + S3`, но для serving и early-R&D это выглядит избыточно и создаёт плохой data-plane contract: внутренние `S3` пути, ручной `RGW` RBAC и отсутствие нормального stable URI для inference runtime.

Параллельно в соседнем проекте уже есть `dkp-registry` с двумя компонентами:
- `bundle-registry`
- `payload-registry`

Нужно понять, подходит ли связка `KitOps/ModelPack + dkp-registry` как более релевантный v0/v1 serving/distribution path для `ai-models`.

## Постановка задачи

Проверить по коду и docs:
- что именно умеют `bundle-registry` и `payload-registry`;
- какой из них вообще релевантен для `KitOps/ModelPack`;
- можно ли на `payload-registry` опереться как на OCI registry backend для `ModelKit`;
- какие сильные и слабые стороны это даёт по сравнению с текущим `MLflow + S3 direct` path для inference.

## Scope

- Изучить `dkp-registry/bundle-registry`.
- Изучить `dkp-registry/payload-registry`.
- Сверить `payload-registry` auth/RBAC model с потребностями `KitOps/KServe`.
- Сверить официальный `KitOps` contract для `MLflow`, `OCI registry` и `KServe`.
- Сформулировать рекомендацию для `ai-models` v0: где `MLflow` нужен, а где лучше идти через `KitOps`.

## Non-goals

- Не менять сейчас код `ai-models` или `dkp-registry`.
- Не проектировать полный phase-2 API `Model` / `ClusterModel`.
- Не внедрять прямо сейчас `KitOps` в кластер.
- Не решать здесь весь training lifecycle.

## Затрагиваемые области

- `plans/active/evaluate-kitops-with-dkp-registry/*`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/dkp-registry/bundle-registry/*`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/dkp-registry/payload-registry/*`
- официальные docs `modelpack.org` / `kitops.org`

## Критерии приёмки

- Есть чёткий вывод по `bundle-registry`: подходит или нет, и почему.
- Есть чёткий вывод по `payload-registry`: подходит или нет, и почему.
- Есть чёткая оценка, насколько `payload-registry` даёт более нормальный v0 serving contract, чем `MLflow + S3 direct`.
- Есть рекомендованная target-схема: где использовать `MLflow`, а где `KitOps`.
- Ответ сформулирован прикладным языком, без абстрактной MLOps-риторики.

## Риски

- `payload-registry` может быть OCI-совместим технически, но всё равно оказаться operationally заточенным под “images”, а не `ModelKit`.
- KitOps/KServe integration может оказаться удобной для `KServe`, но не дать такой же прямой path для `KubeRay`.
- Есть риск переоценить `ModelPack` как replacement `MLflow`, хотя на практике это packaging/distribution layer, а не experiment backend.
