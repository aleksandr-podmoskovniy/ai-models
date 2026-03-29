# ai-models

`ai-models` is the DKP module for AI/ML model registry and catalog services.

The current module runtime already covers the core registry services inside DKP:
metadata storage in PostgreSQL, S3-compatible artifact storage, UI/API access,
Deckhouse ingress/https, Dex SSO, and reproducible image packaging.

Current implementation scope:
1. the internal managed backend of the module;
2. DKP-native `Model` / `ClusterModel` API on top of the same module;
3. hardening, controlled patching, and long-term support improvements.

What is already part of the repository:
- DKP module metadata and user-facing documentation;
- stable `config-values` for ai-models logging, PostgreSQL, and S3-compatible artifacts;
- runtime templates for the internal backend, `Ingress`, `DexAuthenticator`, and managed-postgres wiring, while shared behavior comes from platform/global settings;
- runtime/internal `values` and image-based Go hooks for module wiring;
- `werf`, CI/CD, and repo-local workflows for packaging the internal backend engine.

Development entrypoints:
- `AGENTS.md`
- `DEVELOPMENT.md`
- `docs/development/TZ.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `plans/active/`
