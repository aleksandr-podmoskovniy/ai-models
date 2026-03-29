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

`artifacts` config defines the S3-compatible backend for ai-models artifacts:
bucket, path prefix, endpoint URL, region, TLS policy, addressing style, and
the Secret with access credentials.

High availability mode, HTTPS policy, certificate source, ingress behavior,
and Dex SSO are taken from global Deckhouse configuration and internal module
wiring. The current runtime expects:

- `global.modules.publicDomainTemplate` to be configured;
- HTTPS to be enabled globally;
- the `user-authn` module for module SSO;
- the `managed-postgres` module when `postgresql.mode=Managed`.

`Model` and `ClusterModel` are not exposed as part of the current user-facing
contract yet. They will be added later only when a stable module-level API is ready.
