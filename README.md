# ai-models

`ai-models` is the DKP module for AI/ML model registry and catalog services.

The current live baseline of the module is a controller-owned
publication/runtime path:

- DKP-native `Model` / `ClusterModel` catalog API;
- controller publication into canonical OCI `ModelPack` artifacts;
- internal `DMCR` as the publication backend;
- source mirror / upload staging in S3-compatible storage;
- runtime delivery for consumer workloads.

Current implementation scope:
1. stabilize the ai-models-owned publication/runtime baseline;
2. add distribution/runtime topologies such as `DMZ` registry and node-local cache;
3. hardening, controlled patching, and long-term support improvements.

What is already part of the repository:
- DKP module metadata and user-facing documentation;
- stable `config-values` for the controller/runtime shell and object storage wiring;
- runtime templates for `DMCR`, controller shell, and module-wide manifests;
- runtime/internal `values` and image-based Go hooks for module wiring;
- `werf`, CI/CD, and repo-local workflows for packaging module-owned controller
  / artifact runtime images; the consumer serving image stays an external
  runtime surface, for example from `Docker Hub`.

Current import/publication flow:
- the canonical live path goes through `Model` / `ClusterModel`;
- remote sources are handled through controller-owned source mirror;
- upload sources are handled through controller-owned upload sessions;
- publication produces OCI `ModelPack` artifacts in the internal `DMCR`.

Development entrypoints:
- `AGENTS.md`
- `DEVELOPMENT.md`
- `docs/development/TZ.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `plans/active/`
