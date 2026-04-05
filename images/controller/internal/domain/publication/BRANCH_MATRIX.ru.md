# Branch Matrix

## Slice

Вынесение publication phase/status rules и runtime decision tables из
`application/publication` в `domain/publication`.

## Covered branches

| Branch | Evidence |
| --- | --- |
| Terminal publication phases are explicit | `operation_test.go` |
| Upload status equality compares nil, command, repository and expiry | `operation_test.go` |
| Source worker created/running/success/failure decisions are explicit | `runtime_decisions_test.go` |
| Source worker awaiting-result branch requeues | `runtime_decisions_test.go` |
| Upload session running/update/expiry/success/failure decisions are explicit | `runtime_decisions_test.go` |
| Accepted upload starts in `Pending` | `status_test.go` |
| Running non-upload source stays in `Publishing` | `status_test.go` |
| Running upload without session stays pending and requeues | `status_test.go` |
| Running upload with issued session becomes `WaitForUpload` | `status_test.go` |
| Failed observation becomes `Failed` | `status_test.go` |
| Succeeded observation becomes `Ready` with cleanup handle | `status_test.go` |
| Malformed succeeded observation without snapshot fails closed | `status_test.go` |
| Malformed succeeded observation without cleanup handle fails closed | `status_test.go` |

## Residual gaps

- `ExecutionMode` selection remains in `application/publication`; this slice
  does not move source-type-to-execution-mode selection into the domain layer.
- Replay races and persisted ConfigMap corruption remain covered in
  `publicationoperation` tests, not in the domain package.
