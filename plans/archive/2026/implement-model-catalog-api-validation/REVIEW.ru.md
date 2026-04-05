# REVIEW

Критичных замечаний по текущему slice не найдено.

Что проверено:
- validation/defaulting/immutability остаются в `api/` и не тянут controller
  runtime, templates или phase-1 backend areas;
- `spec` остаётся desired state, `status` не загрязнён internal backend
  деталями;
- `spec.source` получил schema-level one-of semantics;
- artifact-producing `spec` fields (`source`, `package`, `publish`,
  `runtimeHints`) зафиксированы как immutable;
- defaults добавлены только для уже устойчивой happy-path surface:
  `package` и `upload.expectedFormat`;
- появился воспроизводимый CRD schema verify path без коммита production CRD
  manifests в репозиторий.

Выполненные проверки:
- `go generate ./...` в `api/`
- `go test ./...` в `api/`
- `bash ./scripts/verify-crdgen.sh` в `api/`
- `make fmt`
- `make test`
- `git diff --check`

Residual risks:
- `ClusterModel` сейчас валидируется жёстче, чем `Model`:
  - `spec.access` обязателен;
  - namespaced refs для service accounts и HF auth secret должны быть explicit.
  Это выглядит согласованно с current design intent, но стоит переподтвердить
  перед первым external consumer UX.
- `spec.publish` целиком помечен immutable. Это безопасно для v1 baseline, но
  будущие promotion flows внутри одного объекта могут потребовать более тонкого
  split между immutable repository identity и mutable channel semantics.
- verify script тянет `controller-gen` через `go run ...@v0.18.0`, то есть
  verification reproducible по версии, но зависит от доступности модуля при
  cold run.
