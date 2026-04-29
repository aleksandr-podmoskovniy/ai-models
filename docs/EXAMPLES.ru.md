---
title: "Примеры"
menuTitle: "Примеры"
weight: 70
description: "Готовые YAML-примеры для включения ai-models, публикации моделей и подключения их в workload."
---

Все примеры используют публичный контракт модуля. Не добавляйте вручную
`materialize-artifact` initContainer, registry credentials и DMCR path в
прикладные манифесты. Задайте только аннотацию модели; workload delivery
controller сам добавит node-cache CSI volume, artifact attributes и
workload-facing env/mount contract.

## Минимальный ModuleConfig

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ai-models
spec:
  enabled: true
  version: 1
  settings:
    artifacts:
      bucket: ai-models
      endpoint: https://s3.example.com
      region: us-east-1
      credentialsSecretName: ai-models-artifacts
      usePathStyle: true
```

## ModuleConfig с capacity limit

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ai-models
spec:
  enabled: true
  version: 1
  settings:
    artifacts:
      bucket: ai-models
      endpoint: https://s3.example.com
      region: us-east-1
      credentialsSecretName: ai-models-artifacts
      capacityLimit: 1Ti
```

## ModuleConfig с managed node-cache

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ai-models
spec:
  enabled: true
  version: 1
  settings:
    artifacts:
      bucket: ai-models
      endpoint: https://s3.example.com
      region: us-east-1
      credentialsSecretName: ai-models-artifacts
    nodeCache:
      enabled: true
      size: 200Gi
```

По умолчанию ноды и `BlockDevice` выбираются по label
`ai.deckhouse.io/model-cache=true`:

```bash
kubectl label node k8s-w3-gpu ai.deckhouse.io/model-cache=true
kubectl label blockdevice <bd-name> ai.deckhouse.io/model-cache=true
```

## Namespaced Model из Hugging Face

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

## Приватный Hugging Face Model

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hf-token
  namespace: ai-demo
type: Opaque
stringData:
  token: hf_xxx
---
apiVersion: ai.deckhouse.io/v1alpha1
kind: Model
metadata:
  name: private-model
  namespace: ai-demo
spec:
  source:
    url: https://huggingface.co/acme/private-model
    authSecretRef:
      name: hf-token
```

## ClusterModel

```yaml
apiVersion: ai.deckhouse.io/v1alpha1
kind: ClusterModel
metadata:
  name: gemma-small
spec:
  source:
    url: https://huggingface.co/google/gemma-3-4b-it
```

## GGUF-модель из Ollama registry

```yaml
apiVersion: ai.deckhouse.io/v1alpha1
kind: ClusterModel
metadata:
  name: qwen-gguf
spec:
  source:
    url: https://ollama.com/library/qwen3.6:latest
```

## Upload Model

```yaml
apiVersion: ai.deckhouse.io/v1alpha1
kind: Model
metadata:
  name: uploaded-safetensors
  namespace: ai-demo
spec:
  source:
    upload: {}
```

```bash
kubectl -n ai-demo wait --for=jsonpath='{.status.phase}'=WaitForUpload model/uploaded-safetensors
UPLOAD_URL=$(kubectl -n ai-demo get model uploaded-safetensors -o jsonpath='{.status.upload.external}')
curl -fS --progress-bar -T ./model-bundle.zip "$UPLOAD_URL?filename=model-bundle.zip" | cat
```

## Deployment с одной моделью

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: embedder
  namespace: ai-demo
  annotations:
    ai.deckhouse.io/model: bge-m3
spec:
  replicas: 2
  selector:
    matchLabels:
      app: embedder
  template:
    metadata:
      labels:
        app: embedder
    spec:
      containers:
        - name: embedder
          image: registry.example.com/embedder:latest
```

## Deployment с ClusterModel

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: generator
  namespace: ai-demo
  annotations:
    ai.deckhouse.io/clustermodel: gemma-small
spec:
  selector:
    matchLabels:
      app: generator
  template:
    metadata:
      labels:
        app: generator
    spec:
      containers:
        - name: generator
          image: registry.example.com/generator:latest
```

## Workload с несколькими моделями

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rag-service
  namespace: ai-demo
  annotations:
    ai.deckhouse.io/model-refs: main=ClusterModel/gemma-small,embed=Model/bge-m3
spec:
  selector:
    matchLabels:
      app: rag-service
  template:
    metadata:
      labels:
        app: rag-service
    spec:
      containers:
        - name: rag-service
          image: registry.example.com/rag-service:latest
```

В контейнере:

- `AI_MODELS_MODEL_MAIN_PATH`;
- `AI_MODELS_MODEL_EMBED_PATH`;
- `AI_MODELS_MODELS_DIR=/data/modelcache/models`.

## Внешние контроллеры

ai-models не мутирует сторонние CRD по имени. Для operator'ов вроде KubeRay
не ожидайте, что ai-models поймёт higher-level CRD. Рендерите поддержанный
Kubernetes workload (`Deployment`, `StatefulSet`, `DaemonSet` или `CronJob`) с
ai-models annotation на metadata workload'а, либо позже отдавайте этот рендер
ai-inference.

Для обычных Kubernetes workload'ов тот же контракт выглядит так:

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
        - name: embedder
          image: registry.example.com/embedder:latest
```
