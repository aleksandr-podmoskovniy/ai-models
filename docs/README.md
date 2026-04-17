---
title: "Overview"
menuTitle: "Overview"
weight: 10
---

`ai-models` is the DKP module for AI/ML model catalog and publication/runtime
delivery services.
In the current implementation phase, the platform-facing baseline is centered
on `Model` / `ClusterModel`, controller-owned publication, and runtime
delivery. The historical backend attempt is no longer part of the live
repository baseline.

The repository currently contains:

- DKP module metadata and runtime templates for module-owned publication
  surfaces;
- a short stable user-facing configuration contract for logging, PostgreSQL,
  and S3-compatible artifacts; shared runtime settings are derived from
  platform and global Deckhouse defaults;
- phase-2 `Model` / `ClusterModel` API and controller work for source-first
  publication into OCI-backed `ModelPack` artifacts; a standalone runtime
  materializer and reusable consumer-side K8s wiring now exist for
  `OCI -> local path`, while concrete runtime integration still remains a
  separate workstream;
- `werf` and CI/CD pipelines for module packaging;
- repo-local guidance and skills for the next slices of publication,
  distribution, and DKP API work.
