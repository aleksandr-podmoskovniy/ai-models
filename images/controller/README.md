# images/controller

`images/controller/` is the canonical root for executable controller code of the
`ai-models` module.

Rules:
- phase 1 may keep this directory documentation-only while the module runs only
  the internal backend;
- when phase 2 starts, controller source, module-local `go.mod`, and image build
  files must appear from this root;
- reconcile code must not be introduced under top-level `controllers/`;
- public DKP API types still live in top-level `api/`.
