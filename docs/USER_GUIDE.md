---
title: "User Guide"
menuTitle: "Usage"
weight: 30
description: "Creating Model and ClusterModel objects, publishing from Hugging Face or upload, status fields, and workload delivery."
---

This guide is for namespace users and application operators who need to publish
a model and attach it to a workload without manually wiring registry access,
init containers, or internal Secrets.

## Model or ClusterModel

| Resource | Scope | Use it when |
| --- | --- | --- |
| `Model` | namespace | The model belongs to one team or application namespace. |
| `ClusterModel` | cluster | The model is curated by an administrator and shared across namespaces. |

`Model` can reference a namespace-local `authSecretRef` for private Hugging Face
repositories. `ClusterModel` does not support `authSecretRef` because a
cluster-scoped object must not point at a namespaced Secret.

## Publishing from Hugging Face

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

For a private repository, create a Secret in the same namespace. Supported keys:
`token`, `HF_TOKEN`, `HUGGING_FACE_HUB_TOKEN`.

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

Check publication:

```bash
kubectl -n ai-demo get model bge-m3
kubectl -n ai-demo describe model bge-m3
kubectl -n ai-demo get model bge-m3 -o jsonpath='{.status.artifact.uri}{"\n"}'
```

## Uploaded Model

Use upload when the model is already available as a file or archive.

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

After creation, the resource enters `WaitForUpload`, and upload URLs appear in
status:

```bash
kubectl -n ai-demo get model uploaded-model \
  -o jsonpath='{.status.upload.externalURL}{"\n"}'
```

The URL is a time-bounded secret upload credential, the same UX pattern as
virtualization image upload URLs. Do not publish it in logs or tickets. It
expires at `status.upload.expiresAt`. Use `status.upload.inClusterURL` instead
when uploading from inside the cluster.

Upload the file directly to the upload URL:

```bash
UPLOAD_URL=$(
  kubectl -n ai-demo get model uploaded-model \
    -o jsonpath='{.status.upload.externalURL}'
)

curl -fS --progress-bar -T ./model.gguf "$UPLOAD_URL" | cat
```

The gateway accepts GGUF files and supported archive bundles. `curl -T` sends
`Content-Length` for regular files, so the module can reserve storage before it
writes bytes. If a client cannot send a filename and the payload cannot be
identified by magic bytes, pass an explicit filename:

```bash
curl -fS --progress-bar -T ./model-bundle.zip \
  "$UPLOAD_URL?filename=model-bundle.zip" | cat
```

The lower-level multipart API (`/probe`, `/init`, `/parts`, `/complete`) remains
available for future resumable clients and SDKs, but it is not required for the
normal upload workflow.

When `artifacts.capacityLimit` is enabled and a client does not provide
`Content-Length`, the gateway rejects the request before writing data.

## Ollama URL

URLs like `https://ollama.com/library/<name>[:tag]` are valid API input. The
controller resolves them through the Ollama registry manifest/config/blob path,
selects exactly one GGUF model layer, verifies descriptor digests and the GGUF
magic header, then publishes the model through the same internal `ModelPack` /
`DMCR` path as other sources.

`ai-models` does not choose a serving runtime. It publishes source/provider
evidence, format, family, parameter count, quantization, context window and
artifact facts. Future `ai-inference` decides whether a model can run with
`vLLM`, `Ollama`, `llama.cpp`, KubeRay+vLLM or another runtime.

## Status

Main phases:

| Phase | Meaning |
| --- | --- |
| `Pending` | The controller has not started publication or is waiting for preflight. |
| `WaitForUpload` | An upload session exists and the user must upload bytes. |
| `Publishing` | Runtime is publishing the artifact to internal DMCR. |
| `Ready` | Artifact is published and metadata is resolved. |
| `Failed` | Publication failed; inspect conditions for the reason. |
| `Deleting` | Artifact/upload cleanup is in progress. |

Useful fields:

- `status.artifact.uri` is the OCI artifact URI in internal DMCR;
- `status.artifact.digest` is the published artifact digest;
- `status.artifact.sizeBytes` is the artifact size used by capacity/accounting;
- `status.resolved.format` is `Safetensors`, `GGUF`, or `Diffusers`;
- `status.resolved.supportedEndpointTypes` is the normalized serving contract
  for future `ai-inference`;
- `status.resolved.supportedFeatures` describes model inputs, outputs, and
  capabilities;
- `status.resolved.sourceCapabilities` is provider evidence.

## Attaching a Model to a Workload

Set only an annotation on the Pod template. The controller injects the mount,
environment variables, and delivery wiring.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: embedder
  namespace: ai-demo
spec:
  selector:
    matchLabels:
      app: embedder
  template:
    metadata:
      labels:
        app: embedder
      annotations:
        ai.deckhouse.io/model: bge-m3
    spec:
      containers:
        - name: app
          image: registry.example.com/embedder:latest
```

For a cluster-wide model:

```yaml
metadata:
  annotations:
    ai.deckhouse.io/clustermodel: gemma-small
```

For multiple models:

```yaml
metadata:
  annotations:
    ai.deckhouse.io/model-refs: main=ClusterModel/gemma-small,embed=Model/bge-m3
```

Available environment variables:

- primary model: `AI_MODELS_MODEL_PATH`, `AI_MODELS_MODEL_DIGEST`,
  `AI_MODELS_MODEL_FAMILY`;
- multi-model: `AI_MODELS_MODELS_DIR`, `AI_MODELS_MODELS`;
- per-alias: `AI_MODELS_MODEL_<ALIAS>_PATH`,
  `AI_MODELS_MODEL_<ALIAS>_DIGEST`, `AI_MODELS_MODEL_<ALIAS>_FAMILY`.

Stable path for multi-model aliases:
`/data/modelcache/models/<alias>`.

## Deletion

```bash
kubectl -n ai-demo delete model bge-m3
kubectl delete clustermodel gemma-small
```

The controller removes module-owned cleanup state and requests DMCR garbage
collection. The object can stay in `Deleting` while GC is queued or running;
that is part of the controller-owned lifecycle.
