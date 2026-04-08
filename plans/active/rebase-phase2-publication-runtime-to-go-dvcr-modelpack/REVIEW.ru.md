# REVIEW

## Итог

Phase-2 publication/runtime execution path больше не живёт в backend Python
scripts. Live path теперь controller-owned and Go-first:

- manager binary and phase-2 runtime binary are now split:
  - `cmd/ai-models-controller` for manager
  - `cmd/ai-models-artifact-runtime` for `publish-worker`,
    `upload-session`, and `artifact-cleanup`
- source acquisition moved into Go adapters
- source-agnostic ingest validation now lives in a dedicated Go adapter and
  runs before `ModelPack` packaging for `HuggingFace`, `HTTP`, and `Upload`
- `ModelPack` publish/remove moved behind an explicit Go port with current
  `KitOps` implementation adapter
- ai-inference-oriented resolved metadata now lands in public status with a
  stricter calculation path for current live formats
- dedicated runtime image owns the pinned `KitOps` binary
- controller runtime images now build from a module-local `distroless`
  relocation layer instead of pulling `base/distroless` directly
- `KitOps` delivery now has its own artifact stage instead of being hidden
  inside the Go build stage
- the lone `KitOps` installer script now sits in the controller root next to
  `kitops.lock` instead of creating a fake one-file `tools/` boundary
- backend image no longer carries phase-2 publication/upload/cleanup execution
  entrypoints
- render fixtures now include the dedicated `controllerRuntime` digest key for
  the `controller-runtime` image, so the
  new image path is covered by `helm-template`/`kubeconform` instead of being
  invisible to template validation
- controller registration now uses explicit unique names across
  `catalogcleanup` and `catalogstatus`, so startup no longer dies on duplicate
  controller-runtime controller/metric names
- `controller-kitops-artifact` and `distroless-artifact` no longer render a
  malformed `beforeInstall` command list after `alt packages proxy`; their
  `apt-get install` entries are back on the correct YAML level and no longer
  collapse into one broken `apt-get` invocation during `werf build`
- root chart now consumes vendored `deckhouse_lib_helm` through the normal
  DKP dependency path and no longer needs a repo-local helper fork in
  `templates/`

## Проверки

- `cd images/controller && go test ./...`
- `make verify`
- `werf build --dev --platform=linux/amd64 controller controller-runtime`
- `werf build --dev --platform=linux/amd64 distroless controller controller-runtime`
- `werf build --dev --platform=linux/amd64 controller-kitops-artifact controller-runtime`
- `werf build --dev --platform=linux/amd64 backend-source-artifact backend-ui-build backend-oidc-auth-ui-build bundle`
- `git diff --check`

Validation note:

- repo-level checks passed, including `make verify` and `git diff --check`;
- the new targeted `werf build` for `controller-kitops-artifact` was attempted
  twice but the local Docker API returned `_ping` `500 Internal Server Error`
  before the actual image stages started, so this specific build could not be
  re-confirmed end-to-end in the current environment;
- the later targeted `werf build` for `backend-source-artifact`,
  `backend-ui-build`, `backend-oidc-auth-ui-build`, and `bundle` also rendered a
  valid build plan, but hit the same local Docker API `_ping` `500 Internal
  Server Error` before the first base image finished.

## Архитектурный эффект

- virtualization-style ownership improved:
  - controller/runtime data plane in Go
  - build/install shell only for tool installation
  - controller runtime base image is now module-owned at the werf layer too,
    not only at the Go code layer
  - external `KitOps` packaging is now an explicit runtime artifact seam,
    not hidden inside controller compilation
  - hidden backend artifact plane remains hidden behind OCI/ModelPack contract
- public input contract is now simpler:
  - users provide `spec.source`
  - users provide `spec.inputFormat`
  - `spec.source` is either `source.url` or `source.upload`
  - `spec.inputFormat` can stay empty when the format is clear from contents
  - fixed internal `ModelPack` output stays implicit
- current live input formats are `Safetensors` and `GGUF`
- direct single-file `GGUF` now works on generic `HTTP` and `Upload` paths;
  archive uploads also keep their original filename through the upload session
- benign extras are stripped before packaging
- active/ambiguous files are rejected fail-closed
- remote source acquisition now has one canonical remote ingest entrypoint over
  shared provider-specific fetch helpers, so `publishworker` no longer repeats
  `HuggingFace` vs `HTTP` download/orchestration shell
- `sourceworker` and `uploadsession` no longer keep a separate replay-read path
  before the same `CreateOrGet` ensure cycle
- projected auth secret handling in `sourceworker` no longer keeps adapter-local
  `Get/Create/Update`; it now uses one direct reconcile path too
- `uploadsession` no longer keeps a separate request-mapping file; the only
  remaining mapping helper lives locally next to the pod builder
- public contract stays:
  - `ModelPack` in OCI
  - runtime input only `OCI from registry`
- backend phase-1 runtime remains untouched for MLflow-oriented concerns
- chart render path is now honest:
  - `deckhouse_lib_helm` comes from the vendored library chart in `charts/`
  - `.helmignore` no longer drops the library dependency during `helm template`
  - helper ownership matches `gpu-control-plane` / `virtualization` patterns
- remaining `KitOps` debt is now supply-chain provenance, not controller/runtime
  ownership: release asset fetching is still external, but no longer mixed into
  the Go build path

## Остаточный долг

- runtime delivery to `ai-inference` is still not wired

## Оставшиеся drifts против virtualization / gpu-control-plane

- backend Python build/runtime stages still use raw external `python:` bases;
  unlike the former raw `node:` UI stages, this part still has no mapped
  module-owned replacement in the current repo base-image set and remains the
  main honest shell debt.

## Текущая сверка с ADR

Отдельный audit зафиксирован в
[ADR_AUDIT.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/plans/active/rebase-phase2-publication-runtime-to-go-dvcr-modelpack/ADR_AUDIT.ru.md).

Короткий вывод:

- текущий `status` уже живой и в основном лучше структурирован, чем в ADR;
- текущий `spec` заметно ушёл от ADR;
- сам ADR сейчас нельзя считать точным описанием текущего public contract;
- в самом CRD главный remaining drift уже не в dead knobs, а в общем
  расхождении текущего `spec` с историческим ADR.
