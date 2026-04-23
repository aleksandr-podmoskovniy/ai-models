---
title: "Configuration"
menuTitle: "Configuration"
weight: 60
---

<!-- SCHEMA -->

The current `ai-models` configuration contract is intentionally short.
Only stable module-level settings are exposed:

- `logLevel`;
- `artifacts`.
- `dmcr`.
- `nodeCache`.

High availability mode, HTTPS policy, ingress class, controller/runtime wiring,
`DMCR`, upload-gateway, and publication worker internals stay in global
Deckhouse settings plus internal module values. There is no longer a user-facing
module contract for:

- retired backend auth/workspace and metadata-database knobs;
- browser SSO knobs;
- backend-only secrets;
- external publication registry settings;
- backend-specific `artifacts.pathPrefix`.

`artifacts` defines the shared S3-compatible storage for ai-models byte paths.
The split inside the bucket is fixed by runtime code:

- `raw/` for controller-owned upload staging and, when
  `artifacts.sourceFetchMode=Mirror`, the temporary source mirror;
- `dmcr/` for published OCI artifacts in the internal `DMCR`;
- optional future append-only module data under separate fixed prefixes.

Artifact credentials are provided only through `credentialsSecretName`. The
Secret must live in `d8-system` and expose fixed `accessKey` and `secretKey`
keys. The module copies only these keys into its own namespace before rendering
runtime workloads, so users do not manage storage credentials directly in
`d8-ai-models`.

Custom S3 trust is configured through `artifacts.caSecretName`. That Secret
must live in `d8-system` and expose `ca.crt`. When `caSecretName` is empty,
ai-models first reuses `credentialsSecretName` if that Secret also contains
`ca.crt`, and otherwise falls back to the platform CA already discovered by the
module runtime or copied from the global HTTPS `CustomCertificate` path.

`dmcr.gc.schedule` exposes the productized stale-sweep cadence for the internal
registry. By default, ai-models enqueues one stale cleanup cycle daily at
02:00. Setting the schedule to an empty string disables the periodic sweep
without removing the operator-facing inspection surface: `dmcr-cleaner gc check`
still reports stale published repository prefixes and source-mirror prefixes
inside the DMCR Pod.

The public runtime path for models is now controller-owned:

- `Model` / `ClusterModel` use one cluster-level
  `artifacts.sourceFetchMode`:
  - `Mirror`:
    remote `source.url` first goes through a controller-owned source mirror;
  - `Direct`:
    remote `source.url` is consumed directly from the canonical remote source
    boundary;
- `spec.source.upload` uses the controller-owned upload-session path and stays
  on its own staged object boundary;
- all paths publish OCI `ModelPack` artifacts into the internal `DMCR`;
- streamable multi-file remote inputs publish as one bounded companion bundle
  plus dedicated raw layers for large model payloads, instead of repacking the
  whole model into one monolithic tar layer;
- single-file direct or staged-object inputs still publish as one raw layer;
- archive inputs stay on the archive-source streaming path and do not create an
  extracted success-only checkpoint tree.

The default is `artifacts.sourceFetchMode=Direct`.

The trade-off is explicit:

- `Mirror` keeps a durable intermediate copy in object storage and makes
  re-publish/resume on the remote ingest boundary more predictable;
- `Direct` removes that extra full copy and speeds up the first remote import;
- for `spec.source.upload`, the effective source boundary is already the staged
  object, so the mode does not create a second intermediate copy on top of the
  upload staging contract.

There is no separate transport choice for published layer payloads. The
canonical byte path is fixed:

- `publish-worker -> DMCR direct-upload v2 session -> physical multipart object -> DMCR verification read -> canonical digest metadata/link`.

`DMCR` still owns authentication, final blob/link materialization, and the
published artifact contract, but the thick byte stream no longer goes through
the registry `PATCH` path. This removes `DMCR` itself as the network bottleneck
on the large-byte publish path.

The direct helper runs in late-digest mode: the controller starts the session
without the final layer digest, streams the payload parts, and seals the layer
with `complete(session, expectedDigest, size, parts)`. For range-capable raw
layers this removes the old controller-side full descriptor pre-read: the
publish worker reads the remote model source once. After multipart completion
`DMCR` makes one verification pass over the assembled physical object in
object storage, computes the final `sha256` and actual size itself, and uses
the controller-provided `expectedDigest` only as an additional equality check.
If the digest does not match, publication is rejected and the physical upload
object is deleted.
If the verification read fails transiently after multipart assembly, the
physical object is retained: a repeated `complete` call can continue
verification of the already assembled object without uploading the model bytes
again.

Small `config`/manifest writes and final remote inspect still use the normal
registry API so the internal contract changes one responsibility at a time.

The current helper implementation still has one internal seal step, but it no
longer rewrites the full heavy object during sealing. The multipart upload
first lands into a physical object under
`_ai_models/direct-upload/objects/<session-id>/data`, then `DMCR` reads that
object once for verification, writes a tiny `.dmcr-sealed` sidecar near the
canonical digest-addressed blob path, and creates the repository link using the
computed digest/size. The published OCI contract remains digest-based:
repository links still point to the canonical digest, while the internal
`sealeds3` driver resolves that digest to the physical object key.

On the controller side, direct-upload now also keeps one compact owner-scoped
checkpoint `Secret`. During `Running`, that state stores the current layer key,
session token, part size, uploaded bytes, digest continuation state, and
already committed layers. If the `sourceworker` Pod dies while that state still
indicates `Running`, controller recreates the Pod and `publish-worker` can
continue from the saved checkpoint plus `listParts()` while the helper session
is still alive, instead of depending only on one live worker process. Public
running status now exposes both bounded progress and machine-readable running
reasons through status conditions: `PublicationStarted`,
`PublicationUploading`, `PublicationResumed`, `PublicationSealing`, and
`PublicationCommitted`. The condition message still carries current-layer
uploaded bytes where available, otherwise the number of already committed
layers.

For streamable multi-file sources the internal OCI layout is now mixed:
small companion files may be packed into one bounded tar layer under the stable
`model/` root, while large model payload files stay dedicated raw published
layers. This is an internal publisher/materializer decision only. The
consumer-facing materialized contract stays stable: multi-file models still
resolve under the stable `model/` root, while a single-file direct input keeps
its single-file entrypoint.

The successful publication worker path no longer uses a local workspace/PVC.
`HuggingFace` in both modes and staged upload publish through
raw object-source or archive-source streaming semantics. The only local bounded
storage contract left for the publish worker is the container
`ephemeral-storage` request/limit for logs and writable layer usage.

The public model API is also intentionally minimal. Users specify only
`spec.source`; format, task, and other model metadata are calculated by the
controller from the actual model contents and projected into `status.resolved`.

`nodeCache` is the first landed slice of the node-local cache workstream. In
the current state it owns the managed local-storage substrate plus the current
local materialize-bridge volume contract:

- ai-models can keep one managed `LVMVolumeGroupSet` over
  `sds-node-configurator`;
- ai-models can keep one managed `LocalStorageClass` built from the currently
  ready managed `LVMVolumeGroup` objects;
- enabling this slice removes the need to create that `LocalStorageClass`
  manually.

The current bounded contract is:

- `nodeCache.enabled` enables the managed substrate controller;
- `nodeCache.maxSize` becomes the per-node thin-pool budget;
- `nodeCache.fallbackVolumeSize` defines the managed local ephemeral volume
  size that the current workload delivery path auto-injects at
  `/data/modelcache` for the transition materialize-bridge path when an
  annotated workload does not bring its own cache volume;
- `nodeCache.sharedVolumeSize` defines the per-node shared cache volume
  requested by the controller-owned stable runtime Pod/PVC over the managed
  `LocalStorageClass`;
- `nodeCache.storageClassName`, `nodeCache.volumeGroupSetName`,
  `nodeCache.volumeGroupNameOnNode`, and `nodeCache.thinPoolName` define the
  ai-models-owned substrate object names;
- `nodeCache.nodeSelector` and `nodeCache.blockDeviceSelector` are `matchLabels`
  maps used to select substrate nodes and block devices.

This slice still does not replace the live workload delivery path with a
workload-facing node-shared mount service. Workloads still materialize through
controller-owned `materialize-artifact` into `/data/modelcache`, but now:

- the current materialize-bridge path can auto-inject a local generic
  ephemeral volume over the managed `LocalStorageClass` when the workload does
  not bring its own cache topology;
- managed workloads now also get controller-projected registry access for the
  bridge runtime itself: the init-container image pull secret is copied into
  the workload namespace together with the OCI read auth/CA supplements, so the
  materialize bridge no longer depends on a manually pre-created secret next to
  every consumer workload;
- workloads now get one stable runtime-facing model delivery contract via
  `AI_MODELS_MODEL_PATH`, `AI_MODELS_MODEL_DIGEST`, and
  `AI_MODELS_MODEL_FAMILY`; per-pod delivery still projects the stable
  `/data/modelcache/model` entrypoint, while shared PVC bridge topology now
  projects a digest-scoped path inside the shared store instead of relying on a
  global cache-root `current` link;
- the controller now also writes managed `PodTemplateSpec` annotations with the
  selected delivery mode and the reason that mode was chosen, so the
  materialize bridge no longer remains hidden runtime behavior;
- `runtimehealth` metrics now aggregate managed top-level workloads by
  namespace, kind, delivery mode, and delivery reason, so operators can see
  where workloads still use the transition materialize bridge and where they
  already use the shared PVC bridge without scraping ad-hoc object lists;
- `runtimehealth` now also exposes managed workload Pod counts, ready counts,
  and waiting reasons for the `materialize-artifact` init container, so
  `ImagePullBackOff` and similar bridge failures become machine-visible without
  digging through Pod events by hand;
- ai-models now keeps a separate per-node shared cache plane as a
  controller-owned stable runtime Pod plus stable PVC over the managed
  `LocalStorageClass`; the shared volume size is controlled by
  `nodeCache.sharedVolumeSize`, and storage identity no longer depends on the
  node-agent pod lifecycle;
- `node-cache-runtime` derives the set of published artifacts required by live
  managed Pods scheduled on the current node only for the future true
  shared-direct mode; the current shared PVC bridge does not consume that
  per-node plane yet, so node-local prefetch is still preparatory rather than
  workload-facing delivery.

There is still no public cleanup or TTL knob yet: the workload-facing shared
mount contract has not landed, so eviction policy remains internal runtime
behavior rather than a promised user-facing SLA.
