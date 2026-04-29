---
title: "Examples"
menuTitle: "Examples"
weight: 70
description: "Ready-to-use YAML examples for enabling ai-models, publishing models, and attaching them to workloads."
---

All examples use the public module contract. Do not manually add
`materialize-artifact` init containers, registry credentials, or DMCR paths to
application manifests. Declare model annotations only; workload delivery
injects the node-cache CSI volume, artifact attributes, and workload-facing
env/mount contract.

## Minimal ModuleConfig

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

## ModuleConfig with Capacity Limit

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

## ModuleConfig with Managed Node Cache

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

By default, nodes and `BlockDevice` objects are selected by
`ai.deckhouse.io/model-cache=true`:

```bash
kubectl label node k8s-w3-gpu ai.deckhouse.io/model-cache=true
kubectl label blockdevice <bd-name> ai.deckhouse.io/model-cache=true
```

## Namespaced Hugging Face Model

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

## Private Hugging Face Model

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

## Ollama Registry GGUF Model

```yaml
apiVersion: ai.deckhouse.io/v1alpha1
kind: ClusterModel
metadata:
  name: qwen-gguf
spec:
  source:
    url: https://ollama.com/library/qwen3.6:latest
```

## Uploaded Model

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

## Deployment with One Model

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

## Deployment with ClusterModel

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

## Multi-Model Workload

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

Inside the container:

- `AI_MODELS_MODEL_MAIN_PATH`;
- `AI_MODELS_MODEL_EMBED_PATH`;
- `AI_MODELS_MODELS_DIR=/data/modelcache/models`.

## External Controllers

ai-models does not mutate third-party CRDs by name. For operators such as
KubeRay, do not expect ai-models to understand the higher-level CRD. Render a
supported Kubernetes workload (`Deployment`, `StatefulSet`, `DaemonSet` or
`CronJob`) with the ai-models annotation on workload metadata, or let
ai-inference own that rendering later.

For plain Kubernetes workloads the same contract looks like this:

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
