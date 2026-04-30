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

## Нужен ли sds-node-configurator для работы модуля?

Для публикации `Model` / `ClusterModel` SDS не нужен. Для workload delivery
через целевой SharedDirect CSI contract нужен managed node-cache, поэтому
нужны `sds-node-configurator` и `sds-local-volume` либо другой явно
поддержанный substrate для node-local cache.

## Какие режимы доставки модели поддерживаются?

Текущий рабочий режим — `SharedDirect`:

- `nodeCache.enabled=true`;
- на выбранной ноде есть local SDS storage;
- node-cache runtime заранее materialize'ит нужные digest'ы в node-local cache;
- workload получает read-only CSI mount
  `/data/modelcache/models/<model-name>`.

Целевой универсальный режим без local disks — `SharedPVC`:

- `nodeCache.enabled=false`;
- задан RWX `StorageClass`, например CephFS/NFS/аналогичный shared filesystem;
- модуль создаёт controller-owned RWX PVC под конкретный workload/service и
  набор моделей;
- все Pod'ы этого workload'а читают модели из одного сетевого shared volume.

`SharedPVC` — отдельный controller-owned режим с собственным ownership/auth/GC
path, а не автоматическая загрузка через Secret в namespace workload'а. Если
нет ни готового `SharedDirect`, ни настроенного безопасного `SharedPVC`, модуль
fail-closed: workload получает понятный blocking reason, а не пустую директорию
и не скрытую загрузку.

Текущий безопасный SharedPVC foundation уже создаёт controller-owned RWX PVC и
держит workload на scheduling gate до готовности claim/materialization. Полный
запуск workload через SharedPVC требует digest-scoped materializer grant path;
общий DMCR read Secret в namespace workload'а не используется и не будет
использоваться.

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
тикеты, а для загрузки из кластера используйте `status.upload.inCluster`.

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

В Git храните только исходный объект и ai-models model annotation. Не
коммитьте CSI volumes, artifact attributes, env vars или mounts, которые
дописывает контроллер. Для CRD-операторов рендерите поддержанный Kubernetes
workload с model annotation на metadata workload'а; ai-models не патчит
higher-level CRD по имени.

## Почему workload не стартует, если модель не готова?

Delivery controller должен fail-closed: если `Model` не `Ready`, artifact не
опубликован, node-cache delivery выключен или набор моделей не помещается в
configured per-node cache size, workload получает condition/blocking reason
вместо запуска с пустым каталогом модели. Если workload явно запланирован на
ноду, где node-cache runtime не готов, kubelet покажет CSI mount failure/wait;
ai-models не inject'ит node placement.

## Какие метрики смотреть первыми?

- `d8_ai_models_model_ready`;
- `d8_ai_models_model_status_phase`;
- `d8_ai_models_model_condition`;
- `d8_ai_models_model_artifact_size_bytes`;
- `d8_ai_models_storage_backend_limit_bytes`;
- `d8_ai_models_node_cache_runtime_pods_ready`;
- `d8_ai_models_workload_delivery_workloads_managed`;
- `d8_ai_models_workload_delivery_pods_managed`;
- `d8_ai_models_workload_delivery_pods_ready`;
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
SharedDirect CSI mount, стабильным runtime environment, retries и будущими
изменениями delivery topology без выдачи registry credentials в namespace
workload.
