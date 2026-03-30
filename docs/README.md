---
title: "Overview"
menuTitle: "Overview"
weight: 10
---

`ai-models` is the DKP module for AI/ML model registry and catalog services.
In the current implementation phase, the module runs its own managed internal
registry backend.

The repository currently contains:

- DKP module metadata and phase-1 runtime templates;
- a short stable user-facing configuration contract for logging, PostgreSQL,
  and S3-compatible artifacts; shared runtime settings are derived from
  platform and global Deckhouse defaults;
- native MLflow auth/workspaces, ingress/https, and managed-postgres wiring;
- `werf` and CI/CD pipelines for module packaging;
- repo-local guidance and skills for the next slices of backend packaging and
  DKP API work.
