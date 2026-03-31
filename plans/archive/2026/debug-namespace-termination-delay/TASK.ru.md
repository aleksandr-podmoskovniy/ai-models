# Разобрать задержку удаления namespace d8-ai-models

## Контекст

Пользователь выключил модуль `ai-models` и удалил предыдущую managed PostgreSQL
инсталляцию, после чего namespace `d8-ai-models` заметно висел в состоянии
`Terminating`. Нужно понять, что именно задерживало удаление, чтобы не
маскировать lifecycle-проблемы force-finalize'ом.

## Постановка задачи

Нужно снять и интерпретировать фактический delete/finalization path для
`d8-ai-models`: namespace conditions, остаточные namespaced ресурсы, события и
связанные module-owned объекты. Если причина находится в module contract или
его runtime shape, это нужно явно зафиксировать. Если причина только в штатном
cleanup Kubernetes/Deckhouse, это тоже нужно объяснить без домыслов.

## Scope

- чтение live state и событий кластера через `/Users/myskat_90/.kube/k8s-config`;
- анализ delete/finalization path для `d8-ai-models`;
- при необходимости обновление task bundle с выводами.

## Non-goals

- не менять код модуля без явной необходимости;
- не force-finalize namespace без отдельного решения пользователя;
- не смешивать эту задачу с текущим startup-debug backend.

## Затрагиваемые области

- cluster state через `/Users/myskat_90/.kube/k8s-config`
- `plans/active/debug-namespace-termination-delay/*`

## Критерии приёмки

- собрана понятная картина, что именно задерживало удаление `d8-ai-models`;
- отделены реальные blockers удаления от обычного штатного cleanup;
- остаточные риски и при необходимости next step сформулированы явно.

## Риски

- namespace уже мог успеть удалиться до начала диагностики;
- часть событий могла исчезнуть из retention окна;
- delete path может включать как module-owned ресурсы, так и штатные
  kube-controller-generated объекты.
