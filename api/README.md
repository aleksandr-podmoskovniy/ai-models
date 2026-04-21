# api/

`api/` is reserved for DKP-native public API types of the `ai-models` module.

Rules:
- add only module-facing API contracts here;
- do not place internal backend engine types here;
- do not place executable controller code here;
- phase 1 may keep this directory documentation-only;
- phase 2 introduces `Model`, `ClusterModel`, shared validation, defaults, and generated artifacts from this root;
- controller runtime for those APIs must live under `images/controller/`.

Current phase-2 baseline:
- public API group: `ai.deckhouse.io`;
- initial version: `v1alpha1`;
- shared `Model` / `ClusterModel` types live under `api/core/v1alpha1/`;
- upload-driven objects expose staging upload contract via `status.upload`,
  with upload URLs and a separate bearer authorization header value instead of
  query-token URLs;
- upload-driven objects also expose one public local-upload progress indicator
  via top-level `status.progress`, following the same UX shape used in
  `virtualization`;
- published artifacts are described in `status.artifact` as the published OCI
  artifact reference, digest, and media metadata;
- delete lifecycle is represented through `phase=Deleting` and
  `status.conditions`, while backend-specific cleanup handles stay internal to
  controller-owned state;
- generated deepcopy artifacts are refreshed with `go generate ./...` from this module root.
- CRD schema markers are checked with `bash scripts/verify-crdgen.sh`.
