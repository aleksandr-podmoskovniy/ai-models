# Use Cases

## Publication

- `AcceptSource`
  - validate source acceptance preconditions
  - choose initial lifecycle state
- `StartPublication`
  - create or ensure publication operation
- `ObservePublication`
  - interpret worker/session outcome
  - produce domain result
- `ProjectStatus`
  - translate domain state into public `status`

## Upload

- `IssueUploadSession`
  - create session contract
  - issue TTL/token
- `ObserveUploadSession`
  - decide `WaitForUpload` / `Publishing` / `Failed`

## Deletion

- `EnsureFinalizer`
- `CheckDeleteGuards`
- `FinalizeDelete`

## Rule

Each use case owns one bounded decision surface. If one function starts deciding
about source policy, worker pod layout, and public status together, the cut is
wrong.
