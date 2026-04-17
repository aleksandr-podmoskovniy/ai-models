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

- `raw/` for controller-owned upload staging and source mirror;
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

- `Model` / `ClusterModel` with `spec.source.url` fetch remote bytes through the
  controller-owned source mirror path;
- `spec.source.upload` uses the controller-owned upload-session path;
- both paths publish OCI `ModelPack` artifacts into the internal `DMCR`.

The publication workspace default is `PersistentVolumeClaim`, not `EmptyDir`.
If `storageClassName` is left empty, the generated PVC uses the cluster default
`StorageClass`. Large-model publication therefore requires enough persistent
storage capacity rather than relying on node ephemeral storage.

The public model API is also intentionally minimal. Users specify only
`spec.source`; format, task, and other model metadata are calculated by the
controller from the actual model contents and projected into `status.resolved`.
