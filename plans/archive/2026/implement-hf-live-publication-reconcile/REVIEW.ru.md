# REVIEW

## Findings

- Нет блокирующих замечаний по scope этого slice.
- Появился первый live publication path для `Model` / `ClusterModel` с
  `spec.source.type=HuggingFace`.
- Controller-owned HF import Job и structured result handoff выделены в
  `internal/hfimportjob`, а live reconcile lifecycle — в
  `internal/modelpublish`.
- Publish controller держит public ownership только над `phase`, `source`,
  `artifact`, `resolved`, `Accepted`, `ArtifactPublished`, `MetadataReady`,
  `Validated`, `Ready`, и не трогает delete lifecycle, который остаётся у
  cleanup controller.
- Cleanup handle пишется только после успешного publication result и остаётся
  internal-only annotation, без утечки backend identities в public `status`.

## Проверки

Выполнено:

- `go test ./internal/hfimportjob ./internal/modelpublish` in `images/controller`
- `go test ./...` in `images/controller`
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

## Residual risks

- Live path пока реализован только для `HuggingFace -> managed backend mlflow`.
  `Upload`, `HTTP`, `OCIArtifact` и OCI publication в payload-registry остаются
  следующим этапом.
- `spec.source.huggingFace.authSecretRef` для private/gated models пока
  намеренно не реализован и приводит к failed publication path с понятным
  сообщением.
- Result handoff сейчас опирается на pod termination message. Для текущего
  slice это достаточно компактно и проверяемо, но при росте publication result
  может понадобиться более явный channel.
- Runtime delivery/materialization agent, digest enforcement inside agent и pod
  mutation всё ещё вне этого slice.
