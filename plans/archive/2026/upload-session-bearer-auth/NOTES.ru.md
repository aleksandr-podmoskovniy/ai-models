## 2026-04-23. Secret-reference hardening и cleanup audit

### Решение

- Raw upload bearer больше не является частью публичного `status.upload`.
- Runtime upload-session Secret остаётся hash-only.
- User-facing bearer header value хранится в отдельном owner-scoped Secret,
  на который указывает `status.upload.tokenSecretRef`.
- Для namespaced `Model` token Secret создаётся в namespace модели.
- Для cluster-scoped `ClusterModel` token Secret создаётся в runtime namespace
  модуля.

### Cleanup audit

- `make deadcode` не нашёл удаляемого dead code.
- `rg` по `legacy`, `monolith`, `KitOps`, `query-token`,
  `authorizationHeaderValue` не выявил live runtime fallback, который можно
  безопасно удалить в этом slice.
- Оставшиеся legacy markers относятся к guardrail-тестам, docs/evidence или
  отрицательным проверкам против возврата старых поверхностей.
- Query-token строки в `dataplane/uploadsession` тестах оставлены намеренно:
  они доказывают, что `?token=` больше не является валидной аутентификацией.

### Миграционный риск

Существующие активные upload sessions старого формата, у которых raw bearer был
только в `status.upload.authorizationHeaderValue`, после обновления не могут
быть восстановлены из hash-only session Secret. При следующем reconcile
controller выпустит новый token handoff Secret и обновит hash в runtime Secret.
Это безопаснее, но может инвалидировать старый in-flight upload token.

### Проверки

- `go generate ./core/...` в `api`
- `bash scripts/verify-crdgen.sh` в `api`
- `go test ./...` в `api`
- `go test ./...` в `images/controller`
- `make deadcode`
- `make lint`
- `make check-controller-test-evidence`
- `make helm-template`
- `make kubeconform`
- `make verify`
