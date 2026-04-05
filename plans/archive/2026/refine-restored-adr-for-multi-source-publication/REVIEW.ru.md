# REVIEW

## Scope check

Изменение осталось в пределах bundle:

- правился только восстановленный ADR;
- исходная идеология каталога как платформенной сущности `Model` /
  `ClusterModel` сохранена;
- документ не переделан заново в backend-first или inference-first архитектуру.

## Что получилось

- В ADR аккуратно добавлен multi-source ingestion:
  - `HTTP`;
  - `HuggingFace`;
  - `Upload`.
- Зафиксирован controller-owned upload path по смыслу, близкий к
  virtualization images.
- Добавлена observed ссылка на опубликованный артефакт в `status.artifact`,
  включая backend-neutral `kind` и `uri`.
- Runtime consumption теперь описан через local materialization в PVC/shared
  volume, а не через прямую работу runtime с raw backend storage.
- Сохранена старая опора ADR:
  - `status.resolved` как технический профиль;
  - `usagePolicy` / `launchPolicy` как platform constraints;
  - внутренний backend не стал частью публичной идентичности объекта.

## Проверки

- `git -C /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs diff --check -- 2026-03-18-ai-models-catalog.md`

## Residual risks

- Это всё ещё docs-only шаг: текущий код `api/` и controller libraries в
  `ai-models` остаются не до конца выровненными с обновлённым ADR.
- ADR теперь честно описывает separate source и published artifact, но точный
  shape будущих CRD-полей ещё надо закрепить в следующем rebaseline slice.
- `status.upload` и `status.artifact` описаны намеренно на уровне контракта, без
  жёсткой фиксации всех вложенных полей и transport details.
- Секция runtime materialization намеренно задаёт только platform semantic и не
  фиксирует окончательно, будет это init-container, sidecar или отдельный agent.
