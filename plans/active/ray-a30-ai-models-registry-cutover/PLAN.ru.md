# Plan: A30 RayService cutover to ai-models registry

## 1. Current phase

Live integration validation after publication/runtime baseline hardening.

## 2. Orchestration

Mode: `solo`.

Reason: the current turn did not explicitly request delegation. The task is
primarily live inspection, GitOps manifest cutover and e2e validation.

## 3. Active bundle disposition

- `capacity-cache-admission-hardening` — keep; current code hardening slice.
- `live-e2e-ha-validation` — keep; canonical live e2e runbook.
- `observability-signal-hardening` — keep; separate logs/metrics workstream.
- `ray-a30-ai-models-registry-cutover` — current operational cutover.

## 4. Slices

### Slice 1. Discover live and GitOps state

Goal:

- identify model sources, namespace, service names, endpoints and current
  runtime path/env assumptions for A30 embedder, reranker and STT.

Files/commands:

- read `10-14-a30-*` manifests;
- `kubectl --kubeconfig /Users/myskat_90/.kube/k8s-config` inspect namespace,
  RayServices, pods, events and ai-models CRDs.

Status: done.

Evidence:

- Target context: `k8s.apiac.ru`, namespace `kuberay-projects`.
- A30 live workloads found:
  - `llm-a30-embed-rayservice` -> `deepvk/USER-bge-m3`;
  - `llm-a30-stt-rayservice` -> `openai/whisper-medium`;
  - `llm-a30-rerank` -> `qilowoq/bge-reranker-v2-m3-en-ru`.
- RayService is not currently supported by ai-models workload delivery
  controller, so the immediate cutover uses model materialization into the
  existing shared PVC path rather than `ai.deckhouse.io/clustermodel`
  annotations on RayService.

### Slice 2. Publish models through ai-models

Goal:

- create/patch model catalog resources and wait for Ready artifact digest.

Checks:

- `kubectl get model,clustermodel -A`
- source worker/upload pod logs;
- controller and DMCR restart counters.

Status: done.

Evidence:

- Created/confirmed ClusterModels:
  - `a30-user-bge-m3`, Ready, endpoint `Embeddings`,
    digest `sha256:cf4416ceb7ce9492bf78d262951bad36217070f64d86f9df94bde10fa4d4895a`;
  - `a30-whisper-medium`, Ready, endpoint `SpeechToText`,
    digest `sha256:d44e706c590cab1701e105c55b3eb57b8db3de84d5ded6072ae07916bc936104`;
  - `a30-bge-reranker-v2-m3-en-ru`, Ready, endpoint `Rerank`,
    digest `sha256:b641da84c1150a67f12af889b959a9460c18006025f15d08e61c3dff966ce0ae`.
- Manual materialization into `model-cache-pvc` succeeded after correcting the
  runtime UID mismatch by preparing target directories for runtime UID `64535`.
- Follow-up manifest design uses materializer initContainers as UID `1000:100`
  so Ray/vLLM containers can read the produced files without a separate chmod
  job.

### Slice 3. Cut over RayService manifests

Goal:

- update GitOps chart to use ai-models delivery annotations and runtime
  `AI_MODELS_*` env/path contract.

Checks:

- local diff;
- Kubernetes dry-run where possible;
- live rollout after user-applied GitOps/manual apply.

Status: done as manifest-only.

Changed external manifests:

- `09a-a30-ai-models-clustermodels.yaml` declares the three ClusterModels.
- `11-a30-embed-rayservice.yaml` points Ray Serve to
  `/data/model-cache/ai-models/a30-user-bge-m3/model` and materializes the
  published artifact in head/worker initContainers.
- `13-a30-stt-rayservice.yaml` points Ray Serve to
  `/data/model-cache/ai-models/a30-whisper-medium/model`, removes the
  HuggingFace preload initContainer and materializes the published artifact in
  head/worker initContainers.
- `14-a30-rerank-rayservice.yaml` points vLLM to
  `/data/model-cache/ai-models/a30-bge-reranker-v2-m3-en-ru/model` and
  materializes the published artifact in an initContainer.

Important operational note:

- The user explicitly requested not to touch Argo sync/push flow. The
  temporarily disabled `kuberay-service-llm` self-heal was restored to
  `automated.prune=true,selfHeal=true`; further work is manifest-only.

### Slice 4. Load validation

Goal:

- run representative requests for embedding, reranking and STT;
- collect rollout/log/metrics evidence and record defects.

Checks:

- endpoint HTTP responses;
- pod restarts;
- RayService status;
- ai-models controller/DMCR logs and events.

Status: partially done before Argo restriction.

Evidence:

- Reranker reached Ready on the local materialized path during the temporary
  live check.
- STT pending Ray cluster reached Ready and Serve logs showed
  `model_source: /data/model-cache/ai-models/a30-whisper-medium/model`.
- Full endpoint load validation is deferred until these manifest changes are
  delivered through the user's intended GitOps path.

## 5. Rollback point

Rollback is restoring the original RayService manifests and deleting only the
test `Model` / `ClusterModel` resources created by this bundle. Do not delete
shared ai-models storage accounting or DMCR state.

## 6. Final validation

- `git diff --check` in ai-models and external `k8s-config` repo.
- Cluster rollout and request evidence recorded in this plan.
- `yq eval '.'` passed for edited A30 manifests.
- `kubectl apply --dry-run=server` passed for edited A30 manifests.
- `git diff --check` passed for edited A30 manifests.
