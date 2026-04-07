---
title: "Configuration"
menuTitle: "Configuration"
weight: 60
---

<!-- SCHEMA -->

The current `ai-models` configuration contract is intentionally short.
Only stable ai-models-specific settings are exposed at the module level:
logging, Deckhouse SSO settings, PostgreSQL wiring, S3-compatible artifact
storage, and the phase-2 publication OCI registry wiring.

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

Artifact credentials can be provided in two ways:

- via `credentialsSecretName` that points to an existing Secret in `d8-ai-models`
  with fixed keys `accessKey` and `secretKey`;
- via inline `accessKey` and `secretKey` in ModuleConfig, in which case the
  module renders an internal Secret in `d8-ai-models`.

Custom S3 CAs are configured separately through `artifacts.caSecretName`.
That Secret must live in `d8-ai-models` and expose `ca.crt`. When
`caSecretName` is empty, ai-models first reuses `credentialsSecretName` if the
same Secret also contains `ca.crt`, and otherwise falls back to the shared
platform CA that is already discovered for Dex or copied from the global
`CustomCertificate` HTTPS path.

`bucket`, `pathPrefix`, `endpoint`, `region`, and addressing/TLS flags are not
treated as secrets. They remain part of the normal module configuration surface.

Phase-2 publication uses a separate `publicationRegistry` module config. It
defines the controller-owned OCI repository prefix for published `Model` /
`ClusterModel` artifacts and the credentials used by worker Pods by the
current implementation adapter to log in, push, and inspect remote
`ModelPack` artifacts.

Publication registry credentials can be provided in two ways:

- via `credentialsSecretName` that points to an existing Secret in `d8-ai-models`
  with fixed keys `username` and `password`;
- via inline `username` and `password` in ModuleConfig, in which case the
  module renders an internal Secret in `d8-ai-models`.

`publicationRegistry.caSecretName` can point to a Secret in `d8-ai-models`
containing `ca.crt` for private registry trust. When it is empty, ai-models
falls back only to the shared platform CA that is already discovered for Dex or
copied from the global `CustomCertificate` HTTPS path. `publicationRegistry.insecure`
is supported only as a troubleshooting path for plain-HTTP or broken-TLS lab
registries and is not the intended steady-state mode.

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
reconciled through controller-owned worker Pods that determine whether the URL
targets Hugging Face or a generic HTTP source, download the accepted source,
generate a model-package description, package the checkpoint into a
`ModelPack` with the current implementation adapter, push the resulting
artifact into the module-owned OCI publication plane, inspect the remote
manifest, and then project the saved artifact locator and enriched technical profile
back into object `status`. The current live `HTTP` scope is narrow on purpose:
it expects an archive containing a Hugging Face-compatible checkpoint,
requires `spec.runtimeHints.task`, supports inline `caBundle`, and now also supports
`authSecretRef` through controller-owned projection. For `HuggingFace`, the
controller accepts source secrets with one of `token`, `HF_TOKEN`, or
`HUGGING_FACE_HUB_TOKEN` and normalizes them into a projected worker token.
For `HTTP`, the controller accepts either `authorization` or
`username`+`password` and projects only those keys into the worker namespace.
The controller-owned publication worker hardens tar/zip extraction and rejects
path traversal, symlink, hard link, and other special archive entries instead
of relying on raw `extractall`.

`spec.source.upload` now follows a controller-owned session flow rather
than a batch import. The controller creates a worker Pod, a ClusterIP Service,
and a short-lived auth Secret, then projects a local-machine helper command
through `status.upload.command`. The current live controller path accepts
the following uploads:

- for `Safetensors`: an archive with `config.json` and model weight files;
- for `GGUF`: either a direct `.gguf` file or an archive.

The controller then publishes them into the same controller-owned
`ModelPack`/OCI artifact plane through the current Go dataplane and
`ModelPack` adapter.

`spec.inputFormat` is treated as the source-agnostic validation contract for
the uploaded or downloaded model project, not as the final registry artifact
format. The final published form stays internal and fixed:
`ModelPack` in OCI. Regardless of whether bytes came from Hugging Face, HTTP,
or local upload, the controller now validates and sanitizes the project
composition before packaging. The current live rules are:

- `Safetensors`: requires a root `config.json`, at least one `.safetensors`
  file, allows known config/tokenizer/index companions, strips benign extras
  such as `README.md` and images, and rejects active or ambiguous files such
  as `.py`, `.sh`, `.dll`, `.so`, or other unsupported payloads.
- `GGUF`: requires at least one `.gguf` file, strips benign extras, and rejects
  the same active or ambiguous payloads.

For generic `HTTP`, this currently means:

- `Safetensors` expects an archive;
- `GGUF` can arrive as a direct file or as an archive.

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

Destructive cleanup also stays explicit and machine-oriented. The phase-2
controller now persists only an internal backend cleanup handle and runs
controller-owned one-shot Jobs through the dedicated runtime-image
`artifact-cleanup` subcommand. The current live cleanup path logs into the
publication OCI registry with the same controller-owned trust and credential
wiring and removes the remote artifact by its saved reference, while keeping
backend internals out of public status.

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
