# Branch Matrix

## Slice

`application/publication` после corrective cuts отвечает только за
execution-mode selection и planning use cases; source acceptance и
ready/failed short-circuit logic возвращены в `modelpublish`, а
phase/status invariants и worker/session decision tables вынесены в
`internal/domain/publication`.

## Covered branches

| Branch | Evidence |
| --- | --- |
| Terminal publication phases are explicit | `start_publication_test.go` |
| Publication mode selection for HF/HTTP/Upload is explicit | `start_publication_test.go` |
| Upload mode rejects unsupported format and missing task | `start_publication_test.go` |
| Source-worker planning resolves `authSecretRef` namespace for namespaced and cluster-scoped objects, validates HF/HTTP, and still rejects upload on the worker path | `plan_source_worker_test.go` |
| Upload-session issuance validates owner, operation, source type and current upload constraints | `issue_upload_session_test.go` |
| Thin facades for source/upload observation stay wiring-only and carry no independent behavioral logic | `domain/publication/runtime_decisions_test.go`, adapter package tests |

## Residual gaps

- Status/condition projection now lives in `internal/domain/publication`; its
  own branch matrix carries running/failed/ready state evidence.
- Source acceptance and reconcile short-circuit rules are now adapter-local in
  `modelpublish`; их поведение pinned adapter package tests, а не
  `application/publication`.
- Adapter-level reconcile replay and finalizer races remain covered only in
  existing package tests and will be strengthened in the next `modelpublish`
  and later `publicationoperation` follow-up cuts.
