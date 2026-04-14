---
title: "Configuration"
menuTitle: "Configuration"
weight: 60
---

<!-- SCHEMA -->

The current `ai-models` configuration contract is intentionally short.
Only stable ai-models-specific settings are exposed at the module level:
logging, Deckhouse SSO settings, PostgreSQL wiring, and the shared
S3-compatible storage used by the backend, raw ingest, and the internal
phase-2 publication backend.

`postgresql.mode` supports two phase-1 paths:

- `Managed`: create an internal PostgreSQL through Deckhouse `managed-postgres`;
- `External`: connect ai-models to an existing PostgreSQL instance using a password
  from an existing Secret.

The default managed baseline is intentionally small: it reuses an existing
cluster-wide `PostgresClass`, requests a 5Gi volume, and keeps the rest of the
resource profile minimal for phase-1 metadata storage.
The default database and user names are `ai-models`, and HA topology for the
managed `Postgres` follows the selected `PostgresClass.defaultTopology` instead
of hardcoding a module-specific value.
For browser SSO and MLflow permissions, ai-models also uses a separate logical
auth database in the same PostgreSQL instance. In `Managed` mode the module
creates that second database automatically using the `<database>-auth` naming
pattern. In `External` mode the existing PostgreSQL instance must already
provide that second database.

`artifacts` config defines the S3-compatible backend for ai-models artifacts:
bucket, path prefix, endpoint URL, region, TLS policy, addressing style, and
credentials.

Artifact credentials are provided only through `credentialsSecretName`.
That Secret must live in `d8-system` and expose fixed `accessKey` and
`secretKey` keys. Inline S3 credentials are no longer supported. The module
copies only these keys into its own `d8-ai-models` namespace before rendering
workloads, so users do not manage storage credentials directly in the service
namespace.

Custom S3 CAs are configured separately through `artifacts.caSecretName`.
That Secret must live in `d8-system` and expose `ca.crt`. The module copies
that CA into `d8-ai-models` when needed. When
`caSecretName` is empty, ai-models first reuses `credentialsSecretName` if the
same Secret also contains `ca.crt`, and otherwise falls back to the shared
platform CA that is already discovered for Dex or copied from the global
`CustomCertificate` HTTPS path.

`bucket`, `pathPrefix`, `endpoint`, `region`, and addressing/TLS flags are not
treated as secrets. They remain part of the normal module configuration surface.

The internal DMCR publication backend now always uses the same S3-compatible
storage contract from `artifacts`. There is no separate `publicationStorage`
user-facing block and no PVC branch for published model bytes.

Within the shared bucket, ai-models keeps the byte-path split explicit:
- the MLflow backend uses the configured `artifacts.pathPrefix`;
- controller-owned raw ingest uses the fixed `raw/` subtree;
- the internal DMCR publication backend uses the fixed `dmcr/` subtree;
- future append-only audit/provenance data may live under `audit/`.

The controller always publishes to the same internal registry service rendered
by the module. There is no longer a separate external `publicationRegistry`
user-facing endpoint/credentials contract.

High availability mode and HTTPS policy are taken from global Deckhouse
configuration and internal module wiring. The current runtime expects:

- `global.modules.publicDomainTemplate` to be configured;
- HTTPS to be enabled globally through Deckhouse module HTTPS policy
  (`CertManager` or `CustomCertificate`);
- the `managed-postgres` module when `postgresql.mode=Managed`.

Browser login goes through Deckhouse Dex OIDC SSO inside MLflow.
The module automatically wires:

- a `DexClient` in `d8-ai-models` with redirect URI `https://<public-host>/callback`;
- the public Dex discovery URL `https://dex.<cluster-domain>/.well-known/openid-configuration`;
- automatic platform CA trust wiring from Dex discovery or the global HTTPS
  `CustomCertificate` path for MLflow OIDC and S3 TLS;
- MLflow OIDC login through `mlflow-oidc-auth`;
- upstream-native MLflow workspaces.

The `auth.sso.allowedGroups` and `auth.sso.adminGroups` settings define which
Deckhouse groups are allowed to enter ai-models and which of them become MLflow
administrators after SSO login. The default is intentionally conservative:
only the Deckhouse `admins` group is allowed and promoted to MLflow admins.

The module always renders an internal auth Secret with:

- the internal machine username in `machineUsername`;
- a stable generated machine password in `machinePassword`;
- a stable session secret used by MLflow auth runtimes.

This Secret is now machine-only for `ServiceMonitor`, in-cluster import Jobs,
and break-glass operations while browser users go through Dex SSO.

The backend service is therefore no longer protected only at ingress level.
Direct access to the raw backend still requires MLflow machine credentials, and
logical segmentation continues to happen through native MLflow workspaces.

Large machine-oriented imports use direct artifact access instead of server-side
artifact proxying. The backend runs with `--no-serve-artifacts`, and in-cluster
import Jobs authenticate to MLflow metadata APIs while uploading artifacts
directly to S3. The backend and import Jobs reuse the same merged trust bundle
for Dex OIDC and S3 CA overrides, so `artifacts.insecure: true` is only a
temporary troubleshooting path and not the intended steady-state mode.

For phase 2, the controller now owns publication/runtime adapters for its
first live source paths. `Model` / `ClusterModel` with `spec.source.url` are
reconciled through controller-owned worker Pods that accept only Hugging Face
URLs, resolve the exact upstream revision, download the selected files from the
snapshot, generate a model-package description, package the checkpoint into a
`ModelPack` with the current implementation adapter, push the resulting
artifact into the internal module-owned DMCR-style OCI publication plane,
inspect the remote manifest, and then project the saved artifact locator and
enriched technical profile back into object `status`. For `HuggingFace`, the
controller accepts source secrets with one of `token`, `HF_TOKEN`, or
`HUGGING_FACE_HUB_TOKEN` and normalizes them into a projected worker token.
The controller-owned publication worker hardens tar/zip extraction and rejects
path traversal, symlink, hard link, and other special archive entries instead
of relying on raw `extractall`.

`spec.source.upload` now follows a controller-owned session flow rather
than a batch import. The controller creates one short-lived session Secret per
upload and projects shared upload-gateway URLs in `status.upload`:
`inClusterURL` always, `externalURL` when public ingress is enabled. The
gateway footprint is now shared:

- one controller Deployment with the upload-gateway sidecar;
- one shared Service;
- zero or one shared external Ingress.

The current live controller path accepts
the following uploads:

- for `Safetensors`: an archive with `config.json` and model weight files;
- for `GGUF`: either a direct `.gguf` file or an archive.

The controller then publishes them into the same controller-owned
`ModelPack`/OCI artifact plane through the current Go dataplane and
`ModelPack` adapter. The live upload path is now two-phase and staging-first:
the shared upload gateway no longer receives the final model bytes into the
cluster Pod path. Instead, it owns only the session/control API behind the URL
projected in `status.upload`, starts multipart upload in module-owned object
storage staging, signs per-part URLs, and completes the upload after
client-side multipart completion. The controller then observes the staged
result from the session Secret, requeues the object, and starts a separate
publish worker that downloads the staged object, validates/profiles it,
publishes the final `ModelPack` into `DMCR`, and cleans the staging object on
success. `status.upload` no longer exposes a helper command; the live
public contract is limited to `expiresAt`, `repository`, `inClusterURL`, and
optional `externalURL`.

On top of the source contract, `spec` now also carries a live policy layer:

- `spec.modelType`:
  a coarse platform classification (`LLM`, `Embeddings`, `Reranker`,
  `SpeechToText`, `Translation`).
  The field is immutable and is now validated against the resolved profile;
- `spec.usagePolicy.allowedEndpointTypes`:
  a whitelist of allowed platform-facing endpoint categories.
  When set, the controller requires a non-empty intersection with the resolved
  supported endpoint types;
- `spec.launchPolicy`:
  a live whitelist for runtime, accelerator vendor, and precision.
  `preferredRuntime` must be included in `allowedRuntimes` when both are set,
  and the controller no longer marks the object as validated when the
  calculated profile has no intersection with the declared launch policy;
- `spec.optimization.speculativeDecoding.draftModelRefs`:
  this is not consumer runtime magic yet, but it is now a live
  publication-time contract.
  The controller currently allows it only for generative `LLM` profiles and
  accounts for it in `Validated` / `Ready`.

`spec.inputFormat` is treated as the source-agnostic validation contract for
the uploaded or downloaded model project, not as the final registry artifact
format. The final published form stays internal and fixed:
`ModelPack` in OCI. Regardless of whether bytes came from Hugging Face or
local upload, the controller now validates and sanitizes the project
composition before packaging. The current live rules are:

- `Safetensors`: requires a root `config.json`, at least one `.safetensors`
  file, allows known config/tokenizer/index companions, strips benign extras
  such as `README.md` and images, and rejects active or ambiguous files such
  as `.py`, `.sh`, `.dll`, `.so`, or other unsupported payloads.
- `GGUF`: requires at least one `.gguf` file, strips benign extras, and rejects
  the same active or ambiguous payloads.

If `spec.inputFormat` is omitted, the controller tries to determine it
automatically:

- `GGUF` from one or more `.gguf` files;
- `Safetensors` from a root `config.json` plus `.safetensors`.

If the result is not unique, publication fails closed and requires an explicit
`spec.inputFormat`.

After validation, the controller also enriches the metamodel:

- for `Safetensors`
  - reads `config.json`
  - resolves the context window from known config keys
  - calculates `parameterCount` first from explicit config fields, then from
    the real sizes of `.safetensors` shard files
  - derives `quantization` and `compatiblePrecisions`
  - builds `supportedEndpointTypes` from `task`
  - builds `minimumLaunch` as a GPU baseline from the real weight footprint
- for `GGUF`
  - reads the `.gguf` file name and size
  - derives family, quantization, and an approximate `parameterCount`
  - builds `supportedEndpointTypes` from `task`
  - builds `minimumLaunch` as a GPU baseline from the real file size and
    quantization

`Validated` and the final `Ready` condition are no longer a formal
"publication succeeded" marker. After publication, the controller separately
matches the public policy from `spec` against the calculated profile. If the
profile is available but the policy contradicts it, the published artifact
still stays in `status.artifact`, `MetadataReady=True`, and the object moves to
`Failed` with `Validated=False` and a concrete reason such as
`ModelTypeMismatch`, `EndpointTypeNotSupported`, `RuntimeNotSupported`,
`AcceleratorPolicyConflict`, or `OptimizationNotSupported`.

Destructive cleanup also stays explicit and machine-oriented. The phase-2
controller now persists only an internal backend cleanup handle and runs
controller-owned one-shot Jobs through the dedicated runtime-image
`artifact-cleanup` subcommand. The current live cleanup path logs into the
internal module-owned DMCR-style registry service with the same
controller-owned trust and credential wiring, removes the remote artifact by
its saved reference, then creates an internal DMCR garbage-collection request.
The module-owned `dmcr-cleaner` sidecar switches the registry into
maintenance/read-only mode, runs physical blob garbage collection, and only
then lets the controller remove the finalizer, while keeping backend internals
out of public status.

Runtime delivery no longer stops at a standalone materializer binary. The
controller runtime now also carries a reusable K8s-side delivery adapter over
that binary:

- `materialize-artifact` now supports a cache-root contract and keeps
  `store/<digest>/model` plus `/data/modelcache/current`;
- `k8s/modeldelivery` now owns concrete consumer-side `PodTemplateSpec`
  mutation over that init container and reuses an already mounted
  `/data/modelcache` volume from the workload instead of inventing a second
  volume contract;
- runtime delivery now validates storage topology before mutation:
  per-pod mounts and StatefulSet claim templates are accepted, a direct shared
  PVC on multi-replica workloads now requires `ReadWriteMany`, and shared RWX
  caches coordinate a single writer directly on the shared cache root;
- the controller now adopts annotated `Deployment`, `StatefulSet`,
  `DaemonSet`, and `CronJob` workloads directly:
  `ai-models.deckhouse.io/model=<name>` for namespaced `Model` and
  `ai-models.deckhouse.io/clustermodel=<name>` for `ClusterModel`;
- the workload must already mount writable storage at `/data/modelcache`;
  ai-models no longer invents a second delivery volume and does not mutate
  direct `Job` objects through this controller-owned patch path;
- generic runtime delivery stays controller-driven and opt-in instead of
  adding blocking mutating or validating admission hooks for foreign workload
  kinds; the controller now watches only opt-in or already-managed workloads
  plus referenced `Model` / `ClusterModel` objects;
- runtime containers are not patched with model env vars; runtimes read the
  stable local path `/data/modelcache/current` explicitly in their own config;
- OCI auth/CA reuse the existing controller-owned projected DMCR access,
  including cross-namespace projection into the runtime namespace, instead of
  inventing a second delivery-specific secret model.

This still does not hard-wire `ai-models` into a concrete `ai-inference`
module inside this repo. The landed seam is now a concrete consumer-side K8s
service that another module can call without reimplementing materialization or
registry-access projection.

The HF import path now also leaves production-grade metadata in MLflow:

- the source run gets HF-related params and tags;
- the registered model and model version get descriptions and tags;
- run artifacts include `hf/model-info.json`, `hf/snapshot-manifest.json`, and,
  when available, `config.json`, `generation_config.json`, `tokenizer_config.json`,
  and `model-card.md`.

This does not turn the MLflow UI into a raw S3 browser: the UI still shows only
the metadata and artifacts that the importer logs explicitly. For multimodal
task types, the schema section in the UI still depends on upstream
`mlflow.transformers` support and may remain empty without a task-specific
signature.

The current phase-1 backend runtime profile is intentionally conservative:
each backend pod runs a single MLflow web worker, and MLflow server job
execution is disabled. High availability for the backend is achieved through
Deckhouse module HA and pod replicas, not through extra in-process workers or
genai job consumers.

The backend also keeps MLflow security middleware enabled. The module derives
MLflow `allowed-hosts` and same-origin CORS settings from the public ingress
domain and preserves the private-network/service access patterns needed for
in-cluster access. Health probes use the upstream unauthenticated `/health`
endpoint, and `ServiceMonitor` scrapes `/metrics` via the internal machine
account.

The module also provisions an internal Secret with a stable
`MLFLOW_CRYPTO_KEK_PASSPHRASE` value for upstream MLflow crypto-backed runtime
features. This removes the insecure upstream default passphrase from shared
cluster deployments without exposing the KEK as a user-facing module setting.

`Model` and `ClusterModel` are now part of the module installation lifecycle as
CRDs and controller runtime wiring. Their publication UX and final public
contract are still under active phase-2 work, so the current API should be
treated as evolving rather than stable.
