---
title: "Configuration"
menuTitle: "Configuration"
weight: 60
---

<!-- SCHEMA -->

The current `ai-models` configuration contract is intentionally short.
Only stable module-level settings are exposed:

- `logLevel`;
- `artifacts`;
- `nodeCache`.

High availability mode, HTTPS policy, ingress class, controller/runtime wiring,
`DMCR`, upload-gateway, publication worker internals, source-fetch policy and
GC cadence stay in global Deckhouse settings plus internal module values. There
is no user-facing module contract for:

- retired backend auth/workspace and metadata-database knobs;
- browser SSO knobs;
- backend-only secrets;
- external publication registry settings;
- backend-specific `artifacts.pathPrefix`.
- `DMCR` implementation settings;
- source-fetch transport selection;
- internal node-cache object names.

`artifacts` defines the shared S3-compatible storage for ai-models byte paths.
The split inside the bucket is fixed by runtime code:

- `raw/` for controller-owned upload staging and temporary source objects when
  the module needs them;
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

The public runtime path for models is now controller-owned:

- `Model` / `ClusterModel` remote `source.url` ingestion uses a module-owned
  policy. The current default is direct streaming from the canonical source
  boundary; the module may use temporary source objects internally when a
  source adapter requires that for resumability or safety;
- `spec.source.upload` uses the controller-owned upload-session path and stays
  on its own staged object boundary; time-bounded secret upload URLs are
  projected in status, matching the direct upload UX used by virtualization;
- all paths publish OCI `ModelPack` artifacts into the internal `DMCR`;
- streamable multi-file remote inputs publish as one bounded companion bundle
  plus dedicated raw layers for large model payloads, instead of repacking the
  whole model into one monolithic tar layer;
- single-file direct or staged-object inputs still publish as one raw layer;
- archive inputs stay on the archive-source streaming path and do not create an
  extracted success-only checkpoint tree.

RBAC follows the Deckhouse `user-authz` and `rbacv2` split:

- legacy `user-authz` grants read-only `Model` / `ClusterModel` visibility at
  `User`, write access to namespaced `Model` at `Editor`, and write access to
  cluster-wide `ClusterModel` only at `ClusterEditor`;
- `PrivilegedUser`, `Admin`, and `ClusterAdmin` do not add extra ai-models
  verbs until the module has a safe user-facing resource for those levels;
- `rbacv2/use` is limited to namespaced `Model`, so namespaced RoleBindings do
  not imply access to cluster-scoped `ClusterModel`;
- `rbacv2/manage` is the cluster-persona path for `Model`, `ClusterModel`, and
  the `ai-models` `ModuleConfig`;
- human-facing module roles intentionally do not grant `status`, `finalizers`,
  Secret access, pod logs, exec, attach, port-forward, or internal runtime
  objects.
- internal service-account RBAC is not aggregated into human roles; `DMCR`
  garbage collection reads only module-private cleanup/GC `Secret` objects and
  `Lease` objects in the module namespace, not user-facing `Model` or
  `ClusterModel` objects.

There is no separate transport choice for published layer payloads. The
canonical byte path is fixed:

- `publish-worker -> DMCR direct-upload v2 session -> physical multipart object -> DMCR trusted S3 full-object digest when available, otherwise verification read -> canonical digest metadata/link`.

`DMCR` still owns authentication, final blob/link materialization, and the
published artifact contract, but the thick byte stream no longer goes through
the registry `PATCH` path. This removes `DMCR` itself as the network bottleneck
on the large-byte publish path.

The direct helper runs in late-digest mode: the controller starts the session
without the final layer digest, streams the payload parts, and seals the layer
with `complete(session, expectedDigest, size, parts)`. For range-capable raw
layers this removes the old controller-side full descriptor pre-read: the
publish worker reads the remote model source once. After multipart completion
`DMCR` first asks object storage for a trusted full-object `ChecksumSHA256`.
When it is available, `DMCR` uses that value without a second object read.
`ETag`, multipart part checksums, and composite checksums are not accepted as
safe OCI `sha256` digests. If a trusted full-object checksum is unavailable,
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
`_ai_models/direct-upload/objects/<session-id>/data`, then `DMCR` either uses a
trusted full-object SHA256 from object storage or reads the object once for
verification, writes a tiny `.dmcr-sealed` sidecar near the canonical
digest-addressed blob path, and creates the repository link using the computed
digest/size. The published OCI contract remains digest-based:
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

The internal publication runtime default is sized for the streaming path: up to
`4` worker Pods at once, memory request `1Gi`, memory limit `2Gi` per worker,
CPU request `1`, CPU limit `4`, and ephemeral-storage request/limit `1Gi`.
These values stay internal module values, not public `ModuleConfig`.

The public model API is also intentionally minimal. Users specify only
`spec.source`; format, normalized endpoint/features, and source capability
evidence are calculated by the controller from the actual model contents and
projected into `status.resolved`.

`nodeCache` owns the managed local-storage substrate and the workload-facing
SharedDirect delivery contract:

- ai-models can keep one managed `LVMVolumeGroupSet` over
  `sds-node-configurator`;
- ai-models can keep one managed `LocalStorageClass` built from the currently
  ready managed `LVMVolumeGroup` objects;
- enabling this slice removes the need to create that `LocalStorageClass`
  manually.
- enabling this slice now fails render unless `sds-node-configurator-crd`,
  `sds-node-configurator`, `sds-local-volume-crd`, and `sds-local-volume` are
  present in `global.enabledModules`.

The current bounded contract is:

- `nodeCache.enabled` enables the managed substrate controller;
- `nodeCache.size` is the single per-node cache capacity decision. The module
  uses the same value for the managed thin-pool budget, the per-node shared
  cache PVC and the runtime eviction budget;
- by default ai-models selects nodes and `BlockDevice` objects labelled
  `ai.deckhouse.io/model-cache=true`;
- `nodeCache.nodeSelector` and `nodeCache.blockDeviceSelector` may override
  that default only when the cluster already has a stricter labelling
  convention;
- `LocalStorageClass`, `LVMVolumeGroupSet`, VG and thin-pool names are
  internal ai-models constants, not public ModuleConfig knobs.

With `nodeCache.enabled=true`, workloads that do not bring an explicit cache
volume are cut over to the node-cache SharedDirect path:

- the controller injects an inline CSI volume
  `node-cache.ai-models.deckhouse.io` at `/data/modelcache`;
- the injected CSI volume carries only immutable artifact attributes
  (URI, digest, optional family), not storage-specific public API fields;
- the controller propagates the effective node-cache node selector into managed
  workload pod templates and fails on conflicts instead of scheduling them onto
  unsupported nodes;
- the controller also requires the dynamic
  `ai.deckhouse.io/node-cache-runtime-ready=true` node label and keeps the
  managed scheduling gate while no selected node has a ready runtime Pod and a
  bound shared cache PVC;
- the workload namespace no longer receives projected DMCR read Secret/CA or
  bridge runtime imagePullSecret for the managed SharedDirect path;
- workloads get a stable runtime-facing model delivery contract. Legacy
  `ai.deckhouse.io/model` / `ai.deckhouse.io/clustermodel` annotations still
  expose the primary model through `AI_MODELS_MODEL_PATH`,
  `AI_MODELS_MODEL_DIGEST`, and `AI_MODELS_MODEL_FAMILY`; multi-model
  workloads can use `ai.deckhouse.io/model-refs` with values such as
  `main=Model/gemma,embed=ClusterModel/bge`, which exposes stable alias paths
  under `/data/modelcache/models/<alias>` plus `AI_MODELS_MODELS_DIR`,
  `AI_MODELS_MODELS` with alias/path/digest/family entries, and per-alias
  `AI_MODELS_MODEL_<ALIAS>_{PATH,DIGEST,FAMILY}` variables;
- for KubeRay `RayService`, the same annotations stay on the GitOps-owned
  `RayService`, but the controller does not patch `RayService.spec`: runtime
  wiring is applied to generated `RayCluster` pod templates so ArgoCD does not
  roll managed mutation back to the Git state;
- the controller now also writes managed `PodTemplateSpec` annotations with the
  selected delivery mode and the reason that mode was chosen, so the
  node-cache runtime can discover desired artifacts from live Pods on its node;
- `runtimehealth` metrics now aggregate managed top-level workloads by
  namespace, kind, delivery mode, and delivery reason, so operators can see
  where workloads still use explicit legacy bridge storage and where they use
  SharedDirect without scraping ad-hoc object lists;
- ai-models now keeps a separate per-node shared cache plane as a
  controller-owned stable runtime Pod plus stable PVC over the managed
  `LocalStorageClass`; the shared volume size is controlled by
  `nodeCache.size`, and storage identity no longer depends on the node-agent
  pod lifecycle;
- the cache runtime Pod does not use direct `spec.nodeName`: it is pinned to
  the intended node through `kubernetes.io/hostname` node affinity, so the
  Kubernetes scheduler can correctly choose a local LVM volume for a
  `WaitForFirstConsumer` PVC;
- the controller passes the `node-driver-registrar` image from the Deckhouse
  common CSI image set into the runtime Pod, and the runtime Pod exposes the
  kubelet-facing CSI socket
  `/var/lib/kubelet/csi-plugins/node-cache.ai-models.deckhouse.io/csi.sock`;
- the runtime container uses a dedicated internal `nodeCacheRuntime`
  distroless image instead of the shared publication/materialize runtime image;
- CSI NodePublish performs a read-only bind mount of the ready digest store
  from the per-node shared cache PVC into the kubelet target path; if the
  digest is not materialized yet, CSI returns transient `Unavailable` and
  kubelet retries after prefetch;
- CSI NodePublish fail-closes through `podInfoOnMount`: the mount is allowed
  only for the live Pod on the same node that the controller already marked as
  a managed SharedDirect Pod for the same digest;
- `node-cache-runtime` derives the set of published artifacts required by live
  SharedDirect managed Pods scheduled on the current node and prefetches them
  into the shared node-local digest store;
- transient prefetch/materialization failures are retried per digest with
  in-memory backoff, so one unavailable artifact does not restart the runtime
  Pod or block other digests.

Failure scenarios:

- if SDS modules are not enabled in `global.enabledModules`, render fails
  before module rollout;
- if the SDS CRDs are actually absent from the cluster, the substrate
  controller cannot create `LVMVolumeGroupSet` / `LocalStorageClass`, and the
  node-cache layer will not become ready;
- if a selected node has no local block device matching the effective
  node-cache block-device selector, the managed `LVMVolumeGroup` for that node
  will not become ready, the `LocalStorageClass` will not include that node,
  and the cache runtime PVC/Pod remain unscheduled or pending;
- while no selected node has a ready runtime Pod and a bound shared cache PVC,
  managed SharedDirect workload templates keep the ai-models scheduling gate
  instead of rolling out Pods that would hang on CSI mount.

Explicit workload-provided cache volumes remain a legacy bridge path for now:
they still use `materialize-artifact` and the digest-scoped shared-PVC bridge
logic where applicable. They are not the managed default path after this
cutover.

There is still no public cleanup or TTL knob yet: the workload-facing shared
mount contract now exists, but eviction policy remains internal runtime
behavior rather than a promised user-facing SLA.
