# Live cluster triage notes

## 1. Operational context

- repo-local active bundle reused as canonical workstream:
  - `plans/active/live-cluster-error-triage/*`
- explicit kubeconfig used for this triage:
  - `/Users/myskat_90/.kube/k8s-config`
- contexts in that kubeconfig:
  - `k8s-dvp.apiac.ru`
  - `k8s.ap.com`
  - `k8s.apiac.ru`
- factual reachable live cluster for this task:
  - `k8s.apiac.ru`
  - API endpoint `https://192.168.3.51:6443`
- rejected contexts during triage:
  - `k8s-dvp.apiac.ru` -> host down on `192.168.2.149:6443`
  - `k8s.ap.com` -> timeout on `192.168.2.26:6443`

## 2. Primary failure

### 2.1 Live signal

- namespace `d8-ai-models` existed and contained:
  - healthy `ai-models-controller` pods;
  - crashing `dmcr` pods.
- failing pods:
  - `dmcr-58f88b6c5f-vx5j7`
  - `dmcr-6d6dfcf8b-zfzfs`
- immediate runtime failure from `kubectl logs ... -c dmcr --previous`:
  - `panic: StorageDriver not registered: sealeds3`

### 2.2 Why it happened

- live `dmcr` config already switched to `storage.sealeds3`;
- current repo code already contains:
  - blank import in `images/dmcr/cmd/dmcr/main.go`;
  - `sealeds3` driver implementation in
    `images/dmcr/internal/registrydriver/sealeds3/driver.go`;
- local validation confirmed current source is internally consistent:
  - `cd images/dmcr && go test ./cmd/dmcr ./internal/registrydriver/sealeds3/...`
- the actual broken artifact was not the repo code, but the deployed module
  image metadata:
  - Helm release `ai-models` v169 baked `dmcr` digest
    `sha256:23bff80ee996a2580f59203a7b88233f0b982f6dfb75c77e6ec2acd9078e2627`;
  - `crane config` showed that digest was created on `2026-04-19T06:27:16Z`;
  - commit `2efe894` added `sealeds3` support later on `2026-04-21`;
  - therefore the module bundle mixed new controller/config surfaces with an
    older `dmcr` image that did not contain the `sealeds3` registration.

### 2.3 Source of truth that carried the stale digest

- `ModulePullOverride/ai-models` pointed to module image tag `main`;
- exporting `ghcr.io/aleksandr-podmoskovniy/modules/ai-models:main` showed
  `images_digests.json` with:
  - `controller = sha256:e216...`
  - `controllerRuntime = sha256:aa11...`
  - `dmcr = sha256:23bff...`
- so the stale `dmcr` digest was baked into the module artifact itself, not
  introduced by a direct manual patch of the Deployment.

## 3. Controller error signal

- no current active `ai-models-controller` reconcile failures were found during
  the post-fix check window;
- the visible controller errors were startup/leader-election lease timeouts:
  - `Error retrieving lease lock`
  - `Failed to update lease optimistically, falling back to slow path`
- these were not the primary publication/runtime failure and were not
  reproducible as an ongoing controller bug after the module reconcile
  completed.

## 4. Repair path

### 4.1 Temporary corrected module artifact

- a temporary module image was built from current module artifact `main` by
  replacing only `images_digests.json` so that:
  - `dmcr = sha256:3690bc079d3b3f283d77e200f9ca52f525c3a7c1e9d725b653ac05ad24a64ece`
- published temporary module tag:
  - `manual-main-dmcr-sealeds3-20260422-083653`
- resulting module image digest:
  - `sha256:65f370c1d5fe9ee01279af8c9d43f964770f86e2c6aa4b124541ad346e978002`

### 4.2 Cluster-side switch

- `ModulePullOverride/ai-models.spec.imageTag` switched from `main` to:
  - `manual-main-dmcr-sealeds3-20260422-083653`
- Deckhouse accepted the override and reconciled module `ai-models`;
- `deckhouse` logs confirmed:
  - module download from temporary tag;
  - successful `helm upgrade` for release `ai-models`.

## 5. End state

- new Helm release secret appeared:
  - `sh.helm.release.v1.ai-models.v170`
- release v170 baked corrected image digests:
  - `controller = sha256:e216ba57c0b9397e6245ec785139a7a1a06bfe815c171cd8aae2eb5e65e26f3b`
  - `controllerRuntime = sha256:aa11cf1c2713894801404474bf426fe478c649213f5b2d8b589e890cce1bd054`
  - `dmcr = sha256:3690bc079d3b3f283d77e200f9ca52f525c3a7c1e9d725b653ac05ad24a64ece`
- `Deployment/dmcr` moved to generation `22`;
- `dmcr` pods converged to:
  - `dmcr-89dc5f58-dgs5v` -> `4/4 Running`
  - `dmcr-89dc5f58-vf9ml` -> `4/4 Running`
- `dmcr` logs now show healthy startup instead of panic:
  - listening on `:5000`
  - probe requests return `200`
- `Module/ai-models` final state:
  - `phase = Ready`
  - `IsReady = True`
  - `IsOverridden = True`

## 6. Validations performed

- live cluster checks:
  - `kubectl ... get pods/events/describe/logs`
  - `kubectl ... get deploy dmcr -o jsonpath=...`
  - `kubectl ... get module ai-models -o yaml`
  - `kubectl ... get modulepulloverride ai-models -o yaml`
  - `kubectl ... logs deploy/deckhouse -c deckhouse --since=20m`
- repo-local validation:
  - `cd images/dmcr && go test ./cmd/dmcr ./internal/registrydriver/sealeds3/...`
- registry artifact inspection:
  - `crane config`
  - `crane export`
  - `crane ls`
  - `crane append`

## 7. Residual risk

- the current fix is a live dev override through `ModulePullOverride`, not a
  permanent release-catalog correction;
- if the override is removed before the canonical module artifact is rebuilt
  with the corrected `dmcr` digest, the cluster may return to the stale
  `dmcr` image.

## 8. `Gemma 4` smoke after the fix

### 8.1 Live objects

- the live smoke object is not a residue-only workload: cluster contains
  `ai.deckhouse.io/v1alpha1` `Model`:
  - `ai-models-smoke/gemma-4-e4b-it`
- the object is in `status.phase=Ready` with:
  - `status.artifact.uri =
    dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/namespaced/ai-models-smoke/gemma-4-e4b-it/9d2cbb33-0797-41bb-9aef-8e401e7634bf@sha256:7aa7222ddefb81e87cfba03a53d44e9c73f09484fa2bae2e3822fda227cc57c4`
  - `status.resolved.family = gemma4`
  - `status.source.resolvedRevision = 83df0a889143b1dbfc61b591bbc639540fd9ce4c`
- consumer workload:
  - `Deployment/ai-models-smoke/gemma-4-consumer`
  - workload annotation `ai.deckhouse.io/model=gemma-4-e4b-it`
  - resolved runtime annotations point to the same `DMCR` artifact digest.

### 8.2 Fresh runtime re-check against fixed `dmcr`

- after the `dmcr` fix the smoke workload was forced through one fresh
  materialization by:
  - `kubectl -n ai-models-smoke rollout restart deploy/gemma-4-consumer`
- rollout completed successfully:
  - new pod `gemma-4-consumer-667f958ffc-mrs97`
  - `1/1 Running`
- fresh materializer logs from `2026-04-22T06:04:43Z` to `2026-04-22T06:05:47Z`
  confirmed:
  - remote inspect from `DMCR`;
  - layer extraction;
  - marker write;
  - final `artifact materialization completed`.
- fresh `dmcr` logs during the same window showed one authenticated blob read
  from the new smoke pod:
  - reader principal `ai-models-reader`
  - request path under
    `/v2/ai-models/catalog/namespaced/ai-models-smoke/gemma-4-e4b-it/.../blobs/...`
- fresh controller logs showed repeated
  `runtime delivery applied` entries for `ai-models-smoke/gemma-4-consumer`
  with:
  - `delivery_mode=MaterializeBridge`
  - `delivery_reason=WorkloadCacheVolume`
  - `model_path=/data/modelcache/model`

### 8.3 Fresh publish smoke for a new object

- a second live smoke object was created on `2026-04-22`:
  - `ai-models-smoke/gemma-4-e4b-it-smoke-20260422`
- this new object is not reusing old `Ready` status; it exercises a fresh
  publication worker:
  - worker pod `ai-model-publish-42e0f858-a995-467e-9261-920daae987ae`
  - owner-scoped state secret
    `ai-model-publish-state-42e0f858-a995-467e-9261-920daae987ae`
- live worker logs confirmed the current publication path reaches:
  - remote `HuggingFace` metadata fetch;
  - source file selection;
  - profile resolution;
  - native `ModelPack` layer planning;
  - raw layer upload into internal registry;
  - final `PublicationSealing` stage after all
    `15992595884/15992595884` bytes were uploaded.
- at the end of this inspection the object state was still:
  - `status.phase=Publishing`
  - `reason=PublicationSealing`
  - worker pod still `Running`
  - no restart and no failure signal yet.

### 8.4 Practical conclusion

- the current standard smoke path now closes on the live cluster:
  - `Model` resolved and published;
  - artifact served from `DMCR`;
  - consumer materialized the artifact successfully after the `dmcr` fix.
- additionally, a fresh publish smoke for a new `Model` now proves that the
  current `source -> publication worker -> internal registry` path progresses
  through upload and sealing on the fixed cluster;
- the only missing observation at the end of this pass is the final transition
  from `PublicationSealing` to `Ready`.

## 9. Current GC contract

### 9.1 What is implemented

- `dmcr-cleaner` runs as an always-on sidecar loop in the `dmcr` Pod and
  polls request `Secret`s in `d8-ai-models`;
- the controller delete path creates owner-scoped `dmcr-gc-*` request
  `Secret`s after backend artifact cleanup;
- queued requests wait for the internal debounce window and are then armed by
  adding the `ai.deckhouse.io/dmcr-gc-switch` annotation;
- module hook `images/hooks/pkg/hooks/dmcr_garbage_collection` flips the
  internal value `aiModels.internal.dmcr.garbageCollectionModeEnabled=true`
  whenever such armed requests exist;
- rendered `dmcr` config then enables registry maintenance read-only mode, and
  `dmcr-cleaner` runs `dmcr garbage-collect ... --delete-untagged`;
- processed request secrets are deleted afterwards.

### 9.2 What is not implemented

- there is no startup sweep over the bucket or registry metadata if there are
  no GC request secrets;
- there is no public `gc.schedule`, cleanup TTL, or operator-facing cleanup
  policy knob in `ai-models` values/OpenAPI;
- there is no user-facing `check stale objects` command surface equivalent to
  `virtualization` `dvcr-cleaner gc check`.

### 9.3 Current live signal

- during this inspection the cluster had no active GC requests:
  - `kubectl -n d8-ai-models get secret -l ai.deckhouse.io/dmcr-gc-request=true`
    returned an empty list;
- `dmcr-garbage-collection` logs only showed helper startup and loop startup,
  not an active cleanup cycle.

### 9.4 Practical conclusion

- current `ai-models` GC is controller-driven cleanup for known deleted
  artifacts, not a general orphan sweep for arbitrary stale S3 contents;
- broken historical leftovers or manual storage drift are not guaranteed to be
  reclaimed automatically unless a controller-owned delete lifecycle enqueues a
  matching GC request.

## 10. Comparison with `virtualization`

### 10.1 What already matches the pattern

- `images/dmcr` explicitly states that current `DMCR` keeps the proven
  registry runtime shape from `virtualization`, but under the ai-models-owned
  packaging boundary;
- helper-side `registry + cleaner sidecar + maintenance/read-only cycle`
  pattern is retained;
- ai-models controller cleanup also follows the same general Deckhouse style:
  controller-owned lifecycle, internal request objects, and no leakage of raw
  backend cleanup internals into public API.

### 10.2 Main difference

- `virtualization` exposes GC as a productized surface:
  - `dvcr-cleaner gc auto-cleanup` performs stale-image cleanup by comparing
    registry contents with cluster resources;
  - module OpenAPI exposes `dvcr.gc.schedule`;
  - admin docs document both scheduled cleanup and manual `gc check`.
- `ai-models` intentionally does not expose that surface yet:
  - cleanup remains internal and request-driven;
  - docs explicitly say there is still no public cleanup or TTL knob.

### 10.3 Why that difference matters

- the current ai-models design is fine for delete-time reclamation of objects
  that the controller knows about;
- it is not enough for drift recovery after:
  - old broken publishes;
  - stale storage prefixes from earlier bugs;
  - manual operator intervention;
  - future runtime/cache leftovers once phase-2 shared delivery grows.

## 11. Remaining narrow points

### 11.1 Most important operational gap

- canonical module artifact publication is still inconsistent with the actual
  current code path:
  - cluster works only because of `ModulePullOverride`;
  - the release/publish pipeline still needs to produce a corrected module
    artifact so live clusters do not depend on a manual override.

### 11.2 Cleanup gap

- ai-models has no periodic or explicit stale-object sweep comparable to
  `virtualization` `dvcr.gc.schedule`;
- this means orphan storage residue remains a realistic operational debt even
  after the live `dmcr` fix.

### 11.3 Publication completion observation gap

- the new fresh publish smoke already proves the live path through upload and
  sealing, but this inspection ended before the final `Ready` transition was
  observed;
- one more short follow-up check on
  `ai-models-smoke/gemma-4-e4b-it-smoke-20260422` is enough to close this gap.

### 11.4 Runtime topology gap

- docs still explicitly state that shared-cache/node-cache behavior is
  preparatory and not yet the real workload-facing delivery contract;
- phase-1 baseline is therefore still primarily the per-pod
  `MaterializeBridge` path, which is working but not yet the end-state runtime
  topology.

### 11.5 Operator observability gap

- compared with `virtualization`, ai-models still lacks a simple explicit
  operator-facing way to ask:
  - what stale storage objects are currently eligible for cleanup;
  - how much storage can be reclaimed;
  - whether cleanup is blocked on pending requests or maintenance mode.

## 12. Follow-up regression after the temporary `dmcr` recovery

### 12.1 New live failure signal

- after the temporary `sealeds3` registration repair, `dmcr` stayed healthy as
  a process, but a fresh publish attempt for
  `ai-models-smoke/gemma-4-e4b-it-smoke-20260422` failed on manifest publish;
- controller audit log on `2026-04-22T07:06:10Z` recorded:
  - `reason=PublicationFailed`
  - `failed to publish modelpack manifest`
  - HTTP `400`
  - `MANIFEST_BLOB_UNKNOWN` for digests:
    - `sha256:cff088c266ef4c611f0eb93a52b1ea0911b4ff5a7aa32a4d447d14527957d778`
    - `sha256:cfbd3d2f1cd71bd471c37fe2bf8546d5028d41e5736f64e1ca6c6b8893125503`
    - `sha256:cc8d3a0ce36466ccc1278bf987df5f71db1719b9ca6b4118264f45cb627bfe0f`
- matching `dmcr` log sequence showed:
  - layer upload itself succeeded with `POST .../blobs/uploads` -> `202` and
    `PUT ...?digest=...` -> `201`;
  - manifest publish then logged upstream warning
    `blob path should not be a directory` for canonical blob
    `/docker/registry/v2/blobs/.../data` paths;
  - registry responded `400 manifest blob unknown`.

### 12.2 Root cause localized in repo code

- warning string comes from upstream distribution `blobstore.Stat`, which
  treats `driver.Stat(path).IsDir()==true` as blob corruption and returns
  `ErrBlobUnknown`;
- current `sealeds3` adapter resolved sealed blobs only when delegate `Stat`
  returned `PathNotFound`;
- S3-backed `Stat` can legitimately resolve the canonical blob data path as a
  directory/prefix because the only concrete object at that location is the
  sidecar metadata object `.../data.dmcr-sealed`;
- therefore manifest validation did not see the sealed blob indirection and
  rejected fresh publishes even though the physical object and metadata were
  already present.

### 12.3 Local fix implemented and verified

- `images/dmcr/internal/registrydriver/sealeds3/driver.go`
  - `Stat()` now falls back to sealed metadata not only on
    `PathNotFound`, but also when the delegate returns directory-like file info
    for a canonical blob path;
  - virtual blob projection stays unchanged for normal file results and still
    preserves the old upstream-style directory response if no sealed metadata
    exists.
- `images/dmcr/internal/registrydriver/sealeds3/driver_test.go`
  - fake storage driver now can simulate directory-like `Stat` responses;
  - added coverage for the exact regression shape:
    canonical blob path reported as directory while metadata points to the
    physical object.
- validations after the change:
  - `cd images/dmcr && go test ./...`
  - repo-root `make verify`

### 12.4 Current live state after local fix

- reachable cluster for this workstream is still `k8s.apiac.ru`;
- current live workloads there:
  - `d8-ai-models/ai-models-controller` and `d8-ai-models/dmcr` remain
    `Running`;
  - `ai-models-smoke/gemma-4-consumer` remains `Running` and still references
    previously published digest
    `sha256:7aa7222ddefb81e87cfba03a53d44e9c73f09484fa2bae2e3822fda227cc57c4`;
  - `kubectl get model,clustermodel -A` currently returns an empty list, so
    only the historical consumer deployment remains observable right now.
- cluster images are still old and therefore do not yet contain the new local
  fixes:
  - controller main container:
    `ghcr.io/aleksandr-podmoskovniy/modules/ai-models@sha256:e216ba57c0b9397e6245ec785139a7a1a06bfe815c171cd8aae2eb5e65e26f3b`
  - upload gateway:
    `ghcr.io/aleksandr-podmoskovniy/modules/ai-models@sha256:aa11cf1c2713894801404474bf426fe478c649213f5b2d8b589e890cce1bd054`
  - `dmcr`, `dmcr-direct-upload`, `dmcr-garbage-collection`:
    `ghcr.io/aleksandr-podmoskovniy/modules/ai-models@sha256:3690bc079d3b3f283d77e200f9ca52f525c3a7c1e9d725b653ac05ad24a64ece`

### 12.5 Immediate deployment blocker from this workstation

- this session can validate repo code and inspect the cluster, but cannot
  publish a replacement module image right now:
  - local `docker` daemon is unavailable;
  - `buildah`, `podman`, `nerdctl`, `colima`, and `lima` are absent;
  - shell environment does not expose CI registry credentials or
    `WERF_REPO`/`MODULES_REGISTRY_*` variables needed for `werf` publish.
- practical consequence:
  - code fix is ready and verified locally;
  - live cluster still needs one real image build and rollout before the fresh
    publish smoke can be re-run.

## 13. Continuation on 2026-04-22: live rollout, CRD drift and current sealing gap

### 13.1 Live image rollout from this workstation became possible

- the workstation can publish temporary artifacts without Docker by using
  `crane` against
  `ghcr.io/aleksandr-podmoskovniy/modules/ai-models`;
- temporary live image tags were published from current repo code:
  - `manual-dmcr-livefix-20260422-103526` ->
    `sha256:7af095da4bbe9b5ecb496df7a65c7a0891cb8a67dc94f22c940622b508c0b5a1`
  - `manual-controller-livefix-20260422-103526` ->
    `sha256:569a2a3689a143a804d93494c903f689503cd33022aa6614beab663e554b8caf`
  - `manual-controller-runtime-livefix-20260422-103526` ->
    `sha256:cb93ea8cda3d9e53a9502e9174dba12b02a3e77e0a6cf585e2c3db4a364ff5a6`
- temporary module artifact with updated image digests:
  - `manual-main-livefix-20260422-103705` ->
    `sha256:13b92f1524550f83faf6a5df886247f47b7b4e01ac31e48df649598cf1d90cef`
- after switching `ModulePullOverride/ai-models` to that tag, live deployments
  in `k8s.apiac.ru` rolled to the new controller/runtime/dmcr images.

### 13.2 `status.progress` live break was a CRD packaging drift, not a controller bug

- once the new controller image started writing top-level `status.progress`,
  live controller logs switched to:
  - `unknown field "status.progress"`
- generated repo CRDs were stale at that point; after running:
  - `bash api/scripts/update-codegen.sh`
  - `bash api/scripts/verify-crdgen.sh`
  repo CRDs gained:
  - printcolumn `Progress`
  - `status.progress`
  - `status.upload.authorizationHeaderValue`
- first temporary CRD module tag
  `manual-main-livefix-crd-20260422-104417` was packaged incorrectly for
  Deckhouse module loader:
  - Deckhouse error:
    `open /tmp/package.../crds/ai.deckhouse.io_models.yaml: no such file or directory`
  - root cause: the appended layer carried CRD files without an explicit
    `crds/` directory entry.
- corrected temporary CRD module tag:
  - `manual-main-livefix-crd2-20260422-105200` ->
    `sha256:7aba65350ff63e1ab47b20edc38386d83d5365a787e864fc8a10fc0d3f21a9c9`
- Deckhouse accepted the corrected package and restarted with the new override:
  - `ModulePullOverride/ai-models.status.imageDigest` moved to
    `sha256:7aba65350ff63e1ab47b20edc38386d83d5365a787e864fc8a10fc0d3f21a9c9`
  - live CRD `models.ai.deckhouse.io` now exposes:
    - printcolumn `Progress|.status.progress`
    - `status.progress.type=string`

### 13.3 Current fresh `gemma 4` smoke after CRD repair

- fresh live object:
  - `ai-models-smoke/gemma-4-e4b-it-livefix-20260422`
- live status after CRD repair no longer drops progress on the floor:
  - `phase=Publishing`
  - `progress` moved through `81%`, `84%`, `90%`, `94%`, `97%`, `99%`
  - current reason switched from `PublicationUploading` to
    `PublicationSealing`
- current live object snapshot:
  - `status.progress=99%`
  - condition message:
    `controller is sealing the current model artifact layer in the internal registry after 15992626604/16024796230 uploaded bytes`
- persisted direct-upload checkpoint in secret
  `ai-model-publish-state-20a1c92a-62bc-4545-be7b-016b88361f6b` proves:
  - multipart upload is complete for the current raw layer;
  - checkpoint `stage=Sealing`;
  - `currentLayer.uploadedSizeBytes == currentLayer.totalSizeBytes`.

### 13.4 What `PublicationSealing` means in the current implementation

- this state is not a fake percentage stall produced by status code;
- current code path is:
  - controller worker uploads the full raw layer;
  - marks checkpoint as `Sealing`;
  - calls DMCR direct-upload `/v2/blob-uploads/complete`;
  - DMCR completes multipart upload;
  - DMCR then re-reads the whole uploaded object in
    `sealUploadedObject()` to compute a trusted SHA256 and size before writing
    sealed metadata and repository link objects.
- practical consequence for large models:
  - after the first full upload pass, `PublicationSealing` adds a second full
    read of the same large object from object storage;
  - for `gemma-4-E4B-it` this is a real phase-1 narrow point in both latency
    and operator UX.

### 13.5 Current remaining live gap

- the old `unknown field "status.progress"` spam stopped after the CRD repair
  window; subsequent live object status now carries `progress` correctly;
- repeated `runtime delivery applied` controller spam did not reproduce in the
  last 30-minute inspection window after the runtime-delivery log contract fix;
- however, the fresh `gemma-4-e4b-it-livefix-20260422` smoke has not yet been
  observed to transition from `PublicationSealing` to `Ready` during this
  session.

### 13.6 Local code follow-up applied for operator observability

- narrow implementation slice landed locally to make the sealing path
  explainable:
  - controller runtime now logs:
    - `native modelpack direct upload sealing started`
    - `native modelpack direct upload sealing completed`
  - DMCR direct-upload service now logs:
    - complete request start
    - sealing start
    - sealing completion with digest/size/duration
    - explicit failures on multipart completion, sealing, metadata and link
      writes
- local validations for this observability slice:
  - `cd images/controller && go test ./internal/adapters/modelpack/oci/...`
  - `cd images/dmcr && go test ./internal/directupload/...`

### 13.7 Follow-up live observation after the CRD repair window

- by `2026-04-22 11:08 MSK` the transient smoke object
  `ai-models-smoke/gemma-4-e4b-it-livefix-20260422` no longer existed in the
  namespace, but the runtime side of the smoke still remained:
  - `Deployment/gemma-4-consumer` was still present and `Available`;
  - its annotation still pointed to `ai.deckhouse.io/model=gemma-4-e4b-it`;
  - the init/runtime image already used the new controller/runtime digest
    `sha256:cb93ea8cda3d9e53a9502e9174dba12b02a3e77e0a6cf585e2c3db4a364ff5a6`.
- the publish worker and persisted checkpoint for the deleted smoke attempt
  were still alive:
  - pod
    `d8-ai-models/ai-model-publish-20a1c92a-62bc-4545-be7b-016b88361f6b`
    kept `STATUS=Running`;
  - secret
    `d8-ai-models/ai-model-publish-state-20a1c92a-62bc-4545-be7b-016b88361f6b`
    still held `phase=Running`, `stage=Sealing`;
  - both objects had empty `ownerReferences`.
- this sharpens the remaining phase-1 gap:
  - the system currently allows the catalog-side `Model` object to disappear
    while the publication worker/checkpoint continue independently;
  - the operator loses the primary CR status surface, but the heavy backend
    activity is still running.
- `dmcr-direct-upload` logs in the same window still showed only repetitive
  TLS handshake EOFs, without a business-level completion/failure signal for
  this sealing step.

### 13.8 Local lifecycle fix for in-flight publication deletion

- the live orphan state exposed a second root cause beyond the long
  `PublicationSealing` phase:
  - cleanup finalizer used to appear only after a `cleanupHandle` was written;
  - deleting a `Model` during an active publish/upload runtime could therefore
    remove the catalog object immediately, while cross-namespace runtime
    resources in `d8-ai-models` kept running independently.
- bounded corrective slice landed locally:
  - active `Model` / `ClusterModel` objects now get the cleanup finalizer even
    before a `cleanupHandle` exists;
  - delete flow now explicitly observes in-flight publication runtime
    resources:
    - source worker pod;
    - source worker auth/state secrets;
    - projected OCI auth/CA secrets;
    - upload session secret;
  - when such resources still exist and no `cleanupHandle` is present yet,
    delete flow removes those runtime resources first, keeps the finalizer,
    requeues, and only then releases the object.
- this keeps the ownership model aligned with DKP expectations:
  - the catalog object remains the delete gate for runtime resources;
  - cross-namespace worker artifacts are no longer allowed to outlive an early
    delete just because controller references cannot be used there.
- validations for the lifecycle slice:
  - `cd images/controller && go test ./internal/application/deletion/... ./internal/controllers/catalogcleanup/... ./internal/adapters/modelpack/oci/... ./cmd/ai-models-controller/...`
  - `cd images/dmcr && go test ./internal/directupload/...`
