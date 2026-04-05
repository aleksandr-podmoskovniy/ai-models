# Test Strategy

## 1. Test by decisions, not by files

Controller lifecycle code must be tested by behavior:

- state transition matrix
- negative branches
- idempotency
- retry/reconcile replay
- deletion/finalizer races
- malformed worker/session result paths

## 2. Mandatory branch-matrix artifacts

For each lifecycle package there must be a checked-in matrix table in the
bundle or package docs covering at least:

- input state
- event / observation
- expected next state
- expected side effect
- expected condition/reason

## 3. Required scenarios

### Publication

- source accepted
- unsupported source rejected
- operation created once
- operation replay is idempotent
- malformed worker result fails predictably
- successful result projects `Ready`

### Upload

- wait-for-upload projection
- expired/invalid session
- duplicate reconcile during active session
- successful upload completion
- format mismatch

### Delete

- no finalizer without cleanup handle
- deletion blocked by active guards
- cleanup retry after transient failure
- finalizer released only after cleanup success

## 4. Test package structure

- domain tests for state transitions
- application tests for use-case decisions
- adapter tests for K8s object materialization and IO boundaries
- shared controller fixtures under `internal/support/testkit`
- package-local `test_helpers_test.go` only for adapter-local options,
  resource builders and assertions
- split adapter-heavy reconcile coverage by decision family instead of keeping
  one growing `reconciler_test.go` monolith
- for one concrete persisted boundary, prefer one boundary test file over
  separate `codec/mutation/status` files when they do not represent different
  architectural seams

Avoid adapter-heavy tests as the only evidence for business behavior.

## 5. Fixture discipline

- do not re-declare the same scheme bootstrap in each controller package
- do not duplicate `Model` / `ClusterModel` base fixtures across controller
  packages
- do not hide business branching inside helper files; helpers may build test
  data, not decide expected behavior
