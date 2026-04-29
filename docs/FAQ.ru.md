---
title: "FAQ"
menuTitle: "FAQ"
weight: 80
description: "Частые вопросы по ai-models, node-cache, upload, Ollama, ArgoCD и диагностике."
---

## Почему в ModuleConfig нет настроек DMCR?

`DMCR` — internal publication backend, а не пользовательский registry.
Публичный контракт модуля — `Model`, `ClusterModel`, `status.artifact` и
workload annotations. Если вывести наружу DMCR prefixes, TLS, auth или GC
schedule, клиенты начнут зависеть от деталей реализации, которые модуль должен
иметь возможность менять.

## Почему нельзя выбрать sourceFetchMode?

Транспорт скачивания — responsibility controller/runtime, а не пользователя.
Пользователь задаёт источник модели, а модуль выбирает безопасный путь:
streaming, temporary object-source или upload staging. Это уменьшает число
крутилок и исключает ситуации, когда одинаковые модели публикуются разными
неподдерживаемыми путями.

## Нужен ли sds-node-configurator для работы модуля?

Нет. Базовая публикация `Model` / `ClusterModel` и fallback delivery работают
без SDS. `sds-node-configurator` и `sds-local-volume` нужны только для managed
node-cache и SharedDirect delivery.

## Будет ли смешанный режим: где есть локальный диск — cache, где нет — PVC?

Целевая модель такая: ноды с локальным кэшем явно выбираются selector'ом и
получают node-cache runtime; workload с SharedDirect должен попадать только на
ноды, где кэш готов и модель помещается. Ноды без подходящего диска не надо
помечать `ai.deckhouse.io/model-cache=true`. Если нужен гарантированный
fallback через PVC/materialize, держите его отдельным delivery режимом и не
смешивайте с SharedDirect для одного и того же rollout без явного решения.

## sds-node-configurator не видит диск. Что проверить?

Проверьте, что диск:

- действительно добавлен в VM/ноду и виден в ОС;
- не содержит старых LVM/FS signatures;
- не используется kubelet, Ceph, systemd или другим storage stack;
- появился как `BlockDevice` и имеет состояние consumable;
- промаркирован тем же label, что указан в `nodeCache.blockDeviceSelector`.

Команды:

```bash
kubectl get blockdevices.storage.deckhouse.io -o wide
kubectl describe blockdevice <bd-name>
kubectl get nodes --show-labels | grep ai.deckhouse.io/model-cache
```

## Почему upload URL является секретом?

Upload URL содержит временный credential, как direct upload URL в
virtualization. Считайте его секретом: не вставляйте URL в публичные логи или
тикеты, а для загрузки из кластера используйте `status.upload.inClusterURL`.

## Что означает ToolCalling?

`ToolCalling` означает, что metadata модели содержит признаки tool-call
совместимого chat template. Это не включает MCP само по себе. MCP — способность
runtime/host слоя будущего `ai-inference`.

## Как работает publication из Ollama?

Контроллер читает Ollama registry manifest/config/blob path, а не public
HTML-страницу и не локальный Ollama daemon. Он принимает один GGUF model layer,
проверяет descriptor digest и GGUF magic header, затем публикует payload как
module-owned `ModelPack` artifact. Выбор runtime остаётся решением будущего
`ai-inference`.

## Как избежать ArgoCD drift из-за workload mutation?

В Git храните только исходный объект и annotation на Pod template. Не
коммитьте module-injected volumes, env, initContainers или generated object
patches. Для CRD-операторов вроде KubeRay аннотируйте template, из которого
оператор создаёт Pods. Live mutation generated Pods — ответственность
ai-models controller, а не desired state в Git.

## Почему workload не стартует, если модель не готова?

Delivery controller должен fail-closed: если `Model` не `Ready`, artifact не
опубликован или node-cache не готов, workload получает condition/blocking
reason вместо запуска с пустым каталогом модели. Это безопаснее, чем тихо
запустить inference runtime без модели.

## Какие метрики смотреть первыми?

- `d8_ai_models_model_ready`;
- `d8_ai_models_model_status_phase`;
- `d8_ai_models_model_condition`;
- `d8_ai_models_model_artifact_size_bytes`;
- `d8_ai_models_storage_backend_limit_bytes`;
- `d8_ai_models_node_cache_runtime_pods_ready`;
- `d8_ai_models_workload_delivery_workloads_managed`;
- `d8_ai_models_workload_delivery_init_state`;
- `d8_ai_models_dmcr_gc_requests`.

## Что делать, если публикация упала по InsufficientStorage?

Проверьте `artifacts.capacityLimit`, текущие storage usage метрики и размер
модели. Для upload-сессии используйте `curl -T <file> <upload-url>` или другой
клиент, который отправляет `Content-Length`; без известного размера модуль
отклоняет запрос до записи данных, потому что не может безопасно
зарезервировать место.

## Можно ли вручную использовать status.artifact.uri?

Для диагностики — да. Для прикладных workload — нет. Workload должен ссылаться
на `Model` / `ClusterModel` аннотацией, чтобы контроллер мог управлять
credential projection, mount path, cache mode, retries и будущими изменениями
delivery topology.
