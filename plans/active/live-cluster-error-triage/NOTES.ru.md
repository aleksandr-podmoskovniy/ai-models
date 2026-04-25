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

### 13.9 Local corrective slice for controller LOC gate

- a follow-up local quality gate failure surfaced after the lifecycle and
  observability slices:
  - `tools/check-controller-loc.sh` rejected
    `images/controller/internal/adapters/modelpack/oci/direct_upload_transport.go`
    at `353` lines with the controller file-size threshold set to `350`;
- this was a structure problem, not a reason to grow the allowlist:
  - described direct-upload orchestration, session resume, part transport and
    shared uploaded-part helpers had drifted into one file;
- the local slice split that boundary back into explicit files:
  - `direct_upload_transport.go` keeps the described upload orchestration path;
  - `direct_upload_transport_described_session.go` owns described
    session/resume state opening;
  - `direct_upload_transport_described_parts.go` owns described part upload and
    recovery helpers;
  - `upload_transport_common.go` now owns shared direct-upload part
    normalization and chunk-position helpers used by both described and raw
    paths.
- local validations after the split passed through the full repo gate:
  - `go test ./internal/adapters/modelpack/oci/...` from `images/controller`
  - `make lint-controller-size`
  - `make lint-controller-complexity`
  - `make lint`
  - `make verify`

## 14. 2026-04-23 live S3 deletion triage

### 14.1 Live status

- kube-context during this check:
  - `k8s-main`
- module status:
  - `Module/ai-models` is `Ready`;
  - `ModulePullOverride/ai-models.spec.imageTag=main`;
  - override status digest is
    `sha256:de80b072242cba92ac37d656e7c97d52446c4ee22c20088c365ecb848e00be58`;
  - current Helm release is `sh.helm.release.v1.ai-models.v189`, created at
    `2026-04-23T18:25:53Z`.
- `d8-ai-models` pods are healthy after the v189 rollout:
  - `ai-models-controller-5bc8897dbd-*` -> `3/3 Running`;
  - `dmcr-696d574667-*` -> `4/4 Running`.
- live catalog objects are absent:
  - no `models.ai.deckhouse.io` in any namespace;
  - no `clustermodels.ai.deckhouse.io`;
  - no legacy `models.ai-models.deckhouse.io` or
    `clustermodels.ai-models.deckhouse.io`.

### 14.2 S3/DMCR residue

- active GC request secrets are absent:
  - `kubectl -n d8-ai-models get secret -l ai.deckhouse.io/dmcr-gc-request=true`
    returned no resources.
- current `dmcr-garbage-collection` logs only show helper startup and loop
  startup; there is no `dmcr garbage collection requested`, `completed`, or
  `requests removed` entry in the current pod logs.
- report-only `dmcr-cleaner gc check` from the live `dmcr` Pod found:
  - `Live repository prefixes: 0`;
  - `Stored repository prefixes: 0`;
  - `Stored direct-upload object prefixes: 6`;
  - `Open direct-upload multipart uploads: 3`;
  - `Open direct-upload multipart parts: 2905`;
  - `Stale orphan direct-upload object prefixes: 6`;
  - `Stale orphan direct-upload multipart uploads: 3`.
- the stale direct-upload object prefixes are under:
  - `dmcr/_ai_models/direct-upload/objects/03e7186f4878afef660c42d38c938b83`;
  - `dmcr/_ai_models/direct-upload/objects/5994e9ea0ccef69d6730ddb71183c8a3`;
  - `dmcr/_ai_models/direct-upload/objects/5ed31bf34da8c04d49e5a29d53fb28c2`;
  - `dmcr/_ai_models/direct-upload/objects/874c887122bc7d6735a62b91924223ba`;
  - `dmcr/_ai_models/direct-upload/objects/9c875ea629fcbd752b29c4b04e0b119e`;
  - `dmcr/_ai_models/direct-upload/objects/d50d868e4b4425e3fae261f54e1f4af8`.
- the stale multipart uploads started at:
  - `2026-04-22T19:31:50.232Z`;
  - `2026-04-23T08:26:29.62Z`;
  - `2026-04-23T09:15:31.714Z`.

### 14.3 Why S3 was not deleted

- current cleaner can see the stale S3 residue, so this is not an S3
  visibility/auth problem.
- the automatic cleaner did not run because there was no trigger:
  - no controller-created `dmcr-gc-*` request secret exists;
  - no scheduled `dmcr-gc-scheduled` request exists at the moment of this
    check;
  - the current helper command uses `--schedule=0 2 * * *`, so after the
    v189 pod start at `2026-04-23T18:25:53Z` the next scheduled enqueue is the
    next `02:00 UTC` tick, not an immediate backfill.
- two completed publication state secrets remain without owner references:
  - `ai-model-publish-state-5fbeb2ff-06ce-4903-ab9b-fe19335505ac`;
  - `ai-model-publish-state-6c00336f-5173-4629-a387-b829cb4ac584`;
  - both point to `Model ai-models-smoke/gemma-4-e4b-it`;
  - both have `phase=Completed`, `stage=Idle`, `plannedLayerCount=3`, and
    `plannedSizeBytes=16024796230`.
- practical failure mode:
  - the catalog objects are gone;
  - the delete lifecycle did not leave a matching GC request;
  - the remaining residue is now only discoverable by the global stale sweep
    path (`dmcr-cleaner gc check` / scheduled cleanup), not by a per-object
    finalizer path.

### 14.4 Immediate operational conclusion

- do not treat this as `dmcr` being unable to delete from S3;
- the current break is trigger/lifecycle ordering:
  - object deletion completed without an active `dmcr-gc-*` handoff;
  - daily scheduled cleanup has not reached its next tick after the latest
    rollout;
  - therefore S3 still contains stale direct-upload data and multipart parts.
- manual destructive cleanup was not run during this triage.

### 14.5 Manual cleanup run

- manual cleanup was explicitly requested and run at:
  - `2026-04-23T19:13:15Z`;
  - `2026-04-23 22:13:15 MSK`.
- pre-cleanup guard checks were repeated immediately before deletion:
  - no `models.ai.deckhouse.io`;
  - no `clustermodels.ai.deckhouse.io`;
  - no active `ai.deckhouse.io/dmcr-gc-request=true` secrets.
- command used:
  - `kubectl -n d8-ai-models exec deploy/dmcr -c dmcr-garbage-collection -- /usr/local/bin/dmcr-cleaner gc auto-cleanup`
- cleanup completed successfully.
- registry GC output:
  - `0 blobs marked, 0 blobs and 0 manifests eligible for deletion`.
- post-cleanup `dmcr-cleaner gc check` result:
  - `Stored direct-upload object prefixes: 0`;
  - `Open direct-upload multipart uploads: 0`;
  - `Open direct-upload multipart parts: 0`;
  - `Stale orphan direct-upload object prefixes: 0`;
  - `Stale orphan direct-upload multipart uploads: 0`;
  - `No stale prefixes eligible for cleanup.`
- post-cleanup module status:
  - `Module/ai-models` remains `Ready`;
  - `ai-models-controller` pods are `3/3 Running`;
  - `dmcr` pods are `4/4 Running`.

### 14.6 Fresh publish/delete/GC smoke after manual cleanup

- fresh small smoke object was created after the manual S3 cleanup:
  - namespace: `ai-models-smoke`;
  - object: `Model/tiny-random-phi-gc-20260423-1`;
  - source: `https://huggingface.co/hf-internal-testing/tiny-random-PhiForCausalLM`.
- the object reached `Ready` and published a new artifact:
  - model UID: `0295accc-9a90-4843-909e-f5be00394133`;
  - digest:
    `sha256:430809d43231b77dd8b4e64ec098976d77b6081838f23dd2328adbc36fbd6526`;
  - size: `382119` bytes;
  - artifact URI:
    `dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/namespaced/ai-models-smoke/tiny-random-phi-gc-20260423-1/0295accc-9a90-4843-909e-f5be00394133@sha256:430809d43231b77dd8b4e64ec098976d77b6081838f23dd2328adbc36fbd6526`.
- the live object carried the expected cleanup ownership markers before
  deletion:
  - `ai.deckhouse.io/model-cleanup` finalizer;
  - cleanup handle annotation.
- deletion command:
  - `kubectl -n ai-models-smoke delete models.ai.deckhouse.io tiny-random-phi-gc-20260423-1 --wait=false`.
- delete-triggered cleanup behaved as expected:
  - cleanup job:
    `ai-model-cleanup-0295accc-9a90-4843-909e-f5be00394133`;
  - cleanup pod log reported `artifact cleanup started` and
    `artifact cleanup completed`;
  - delete-triggered GC request was created:
    `dmcr-gc-0295accc-9a90-4843-909e-f5be00394133`;
  - the request carried:
    - `ai.deckhouse.io/dmcr-gc-direct-upload-mode=immediate-orphan-cleanup`;
    - `ai.deckhouse.io/dmcr-gc-requested-at=2026-04-23T19:19:46.551524254Z`;
    - `ai.deckhouse.io/dmcr-gc-switch=2026-04-23T19:19:46.551524254Z`.
- `dmcr` entered and then left maintenance mode for that request:
  - maintenance ReplicaSet appeared as `dmcr-68ff4cb455-*`;
  - normal ReplicaSet returned as `dmcr-8678549d95-*`;
  - the request secret disappeared after the active GC cycle.
- post-smoke `dmcr-cleaner gc check` result was clean:
  - `Live repository prefixes: 0`;
  - `Stored repository prefixes: 0`;
  - `Stored direct-upload object prefixes: 0`;
  - `Open direct-upload multipart uploads: 0`;
  - `Open direct-upload multipart parts: 0`;
  - `Stale orphan direct-upload object prefixes: 0`;
  - `Stale orphan direct-upload multipart uploads: 0`;
  - `No stale prefixes eligible for cleanup.`
- conclusion:
  - fresh publish/delete/GC path works in the current live build;
  - the earlier S3 residue was historical/orphaned state that only the global
    stale sweep could discover;
  - the remaining product gap is startup/backfill behavior for scheduled stale
    sweep after rollout, not a fresh delete-controller failure.

## 15. Live log pass on 2026-04-24

### 15.1 ai-models pod status

- current context: `k8s-main`;
- namespace `d8-ai-models` contains two healthy controller pods and two `dmcr`
  pods;
- only module-local restart signal is in one `dmcr-garbage-collection`
  container:
  - `dmcr-76688666c-ml6d9`;
  - `dmcr-garbage-collection` restart count: `7`;
  - last failed run in previous logs: `2026-04-24T14:00:00Z`.
- there are no current `Model` or `ClusterModel` resources.

### 15.2 DMCR garbage-collection RBAC failure

- previous `dmcr-garbage-collection` log:
  - `dmcr-cleaner exited with error`;
  - `User "system:serviceaccount:d8-ai-models:dmcr" cannot create resource
    "secrets" in namespace "d8-ai-models"`.
- live authz confirmed the mismatch:
  - `create secrets` -> `no`;
  - `get/list/update/patch/delete secrets` -> `yes`.
- fix in repository:
  - `templates/dmcr/rbac.yaml` now grants `create` on core `secrets` to the
    internal `dmcr` service-account Role.
- live direct patch was rejected by Deckhouse admission because
  `heritage: deckhouse` objects cannot be changed directly.
- current cluster therefore remains unfixed until the rendered module is
  deployed through the module pipeline.

### 15.3 Direct upload TLS handshake noise

- `dmcr-direct-upload` produced 120 `TLS handshake error ... EOF` records in
  10 minutes.
- source `10.111.0.116` is inside `k8s-m1.apiac.ru` pod CIDR and is not a
  current pod endpoint, which is consistent with node/probe traffic after CNI
  translation.
- root cause in templates/code:
  - direct-upload listens with TLS on `https-upload`;
  - probes used raw `tcpSocket`;
  - the Go TLS server logs each TCP connect/close as a handshake EOF.
- fix in repository:
  - add unauthenticated `GET/HEAD /healthz` to direct-upload;
  - switch readiness/liveness probes to HTTPS GET `/healthz`.

### 15.4 S3 RequestTimeTooSkewed

- both `dmcr` pods log repeated storage-driver health errors:
  - `s3aws: RequestTimeTooSkewed`;
  - HTTP status `403`;
  - interval roughly 30 seconds in the sampled window.
- local UTC time and public S3 `Date` header matched on `2026-04-24`;
- repeated local curl requests to `https://s3.api.apiac.ru` returned current
  `Date` headers and current request-id timestamp segments;
- request-id timestamp segments from DMCR errors decode to `2025-05-29`;
- this points to an S3/RGW/backend route or clock problem for requests from the
  cluster, not to human RBAC and not to the `dmcr` service-account Secret verb.

### 15.5 Other cluster-wide noise

- unrelated current cluster problems:
  - nodes `k8s-w2-gpu.apiac.ru` and `k8s-w3.apiac.ru` report
    `NodeStatusUnknown` / `Kubelet stopped posting node status`;
  - `d8-n8n` pods are in `ImagePullBackOff`;
  - `d8-monitoring` has PVC mount/readiness issues;
  - `kuberay-projects/open-webui-web-ui-0` is in `CrashLoopBackOff`.
- these were not changed in this module slice.

### 15.6 Validation

- `cd images/dmcr && go test -count=1 ./internal/directupload`;
- `make helm-template`;
- `make kubeconform`;
- rendered checks across six Helm scenarios:
  - DMCR Role has `get,list,create,update,patch,delete` on internal Secrets;
  - direct-upload readiness/liveness probes are HTTPS `/healthz`;
  - human-facing `ai-models:*` roles do not grant `secrets`.

## 16. ARC runner CephFS mount recovery on 2026-04-24

- failing object:
  - `arc-runners/ai-models-runners-f2cpk-runner-z8z5j`;
  - shared PVC `arc-runners/ai-models-runners-cache`;
  - PV `pvc-a3e24a47-3a32-4b3d-aa7c-d3fcc8ff8067`;
  - storage class `ceph-fs-nvme-sc`;
  - CSI driver `cephfs.csi.ceph.com`.
- failure signal:
  - `MountVolume.MountDevice failed`;
  - `rpc error: code = Aborted`;
  - `an operation with the given Volume ID ... already exists`.
- root cause observed in CSI logs:
  - CephFS node-plugin had a stuck `NodeStageVolume` call for the same volume
    for more than 42 minutes.
- recovery actions:
  - temporarily patched
    `arc-runners/AutoscalingRunnerSet/ai-models-runners` to `maxRunners=0`;
  - pending ephemeral runners disappeared and runner churn stopped;
  - restarted CephFS CSI node-plugin pods on affected nodes `k8s-w2` and
    `k8s-w4`;
  - returned `maxRunners=2`.
- validation:
  - temporary probe pods mounted `ai-models-runners-cache` successfully on
    `k8s-w2` and `k8s-w4`;
  - probes reached `Running` and were deleted afterwards;
  - ARC created fresh runner pods after re-enable;
  - runner on `k8s-w4` reached `2/2 Running`;
  - runner on `k8s-w3` started normally and did not hit `FailedMount`.
- remaining unrelated storage noise:
  - separate RBD mount issues remain visible for monitoring/trivy PVCs and are
    not part of the ARC CephFS runner recovery.

## 17. Fresh tiny model publication smoke on 2026-04-24

### 17.1 Preflight after module update

- `d8-ai-models` live state:
  - `ai-models-controller`: 2 pods, `3/3 Running`, zero restarts;
  - `dmcr`: 2 pods, `4/4 Running`, zero restarts;
  - rendered direct-upload probes are HTTPS `GET /healthz`;
  - `system:serviceaccount:d8-ai-models:dmcr` can create internal Secrets.
- no current `Model` or `ClusterModel` resources existed before the smoke;
- no `ai.deckhouse.io/dmcr-gc-request=true` Secrets existed before the smoke;
- direct-upload TLS EOF probe noise was not reproduced.

### 17.2 Model publication

- created namespaced smoke model:
  - namespace: `ai-models-smoke`;
  - resource: `models.ai.deckhouse.io/v1alpha1`;
  - name: `tiny-random-phi-registry-20260424152425`;
  - source: `https://huggingface.co/hf-internal-testing/tiny-random-PhiForCausalLM`.
- controller events:
  - `15:24:26Z` `RemoteFetchStarted`;
  - `15:24:42Z` `PublicationSucceeded`.
- publication worker:
  - pod `d8-ai-models/ai-model-publish-53f5393e-994a-42b9-adfc-0f0044c1673a`
    was scheduled, pulled the module image, started and exited before log
    collection;
  - the stage is confirmed by model status, events, controller logs and DMCR
    registry readback.
- final status:
  - `phase: Ready`;
  - `artifact.digest:
    sha256:430809d43231b77dd8b4e64ec098976d77b6081838f23dd2328adbc36fbd6526`;
  - `artifact.sizeBytes: 382119`;
  - `artifact.uri:
    dmcr.d8-ai-models.svc.cluster.local/ai-models/catalog/namespaced/ai-models-smoke/tiny-random-phi-registry-20260424152425/53f5393e-994a-42b9-adfc-0f0044c1673a@sha256:430809d43231b77dd8b4e64ec098976d77b6081838f23dd2328adbc36fbd6526`;
  - resolved model family: `phi`;
  - resolved format: `Safetensors`;
  - resolved architecture: `PhiForCausalLM`;
  - all observed public conditions were `True`.

### 17.3 Registry readback

- independent in-cluster registry check used DMCR read credentials and module
  CA without printing secret values;
- manifest request returned `HTTP 200` and `734` bytes;
- first blob request returned `HTTP 200` and `231` bytes;
- materializer later read the manifest/config/layers from DMCR successfully;
- DMCR logs include one expected `HEAD blob unknown` during publication blob
  existence probing, followed by successful `GET` requests;
- no `RequestTimeTooSkewed`, `SlowDown`, `timeout`, `panic`, `500` or `503`
  entries were observed in sampled `dmcr`, `dmcr-direct-upload` or
  `dmcr-garbage-collection` logs during the smoke window.

### 17.4 Workload delivery

- created smoke consumer deployment:
  - namespace: `ai-models-smoke`;
  - name: `tiny-random-phi-registry-20260424152425-consumer`;
  - annotation: `ai.deckhouse.io/model:
    tiny-random-phi-registry-20260424152425`;
  - writable cache mount: `/data/modelcache`.
- controller applied runtime delivery:
  - mode: `MaterializeBridge`;
  - reason: `WorkloadCacheVolume`;
  - digest:
    `sha256:430809d43231b77dd8b4e64ec098976d77b6081838f23dd2328adbc36fbd6526`;
  - env injected into workload:
    `AI_MODELS_MODEL_PATH`, `AI_MODELS_MODEL_DIGEST`,
    `AI_MODELS_MODEL_FAMILY`.
- rollout result:
  - deployment reached `1/1` available;
  - active pod
    `tiny-random-phi-registry-20260424152425-consumer-65f68cccb98pjw`
    is `Running` on `k8s-w3.apiac.ru`;
  - materializer completed in about `697ms` from start to ready marker.
- materialized files under `/data/modelcache/model`:
  - `config.json`;
  - `generation_config.json`;
  - `merges.txt`;
  - `model.safetensors`;
  - `special_tokens_map.json`;
  - `tokenizer.json`;
  - `tokenizer_config.json`;
  - `vocab.json`.
- marker file records:
  - media type `application/vnd.cncf.model.manifest.v1+json`;
  - family `phi`;
  - ready timestamp `2026-04-24T15:30:07Z`.

### 17.5 Residual observations

- legacy API group `models.ai-models.deckhouse.io` is still installed but was
  empty for this smoke; the new smoke object exists only under
  `models.ai.deckhouse.io`.
- controller-side delivery is eventual:
  - the initial deployment revision briefly created an unmaterialized pod and
    emitted a stale `BackOff` event;
  - the controller patched the workload and the final revision is healthy.
- no GC request Secrets were created by this positive smoke.
- smoke resources were left in place intentionally for follow-up inspection.
