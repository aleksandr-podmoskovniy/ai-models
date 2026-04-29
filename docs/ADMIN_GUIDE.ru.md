---
title: "Руководство администратора"
menuTitle: "Администрирование"
weight: 40
description: "Включение ai-models, artifact storage, managed node-cache поверх SDS, RBAC, monitoring и эксплуатация."
---

Это руководство предназначено для администратора DKP-кластера, который включает
модуль `ai-models`, подключает объектное хранилище, настраивает локальный кэш
моделей и проверяет эксплуатационную готовность.

## Ключевая идея

- Пользовательский `ModuleConfig` должен быть коротким.
- `DMCR`, имена внутренних объектов, source-fetch policy и GC cadence
  управляются модулем.
- Администратор задаёт только стабильные решения: artifact storage,
  capacity limit и, если нужен node-local cache, selector/size для cache
  nodes и `BlockDevice`.

## Требования

- Deckhouse Kubernetes Platform `>= 1.74`.
- Kubernetes `>= 1.30`.
- S3-compatible object storage с bucket для ai-models.
- Secret в `d8-system` с ключами `accessKey` и `secretKey`.
- Для managed node-cache: включены модули `sds-node-configurator` и
  `sds-local-volume`, а на выбранных нодах есть consumable `BlockDevice`.

## Минимальное включение

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ai-models
spec:
  enabled: true
  version: 1
  settings:
    logLevel: Info
    artifacts:
      bucket: ai-models
      endpoint: https://s3.example.com
      region: us-east-1
      credentialsSecretName: ai-models-artifacts
      usePathStyle: true
```

Secret с credentials должен быть в `d8-system`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-artifacts
  namespace: d8-system
type: Opaque
stringData:
  accessKey: "<access-key>"
  secretKey: "<secret-key>"
```

Если S3 endpoint использует custom CA, добавьте `ca.crt` в отдельный Secret и
укажите `artifacts.caSecretName`, либо положите `ca.crt` в тот же Secret с
credentials.

## Capacity Limit

`artifacts.capacityLimit` задаёт общий бюджет module-owned artifact storage.
При включённом limit upload-сессии должны знать размер payload до записи
данных. Обычный путь `curl -T` передаёт размер через `Content-Length`;
multipart-клиенты передают его на `/probe`. Модуль резервирует место до
загрузки и возвращает понятную ошибку при нехватке capacity.

```yaml
spec:
  settings:
    artifacts:
      capacityLimit: 500Gi
```

Capacity limit не заменяет quota object storage backend. Это admission и
accounting слой ai-models, который защищает модуль от заведомо невозможных
публикаций.

## Managed Node Cache

Managed node-cache нужен, чтобы workload не скачивал модель в свой PVC каждый
раз, а получал read-only mount из per-node shared digest store.

Базовая схема:

1. Включите `sds-node-configurator` и `sds-local-volume`.
2. Пометьте ноды, где можно держать кэш:

   ```bash
   kubectl label node <node> ai.deckhouse.io/model-cache=true
   ```

3. Пометьте `BlockDevice`, который можно отдать под кэш:

   ```bash
   kubectl label blockdevice <bd-name> ai.deckhouse.io/model-cache=true
   ```

4. Включите node-cache:

   ```yaml
   spec:
     settings:
       nodeCache:
         enabled: true
         size: 200Gi
   ```

По умолчанию ai-models выбирает `Node` и `BlockDevice` по одному label
`ai.deckhouse.io/model-cache=true`. Переопределяйте `nodeSelector` и
`blockDeviceSelector` только если в кластере уже есть другая строгая схема
лейблинга.

### Проверка SDS и cache substrate

```bash
kubectl get blockdevices.storage.deckhouse.io -o wide
kubectl get lvmvolumegroupsets.storage.deckhouse.io
kubectl get lvmvolumegroups.storage.deckhouse.io
kubectl get localstorageclasses.storage.deckhouse.io
kubectl -n d8-ai-models get pods,pvc -l app=ai-models-node-cache-runtime -o wide
```

Если `sds-node-configurator` не видит диск, проверьте, что диск действительно
новый/свободный, не содержит старых сигнатур LVM/FS и попал в
`BlockDevice` как `consumable=true`.

## RBAC

Модуль следует Deckhouse access-level model:

- `User` видит `Model` / `ClusterModel` и статусы.
- `Editor` управляет namespaced `Model`.
- `ClusterEditor` управляет `ClusterModel`.
- `rbacv2/use` даёт namespaced использование `Model`.
- `rbacv2/manage` даёт cluster-persona управление `Model`, `ClusterModel` и
  `ModuleConfig`.

Human-facing роли не должны давать `Secret`, `pods/exec`, `pods/attach`,
`pods/portforward`, `status`, `finalizers` и internal runtime objects.

## Monitoring

Проверьте основные метрики:

```bash
kubectl -n d8-ai-models get podmonitor,prometheusrule
```

Ключевые группы метрик:

- catalog state: phase/ready/conditions/artifact size для `Model` и
  `ClusterModel`;
- storage usage: backend limit/used/reserved;
- runtime health: node-cache runtime Pods/PVC, workload delivery mode/reason;
- DMCR GC lifecycle: queued/armed/done request phases.

## Эксплуатация и диагностика

Проверить компоненты:

```bash
kubectl -n d8-ai-models get pods -o wide
kubectl get models.ai.deckhouse.io -A
kubectl get clustermodels.ai.deckhouse.io
```

Посмотреть состояние модели:

```bash
kubectl -n <ns> describe model <name>
kubectl get clustermodel <name> -o yaml
```

Проверить публикацию:

- `status.phase`;
- `status.conditions`;
- `status.artifact.uri`;
- `status.resolved.supportedEndpointTypes`;
- `status.resolved.supportedFeatures`.

## Что не настраивается снаружи

- `DMCR` storage prefixes, auth, TLS и GC schedule;
- source fetch mode (`Direct` / mirror) как public toggle;
- names of `LocalStorageClass`, `LVMVolumeGroupSet`, VG and thin pool;
- publication worker resources, если это не вынесено отдельным future slice.
