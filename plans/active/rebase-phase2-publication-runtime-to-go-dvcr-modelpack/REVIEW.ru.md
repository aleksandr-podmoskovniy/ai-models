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
- backend image no longer carries phase-2 publication/upload/cleanup execution
  entrypoints
- render fixtures now include the dedicated `controller-runtime` digest, so the
  new image path is covered by `helm-template`/`kubeconform` instead of being
  invisible to template validation

## Проверки

- `cd images/controller && go test ./...`
- `make verify`
- `werf build --dev --platform=linux/amd64 controller controller-runtime`
- `git diff --check`

## Архитектурный эффект

- virtualization-style ownership improved:
  - controller/runtime data plane in Go
  - build/install shell only for tool installation
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

## Остаточный долг

- runtime delivery to `ai-inference` is still not wired

## Текущая сверка с ADR

Отдельный audit зафиксирован в
[ADR_AUDIT.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/plans/active/rebase-phase2-publication-runtime-to-go-dvcr-modelpack/ADR_AUDIT.ru.md).

Короткий вывод:

- текущий `status` уже живой и в основном лучше структурирован, чем в ADR;
- текущий `spec` заметно ушёл от ADR;
- сам ADR сейчас нельзя считать точным описанием текущего public contract;
- в самом CRD главный remaining drift уже не в dead knobs, а в общем
  расхождении текущего `spec` с историческим ADR.
