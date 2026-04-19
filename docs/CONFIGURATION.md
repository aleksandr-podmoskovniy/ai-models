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
  `artifacts.sourceAcquisitionMode=Mirror`, the temporary source mirror;
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

- `Model` / `ClusterModel` use one cluster-level
  `artifacts.sourceAcquisitionMode`:
  - `Mirror`:
    remote `source.url` first goes through a controller-owned source mirror;
  - `Direct`:
    remote `source.url` is consumed directly from the canonical remote source
    boundary;
- `spec.source.upload` uses the controller-owned upload-session path and stays
  on its staged object boundary under the same acquisition contract;
- all paths publish OCI `ModelPack` artifacts into the internal `DMCR`.

The default is `artifacts.sourceAcquisitionMode=Direct`.

The trade-off is explicit:

- `Mirror` keeps a durable intermediate copy in object storage and makes
  re-publish/resume on the remote ingest boundary more predictable;
- `Direct` removes that extra full copy and speeds up the first remote import;
- for `spec.source.upload`, the effective source boundary is already the staged
  object, so the mode does not create a second intermediate copy on top of the
  upload staging contract.

There is no separate upload-mode choice for heavy layer blobs published into
`DMCR`. The canonical byte path is now fixed:

- `publish-worker -> DMCR direct-upload helper -> DMCR backing storage`.

`DMCR` still owns authentication, final blob/link materialization, and the
published artifact contract, but the thick byte stream no longer goes through
the registry `PATCH` path. This removes `DMCR` itself as the network bottleneck
on the heavy upload path.

In the current bounded slice direct upload affects only heavy layer blobs. The
`config` blob, manifest publish, and final remote inspect still use the normal
registry path so the internal contract changes one responsibility at a time.

The successful publication worker path no longer uses a local workspace/PVC.
`HuggingFace` in both modes and staged upload publish through
object-source/archive-source streaming semantics. The only local bounded
storage contract left for the publish worker is the container
`ephemeral-storage` request/limit for logs and writable layer usage.

The public model API is also intentionally minimal. Users specify only
`spec.source`; format, task, and other model metadata are calculated by the
controller from the actual model contents and projected into `status.resolved`.
