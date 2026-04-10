---
title: "Overview"
menuTitle: "Overview"
weight: 10
---

`ai-models` is the DKP module for AI/ML model catalog and publication/runtime
delivery services.
In the current implementation phase, the module keeps a hidden managed backend
next to the platform-facing `ModelPack` catalog/runtime path instead of
exposing backend internals as the platform contract.

The repository currently contains:

- DKP module metadata and phase-1 runtime templates;
- a short stable user-facing configuration contract for logging, PostgreSQL,
  and S3-compatible artifacts; shared runtime settings are derived from
  platform and global Deckhouse defaults;
- native MLflow auth/workspaces, ingress/https, and managed-postgres wiring;
- phase-2 `Model` / `ClusterModel` API and controller work for source-first
  publication into OCI-backed `ModelPack` artifacts; live runtime
  materialization into local paths is still a separate future workstream;
- `werf` and CI/CD pipelines for module packaging;
- repo-local guidance and skills for the next slices of backend packaging and
  DKP API work.
