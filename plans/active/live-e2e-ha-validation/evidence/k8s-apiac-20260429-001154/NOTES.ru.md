# Evidence: k8s.apiac.ru live e2e/HA

Status: completed with one code fix and two follow-up observations.

## Cluster target

- context: k8s.apiac.ru
- kubeconfig: /Users/myskat_90/.kube/k8s-config

## What Passed

- Platform gate: `ai-models` module Ready/Enabled, controller/upload-gateway/DMCR
  replicas were on worker nodes and stayed without restarts during the test.
- HF publication: tiny Safetensors, GGUF and Diffusers models reached `Ready`.
- HA publication: `google/gemma-4-E2B-it` (`~10.27 GB`) reached `Ready` after
  controlled deletion of the active controller leader, one DMCR Pod and the
  publish worker Pod.
- Upload publication: upload session issued, probe/init/presigned PUT completed,
  staged archive published and reached `Ready`.
- RBAC: user-authz and rbacv2/use/manage live `can-i` matrix passed, including
  deny checks for Secrets, pod logs/exec and `/status`.
- Single-model workload delivery: MaterializeBridge injected init container,
  projected registry auth/runtime pull secret, materialized the model and
  exposed `AI_MODELS_*` env to the workload.
- Negative workload delivery: a workload without `/data/modelcache` was blocked
  with scheduling gate and clear `ModelDeliveryBlocked` event.
- Monitoring: controller metrics endpoint exposed catalog/runtime/storage
  collectors with `collector_up=1`; Prometheus had no active ai-models/DMCR
  alerts.
- Cleanup: e2e workloads, Models, ClusterModel and temporary RBAC subjects were
  removed. DMCR GC requests reached `done`; GC reported
  `deletedRegistryBlobCount=22`, zero stale/open multipart leftovers and only a
  bounded `registryOutputSHA256` summary.

## Defects Found

- Multi-model MaterializeBridge dropped the projected runtime `imagePullSecret`
  from workload Pod templates. Root cause: alias render added the secret and
  then pruned the same name in `applyRendered`. Fixed in code:
  `internal/adapters/k8s/modeldelivery/render_alias.go` and covered by
  `internal/controllers/workloaddelivery/reconciler_multi_model_test.go`.

## Follow-Up Observations

- Blocked workload delivery emits one user event, but controller logs repeated
  `runtime delivery blocked by workload spec` several times during the
  reconcile burst. This is lower priority log-noise hardening.
- DMCR logs the expected post-delete manifest `404` as `level=error` during
  cleanup verification. This does not break cleanup, but is noisy for operations.
- DMCR metrics were not scraped directly through local port-forward because the
  kube-rbac-proxy metrics endpoint is not reachable on localhost inside the Pod
  network namespace. Prometheus rule and active alerts were still checked.
