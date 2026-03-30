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

High availability mode and HTTPS policy are taken from global Deckhouse
configuration and internal module wiring. The current runtime expects:

- `global.modules.publicDomainTemplate` to be configured;
- HTTPS to be enabled globally through Deckhouse module HTTPS policy
  (`CertManager` or `CustomCertificate`);
- the `managed-postgres` module when `postgresql.mode=Managed`.

Backend authentication is now provided by upstream-native MLflow basic auth
plus MLflow workspaces. The module renders an internal auth Secret with:

- the bootstrap admin username;
- a stable generated admin password;
- a stable `MLFLOW_FLASK_SERVER_SECRET_KEY` required by the upstream auth app.

The backend service is therefore no longer protected only at ingress level.
Direct access to the raw backend still requires MLflow credentials, and logical
segmentation happens through native MLflow workspaces.

Large machine-oriented imports use direct artifact access instead of server-side
artifact proxying. The backend runs with `--no-serve-artifacts`, and in-cluster
import Jobs authenticate to MLflow metadata APIs while uploading artifacts
directly to S3.

The current phase-1 backend runtime profile is intentionally conservative:
each backend pod runs a single MLflow web worker, and MLflow server job
execution is disabled. High availability for the backend is achieved through
Deckhouse module HA and pod replicas, not through extra in-process workers or
genai job consumers.

The backend also keeps MLflow security middleware enabled. The module derives
MLflow `allowed-hosts` and same-origin CORS settings from the public ingress
domain and preserves the private-network/service access patterns needed for
in-cluster access. Health probes use the upstream unauthenticated `/health`
endpoint, and `ServiceMonitor` scrapes `/metrics` via MLflow basic auth.

The module also provisions an internal Secret with a stable
`MLFLOW_CRYPTO_KEK_PASSPHRASE` value for upstream MLflow crypto-backed runtime
features. This removes the insecure upstream default passphrase from shared
cluster deployments without exposing the KEK as a user-facing module setting.

`Model` and `ClusterModel` are not exposed as part of the current user-facing
contract yet. They will be added later only when a stable module-level API is ready.
