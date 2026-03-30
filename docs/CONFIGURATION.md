---
title: "Configuration"
menuTitle: "Configuration"
weight: 60
---

<!-- SCHEMA -->

The current `ai-models` configuration contract is intentionally short.
Only stable ai-models-specific settings are exposed at the module level:
logging, PostgreSQL wiring, and S3-compatible artifact storage.

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

`artifacts` config defines the S3-compatible backend for ai-models artifacts:
bucket, path prefix, endpoint URL, region, TLS policy, addressing style, and
credentials.

Artifact credentials can be provided in two ways:

- via `credentialsSecretName` that points to an existing Secret in `d8-ai-models`
  with fixed keys `accessKey` and `secretKey`;
- via inline `accessKey` and `secretKey` in ModuleConfig, in which case the
  module renders an internal Secret in `d8-ai-models`.

`bucket`, `pathPrefix`, `endpoint`, `region`, and addressing/TLS flags are not
treated as secrets. They remain part of the normal module configuration surface.

High availability mode, HTTPS policy, ingress behavior, and Dex SSO are taken
from global Deckhouse configuration and internal module wiring. The current
runtime expects:

- `global.modules.publicDomainTemplate` to be configured;
- HTTPS to be enabled globally through Deckhouse module HTTPS policy
  (`CertManager` or `CustomCertificate`);
- the `user-authn` module for module SSO;
- the `managed-postgres` module when `postgresql.mode=Managed`.

`Model` and `ClusterModel` are not exposed as part of the current user-facing
contract yet. They will be added later only when a stable module-level API is ready.
