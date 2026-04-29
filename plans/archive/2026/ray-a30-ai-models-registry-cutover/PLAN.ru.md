# Plan: A30 RayService cutover to ai-models registry

## 1. Current phase

Live integration validation after publication/runtime baseline hardening.

## 2. Orchestration

Mode: `solo`.

Reason: the current turn did not explicitly request delegation. The task is
primarily live inspection, GitOps manifest cutover and e2e validation.

## 3. Active bundle disposition

- `live-e2e-ha-validation` — keep; canonical live e2e runbook.
- `observability-signal-hardening` — keep; separate executable logs/metrics
  workstream.
- `ray-a30-ai-models-registry-cutover` — keep; current operational cutover,
  pending post-rollout endpoint load validation.
- `capacity-cache-admission-hardening` — archived; implementation is complete
  and live proof belongs to `live-e2e-ha-validation`.

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
- Defect found: RayService was not supported by ai-models workload delivery
  controller, so the earlier manifest-only workaround leaked internal
  `materialize-artifact` wiring into GitOps manifests. This is now treated as
  controller feature work, not as acceptable workload config.

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

### Slice 3. Add Argo-safe KubeRay workload delivery

Goal:

- keep `RayService` as GitOps-owned declaration object with only
  user-facing ai-models annotations;
- mutate only generated `RayCluster` pod templates from the same model delivery
  annotations;
- keep clusters without KubeRay safe by enabling the RayCluster controller only
  when both `ray.io/v1 RayService` and `ray.io/v1 RayCluster` exist in
  discovery.

Files:

- `images/controller/internal/controllers/workloaddelivery/*ray*`
- `images/controller/internal/controllers/workloaddelivery/setup.go`
- `templates/controller/webhook.yaml`

Status: done.

Evidence:

- `RayService` is registered only as the annotation/owner source and is not
  patched by reconciler or webhook.
- `RayCluster` is registered as the runtime target and receives generated
  pod-template delivery state.
- `RayService` annotation updates enqueue owned RayClusters.
- Model/ClusterModel readiness updates enqueue RayClusters through indexed
  RayService declarations.
- The webhook mutates `RayCluster`, not `RayService`, so ArgoCD no longer sees
  ai-models-owned drift on `RayService.spec`.
- Regression tests cover generated RayCluster admission through owner
  RayService annotations and RayCluster reconciliation over head/worker
  templates.

Checks:

- `cd images/controller && go test ./internal/controllers/workloaddelivery`
- `cd images/controller && go test ./...`
- `make helm-template`
- `make kubeconform`

Validation:

- `cd images/controller && go test ./internal/controllers/workloaddelivery`
  passed.
- `cd images/controller && go test ./...` passed.
- `make helm-template` passed.
- `make kubeconform` passed.
- `make verify` passed after splitting the RayCluster pending path to keep
  cyclomatic complexity below the repository threshold.
- MutatingWebhookConfiguration server dry-run passed after rendering with a
  temporary non-Deckhouse name and without the protected `heritage: deckhouse`
  label; this validates the CEL `matchConditions` expression against the API
  server.

### Slice 4. Cut over RayService manifests

Goal:

- update GitOps chart to use ai-models delivery annotations and runtime
  `AI_MODELS_*` env/path contract.

Checks:

- local diff;
- Kubernetes dry-run where possible;
- live rollout after user-applied GitOps/manual apply.
- `yq eval '.'` for edited A30 manifests.
- `git diff --check` for edited external manifests.

Status: done as annotation-only manifests.

Changed external manifests:

- `09a-a30-ai-models-clustermodels.yaml` declares the three ClusterModels.
- `11-a30-embed-rayservice.yaml` uses
  `ai.deckhouse.io/model-refs: model=ClusterModel/a30-user-bge-m3` and points
  Ray Serve to `/data/modelcache/models/model`.
- `13-a30-stt-rayservice.yaml` uses
  `ai.deckhouse.io/model-refs: model=ClusterModel/a30-whisper-medium` and
  points Ray Serve to `/data/modelcache/models/model`.
- `14-a30-rerank-rayservice.yaml` uses the matching `model-refs`
  ClusterModel reference; vLLM points to `/data/modelcache/models/model`.
- The three manifests no longer contain manual `materialize-artifact`
  initContainers, raw DMCR URI/digest, registry auth/CA, runtime image, or
  `mkdir/chown` permission jobs.

Important operational note:

- The user explicitly requested not to touch Argo sync/push flow. The
  temporarily disabled `kuberay-service-llm` self-heal was restored to
  `automated.prune=true,selfHeal=true`; further work is manifest-only.

### Slice 5. Load validation

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
- STT pending Ray cluster reached Ready and Serve logs showed the old manual
  path before this cleanup. Full endpoint load validation must be repeated
  after the annotation-only controller build is deployed.
- Full endpoint load validation is deferred until these manifest changes are
  delivered through the user's intended GitOps path.

## 5. Rollback point

Rollback is restoring the original RayService manifests and deleting only the
test `Model` / `ClusterModel` resources created by this bundle. Do not delete
shared ai-models storage accounting or DMCR state.

## 6. Final validation

- `git diff --check` in ai-models and external `k8s-config` repo.
- `cd images/controller && go test ./internal/controllers/workloaddelivery`
- `cd images/controller && go test ./...`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `yq eval '.'` for edited A30 manifests.
- `kubectl apply --dry-run=server -n kuberay-projects` for edited A30
  manifests.
- Cluster rollout and request evidence recorded in this plan.
