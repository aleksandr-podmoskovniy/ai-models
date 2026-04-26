# Plan

## Current phase

Publication/runtime baseline и live distribution hardening.

## Orchestration

`solo`: пользователь не просил subagents; задача live-операционная, основной
риск управляется аккуратным namespace isolation и read-only inspection перед
созданием ресурсов.

## Slices

1. Baseline inventory
   - Проверить module/deploy/pod состояние `d8-ai-models`.
   - Проверить CRD schema и примеры `Model`/workload delivery.
   - Найти/подтвердить маленький Gemma-compatible artifact.

2. Test apply
   - Создать namespace для E2E.
   - Создать model object.
   - Наблюдать controller/runtime pods/jobs/secrets/events/status.

3. Workload delivery
   - Создать минимальный consumer workload.
   - Проверить mount/env/path/status/logs/events.

4. Cleanup
   - Удалить test workload/model/namespace, если это безопасно.
   - Проверить cleanup path and DMCR/GC behaviour.

5. Findings
   - Свести timeline, failures, logs completeness, secrets/storage/jobs
     замечания.
   - Отделить code defects от operational transient effects.

## Validation commands

- `kubectl --context k8s.apiac.ru get module ai-models`
- `kubectl --context k8s.apiac.ru get models -A`
- `kubectl --context k8s.apiac.ru describe model ...`
- `kubectl --context k8s.apiac.ru logs ...`
- `kubectl --context k8s.apiac.ru get events ... --sort-by=.lastTimestamp`

## Rollback point

Тестовые ресурсы живут в отдельном namespace. Rollback: удалить test workload,
test model и namespace; если finalizer зависнет, зафиксировать причину перед
ручным вмешательством.

## Live trace

- Context: `k8s.apiac.ru`, namespace `ai-models-smoke`.
- Model: `gemma-4-e2b-it-e2e-20260426071307`.
- Source: `https://huggingface.co/google/gemma-4-E2B-it`.
- Published digest:
  `sha256:297035e4d0ae58156c57ec583a8f0edb02f60a180b969caaa1d8e246d4bc336b`.
- Artifact size: `10278822057` bytes.
- Publish runtime: pod
  `d8-ai-models/ai-model-publish-232ed19b-e37f-4470-b8c1-727a43fbdd0d`
  completed with `0` restarts.
- Publish duration: roughly `8m45s` from create to `Ready`.
- Workload: Deployment
  `ai-models-smoke/gemma-4-e2b-it-e2e-20260426071307-consumer`.
- Workload delivery mode: `MaterializeBridge` with reason
  `WorkloadCacheVolume`, because live controller has
  `--node-cache-enabled=false`.
- Materializer duration: `37.4s` for layer extraction of the 10.25 GB raw
  weight layer plus companion files.
- Workload result: `Running`, `0` restarts, env contract present:
  `AI_MODELS_MODEL_PATH=/data/modelcache/model`,
  `AI_MODELS_MODEL_DIGEST=<published digest>`,
  `AI_MODELS_MODEL_FAMILY=gemma4`.
- Files available in workload: `model.safetensors` `10246621918` bytes,
  `tokenizer.json` `32169626` bytes and companion config/template files.
- Cleanup request deleted the Model CR immediately and left a
  `d8-ai-models/dmcr-gc-232ed19b-e37f-4470-b8c1-727a43fbdd0d` GC marker for
  the DMCR garbage-collection loop.
- GC marker was created at `2026-04-26T07:31:17Z`, processed at
  `2026-04-26T07:41:18Z`, and removed at `2026-04-26T07:41:21Z`.
- GC deleted the five blobs associated with the published artifact and did not
  restart any `dmcr` or controller containers.

## Findings

- Publication and workload delivery work end-to-end for
  `google/gemma-4-E2B-it`: controller status reaches `Ready`, the artifact is
  readable from DMCR, and a consumer workload can see the materialized model
  files.
- `node-cache` is disabled in the live controller flags, so this test did not
  validate the intended CSI/shared-direct target path. It validated the
  fallback path only.
- A first pre-patch ReplicaSet/pod was created with a scheduling gate, then the
  controller patched the Deployment and rolled a second pod with the
  materializer. The final rollout is healthy, but UX still shows an avoidable
  transient pending pod.
- Materializer logs are structured and include start/end/duration, but long
  extraction has no per-layer progress events between start and completion.
- Worker/materializer logs still duplicate fields such as `artifact_uri`,
  `artifact_digest`, `destination_dir`, `source_type`, and `layer_count` inside
  the same JSON record.
- Model events are too sparse for long operations: only remote-fetch started
  and publication succeeded were visible for publish; delivery adds one
  `ModelDeliveryApplied` event. There is no event-level view of layer upload,
  sealing, readback or materialization progress.
- Workload delivery projects three Secrets into the workload namespace:
  registry auth, registry CA and runtime image pull Secret. They have
  ownerReferences to the workload and disappear with the Deployment.
- Publication state and cleanup state Secrets live in `d8-ai-models` without
  ownerReferences and rely on controller cleanup. Two older orphan
  `ai-model-publish-state-*` Secrets for deleted `gemma-4-e4b-it` objects are
  still present, so state-secret garbage collection is incomplete.
- DMCR logs normal existence-probe `HEAD` misses as `level=error` with
  `blob unknown`; this is expected behavior but noisy and misleading in
  incident analysis.
- DMCR access logs are duplicated between structured registry logs and plain
  combined access logs.
- No DMCR/controller pod restarts were observed during publish, materialize or
  cleanup marker processing.
- Delete/GC correctness was acceptable for the fresh test object: workload
  projected Secrets, publish-state Secret, cleanup-state Secret and GC marker
  were removed. The remaining issue is older orphan state from previous runs,
  not this run.
