# Quality Gates

## 1. Mandatory hard gates

### Cyclomatic complexity

- Tool: `gocyclo`
- Threshold: `<= 15` per function
- Scope:
  - `images/controller/internal/...`
  - exclude `_test.go`

### Max LOC per file

- Threshold: `<= 350` LOC for non-test Go files under `images/controller/internal`
- Initial exceptions are allowed only through an explicit allowlist file checked
  into git and referenced from the current bundle:
  - `tools/controller-loc-allowlist.txt`

### Thin reconciler check

Custom repo check for files named `reconciler.go` under
`images/controller/internal/`:

- max LOC stricter target: `<= 250`
- forbid direct creation of large K8s object literals for:
  - `corev1.Pod{`
  - `corev1.Service{`
  - `corev1.Secret{`
  - `corev1.ConfigMap{`

This is intentionally heuristic. The goal is to block obvious regressions.
Initial exceptions are allowed only through:

- `tools/thin-reconciler-allowlist.txt`

### Coverage

- Package-level coverage threshold for controller packages touched by a slice:
  - initial floor: `>= 80%`
- For pure orchestration adapters, controller test evidence is mandatory even if
  statement coverage is high.
- Current implementation scopes these two gates to future
  `domain` / `application` packages, so they are already wired into `verify`
  without breaking the current pre-refactor controller.
- Coverage artifacts must also be collected under `artifacts/coverage` through
  bounded controller package groups, similar to `gpu-control-plane`, so the
  coverage gate is not the only evidence left after a local run.

### Deadcode

- Tool: `golang.org/x/tools/cmd/deadcode`
- Scope:
  - `images/controller/cmd/...`
  - `images/controller/internal/...`
- Mode:
  - run with `-test`
  - fail on any reported dead function or method
- Policy:
  - no silent allowlist by default;
  - if a future exception is unavoidable, it must be explicit and justified in
    the current bundle first.

## 2. Verify hook points

### `Makefile`

Add:

- `ensure-gocyclo`
- `ensure-deadcode`
- `coverage-dir`
- `controller-coverage-artifacts`
- `lint-controller-complexity`
- `lint-controller-size`
- `lint-thin-reconcilers`
- `test-controller-coverage`
- `deadcode`
- `check-controller-test-evidence`

Wire them into:

- `ensure-tools`
- `verify`
- `verify-ci`

### `tools/`

Add scripts:

- `tools/install-gocyclo.sh`
- `tools/install-deadcode.sh`
- `tools/check-controller-complexity.sh`
- `tools/check-controller-loc.sh`
- `tools/check-thin-reconcilers.sh`
- `tools/check-controller-test-evidence.sh`
- `tools/check-controller-deadcode.sh`
- `tools/collect-controller-coverage.sh`
- `tools/test-controller-coverage.sh`
- `tools/controller-complexity-allowlist.txt`
- `tools/controller-loc-allowlist.txt`
- `tools/thin-reconciler-allowlist.txt`

## 3. Non-negotiable policy

If a future slice violates these gates, the answer is not “tests pass anyway”.
The slice must stop and be cut differently.
