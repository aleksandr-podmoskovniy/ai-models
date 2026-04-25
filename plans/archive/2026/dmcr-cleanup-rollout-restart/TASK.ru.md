## 1. Заголовок

DMCR rollout после удаления модели и garbage-collection trigger

## 2. Контекст

После удаления модели и запуска cleanup / DMCR garbage collection пользователь наблюдает замену pod-ов `dmcr-*`, в частности старую ревизию `dmcr-69d67887d9-bsnk6`.

Первые события кластера показывают не crash-loop текущих pod-ов, а rolling update Deployment `dmcr`: сменяются ReplicaSet-ы, текущая ревизия `dmcr-744549c448` здорова.

## 3. Постановка задачи

Нужно понять, почему cleanup/GC path приводит к rollout `dmcr`, и исправить это, если rollout вызван нестабильным render/checksum или побочным изменением runtime resources.

## 4. Scope

- live cluster inspection of `d8-ai-models` DMCR Deployment/ReplicaSet/events/logs;
- templates and hooks that render `dmcr` Deployment, Secret, ConfigMap and GC request resources;
- DMCR garbage-collection controller/cleaner code only if rollout caused by its writes.

## 5. Non-goals

- не менять public Model/ClusterModel API;
- не менять human-facing RBAC;
- не переписывать весь DMCR backend;
- не смешивать это с текущим workload admission slice.

## 6. Критерии приёмки

- Установлена точная причина rollout: какой PodTemplate field/checksum changed and why.
- Если причина в нестабильном template/render, исправлен render так, чтобы cleanup/GC не менял DMCR PodTemplate без реального config/secret change.
- Если причина операционная, зафиксирована команда/ресурс, который инициировал rollout.
- Текущий `dmcr` остаётся healthy.
- Релевантные проверки проходят: targeted helm render/kubeconform and focused tests if code changes.

## 7. Orchestration

`solo`: сначала live triage и narrow fix. Сабагенты не используются, потому что пользователь не просил их в этой задаче и первичный риск проверяется напрямую по cluster events/render diff.

## 8. Rollback point

Если правка затрагивает templates, rollback is to revert this bundle's DMCR template changes; live cluster can continue on current healthy `dmcr-744549c448` while root cause is isolated.
