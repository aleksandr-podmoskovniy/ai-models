# api/

`api/` is reserved for DKP-native public API types of the `ai-models` module.

Rules:
- add only module-facing API contracts here;
- do not place internal backend engine types here;
- do not place executable controller code here;
- phase 1 may keep this directory documentation-only;
- phase 2 introduces `Model`, `ClusterModel`, shared validation, defaults, and generated artifacts from this root;
- controller runtime for those APIs must live under `images/controller/`.
