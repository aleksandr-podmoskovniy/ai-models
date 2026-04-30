---
title: "Руководство пользователя"
menuTitle: "Использование"
weight: 30
description: "Создание Model и ClusterModel, публикация из Hugging Face или upload, статусы и подключение моделей в workload."
---

Это руководство предназначено для пользователя namespace или оператора
прикладного сервиса, которому нужно опубликовать модель и подключить её в
workload без ручной настройки registry, initContainer и внутренних Secret.

## Model или ClusterModel

| Ресурс | Scope | Когда использовать |
| --- | --- | --- |
| `Model` | namespace | Модель принадлежит команде или приложению в одном namespace. |
| `ClusterModel` | cluster | Модель курируется администратором и используется несколькими namespace. |

`Model` может ссылаться на namespace-local `authSecretRef` для приватного
Hugging Face репозитория. `ClusterModel` не поддерживает `authSecretRef`,
потому что cluster-scoped объект не должен ссылаться на namespaced Secret.

## Публикация из Hugging Face

```yaml
apiVersion: ai.deckhouse.io/v1alpha1
kind: Model
metadata:
  name: bge-m3
  namespace: ai-demo
spec:
  source:
    url: https://huggingface.co/BAAI/bge-m3
```

Для приватного репозитория создайте Secret в том же namespace. Поддерживаемые
ключи: `token`, `HF_TOKEN`, `HUGGING_FACE_HUB_TOKEN`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hf-private-token
  namespace: ai-demo
type: Opaque
stringData:
  token: hf_xxx
---
apiVersion: ai.deckhouse.io/v1alpha1
kind: Model
metadata:
  name: private-llm
  namespace: ai-demo
spec:
  source:
    url: https://huggingface.co/acme/private-llm
    authSecretRef:
      name: hf-private-token
```

Проверка:

```bash
kubectl -n ai-demo get model bge-m3
kubectl -n ai-demo describe model bge-m3
kubectl -n ai-demo get model bge-m3 -o jsonpath='{.status.artifact.uri}{"\n"}'
```

## Upload-модель

Upload используется, когда модель уже есть у пользователя как файл или архив.

```yaml
apiVersion: ai.deckhouse.io/v1alpha1
kind: Model
metadata:
  name: uploaded-model
  namespace: ai-demo
spec:
  source:
    upload: {}
```

После создания ресурс переходит в `WaitForUpload`, а в status появляются URL:

```bash
kubectl -n ai-demo get model uploaded-model \
  -o jsonpath='{.status.upload.external}{"\n"}'
```

URL сам является временным секретом для загрузки, как upload URL в
virtualization. Не публикуйте его в логах и тикетах. Срок действия указан в
`status.upload.expiresAt`. Если загрузка идёт изнутри кластера, используйте
`status.upload.inCluster`.

Загрузите файл напрямую в upload URL:

```bash
UPLOAD_URL=$(
  kubectl -n ai-demo get model uploaded-model \
    -o jsonpath='{.status.upload.external}'
)

curl -fS --progress-bar -T ./model.gguf "$UPLOAD_URL" | cat
```

Gateway принимает GGUF-файлы и поддерживаемые архивы. Для обычного файла
`curl -T` отправляет `Content-Length`, поэтому модуль может зарезервировать
место до записи байтов. Если клиент не передаёт имя файла, а тип payload нельзя
надёжно определить по magic bytes, передайте имя явно:

```bash
curl -fS --progress-bar -T ./model-bundle.zip \
  "$UPLOAD_URL?filename=model-bundle.zip" | cat
```

Низкоуровневый multipart API (`/probe`, `/init`, `/parts`, `/complete`) остаётся
для будущих resumable клиентов и SDK, но для обычной загрузки он не нужен.

При включённом `artifacts.capacityLimit`, если клиент не передал
`Content-Length`, gateway отклонит запрос до записи данных.

## Ollama URL

URL вида `https://ollama.com/library/<name>[:tag]` валиден для API. Контроллер
разрешает его через Ollama registry manifest/config/blob path, выбирает ровно
один GGUF model layer, проверяет descriptor digest и GGUF magic header, затем
публикует модель через тот же внутренний `ModelPack` / `DMCR` path, что и
остальные источники.

`ai-models` не выбирает serving runtime. Он публикует source/provider evidence,
формат, семейство, parameter count, quantization, context window и artifact
facts. Будущий `ai-inference` сам решает, можно ли запускать модель через
`vLLM`, `Ollama`, `llama.cpp`, KubeRay+vLLM или другой runtime.

## Статусы

Основные фазы:

| Phase | Значение |
| --- | --- |
| `Pending` | Контроллер ещё не начал публикацию или ожидает preflight. |
| `WaitForUpload` | Создана upload-сессия, пользователь должен загрузить байты. |
| `Publishing` | Runtime публикует artifact во внутренний DMCR. |
| `Ready` | Artifact опубликован и metadata рассчитана. |
| `Failed` | Публикация завершилась ошибкой, причина в conditions. |
| `Deleting` | Идёт cleanup artifact/upload state. |

Полезные поля:

- `status.artifact.uri` — OCI artifact URI во внутреннем DMCR;
- `status.artifact.digest` — digest опубликованного artifact;
- `status.artifact.sizeBytes` — размер artifact для capacity/accounting;
- `status.resolved.format` — `Safetensors`, `GGUF` или `Diffusers`;
- `status.resolved.supportedEndpointTypes` — нормализованный serving
  contract для будущего `ai-inference`;
- `status.resolved.supportedFeatures` — входы/выходы и способности модели;
- `status.resolved.sourceCapabilities` — evidence от source provider.

## Подключение модели в workload

Указывайте только аннотацию на metadata workload'а. Контроллер сам добавит
node-cache CSI volume, mount, runtime env и internal artifact attributes.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: embedder
  namespace: ai-demo
  annotations:
    ai.deckhouse.io/model: bge-m3
spec:
  selector:
    matchLabels:
      app: embedder
  template:
    metadata:
      labels:
        app: embedder
    spec:
      containers:
        - name: app
          image: registry.example.com/embedder:latest
```

Для cluster-wide модели:

```yaml
metadata:
  annotations:
    ai.deckhouse.io/clustermodel: gemma-small
```

Для нескольких моделей:

```yaml
metadata:
  annotations:
    ai.deckhouse.io/clustermodel: gemma-small,qwen3-14b
```

Для нескольких namespaced моделей:

```yaml
metadata:
  annotations:
    ai.deckhouse.io/model: bge-m3,bge-reranker
```

В контейнере доступны:

- `AI_MODELS_MODELS_DIR=/data/modelcache/models`;
- `AI_MODELS_MODELS` — JSON со списком `name`, `path`, `digest`, `family`.

Каждая модель видна по имени ресурса:

```text
/data/modelcache/models/gemma-small
/data/modelcache/models/qwen3-14b
/data/modelcache/models/bge-m3
```

Если одновременно указаны `ai.deckhouse.io/model` и
`ai.deckhouse.io/clustermodel`, имена в итоговом списке должны быть
уникальными. Это нужно, чтобы путь `/data/modelcache/models/<name>` был
однозначным.

## Удаление

```bash
kubectl -n ai-demo delete model bge-m3
kubectl delete clustermodel gemma-small
```

Контроллер удаляет module-owned cleanup state и ставит GC-запрос для DMCR.
Во время GC объект может оставаться в `Deleting`; это нормальная часть
controller-owned lifecycle.
