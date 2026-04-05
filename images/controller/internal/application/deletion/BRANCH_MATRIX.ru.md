# Branch Matrix

## Slice

Bounded corrective cut for `modelcleanup`: finalizer guard logic and delete
cleanup policy move out of the adapter into `internal/application/deletion`.

## Covered branches

| Branch | Evidence |
| --- | --- |
| Invalid cleanup handle fails closed before finalizer mutation | `ensure_cleanup_finalizer_test.go` |
| Missing handle without finalizer is explicit noop | `ensure_cleanup_finalizer_test.go` |
| Missing handle removes stale finalizer | `ensure_cleanup_finalizer_test.go` |
| Present handle adds finalizer only when needed | `ensure_cleanup_finalizer_test.go` |
| Deletion without finalizer is explicit noop | `finalize_delete_test.go` |
| Invalid delete-time handle becomes cleanup-failed status path | `finalize_delete_test.go` |
| Missing delete-time handle removes finalizer | `finalize_delete_test.go` |
| Unsupported cleanup handle kind becomes cleanup-blocked status path | `finalize_delete_test.go` |
| Missing cleanup job creates job and requeues | `finalize_delete_test.go` |
| Running cleanup job keeps pending status and requeues | `finalize_delete_test.go` |
| Failed cleanup job fails closed and requeues | `finalize_delete_test.go` |
| Completed cleanup job removes finalizer | `finalize_delete_test.go` |
| Unsupported cleanup job state fails closed | `finalize_delete_test.go` |

## Residual gaps

- Adapter-level create-race handling and status-update replay are still covered
  only partly in `internal/modelcleanup` tests and should be tightened in a
  separate bounded follow-up if this path changes again.
