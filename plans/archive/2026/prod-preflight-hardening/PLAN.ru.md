# План: prod preflight hardening

## Current phase

Phase 1/2 publication/runtime baseline перед production rollout.

## Orchestration

`solo`: текущий проход является systematic audit + narrow fixes. Delegation не
используется, потому что пользователь не просил subagents в этом turn; широкий
architecture review можно запускать отдельным `full` slice.

## Active bundle disposition

- `live-e2e-ha-validation` — keep: executable e2e after rollout.
- `observability-signal-hardening` — keep: отдельный observability stream.
- `ray-a30-ai-models-registry-cutover` — keep: отдельная workload migration.

## Slices

1. Audit API/CRD/status surface.
   - Files: `api/`, `crds/`, `images/controller/internal/*status*`.
   - Goal: find public Secret/token leaks and convention drift.
   - Check: targeted `rg`, API tests, CRD verify.
   - Status: completed. No new public Secret/token field was introduced by this
     slice; existing upload secret URL contract is already secret-backed and
     intentionally status-projected.

2. Audit templates and RBAC.
   - Files: `templates/`.
   - Goal: find wildcard/broad verbs, missing securityContext/token hardening,
     dangerous host mounts or exposure drift.
   - Check: render, kubeconform.
   - Status: completed. Found publication workers reused the broad controller
     ServiceAccount and runtime containers missed explicit restricted security
     context.

3. Implement narrow fixes.
   - Files: based on findings only.
   - Goal: fix defects without introducing new topology/UX.
   - Check: narrow tests per package.
   - Status: completed. Split publication worker identity into a dedicated
     module-local ServiceAccount/RoleBinding and hardened source-worker plus
     materializer containers.

4. Final validation and review.
   - Files: current bundle notes.
   - Goal: record evidence and residual risks.
   - Check: `make verify`, `git diff --check`, review-gate.
   - Status: completed.

## Rollback point

Revert this bundle's template/code changes to return to the previous verified
pre-prod state.

## Final validation

- `cd api && go test ./...`
- `bash api/scripts/verify-crdgen.sh`
- `cd images/controller && go test ./...`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
- `rg -n 'resources:\s*\["\*"\]|verbs:\s*\["\*"\]|apiGroups:\s*\["\*"\]' templates tools/kubeconform/renders`
- `make verify`
- `git diff --check`

## Evidence

- `python3 -m py_compile tools/helm-tests/validate-renders.py`
- `python3 -m unittest tools/helm-tests/validate_renders_test.py`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `cd images/controller && go test ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/modeldelivery`
- `make helm-template`
- `make kubeconform`

## Findings Closed

- Publication worker Pods no longer inherit `ai-models-controller`
  ClusterRole. The controller now passes
  `--publication-worker-service-account=ai-models-publication-worker`, and the
  worker gets only module-namespace Secret `get/update` through a dedicated
  RoleBinding.
- Source-worker publication runtime containers now have explicit restricted
  `securityContext`: non-root user, no privilege escalation, read-only rootfs,
  dropped capabilities and `RuntimeDefault` seccomp.
- Workload materializer init containers now have explicit restricted
  `securityContext` without forcing a numeric UID over user workload policy.
- Render guardrails now fail if publication worker identity is wired back to
  the controller ServiceAccount or if the dedicated ServiceAccount/RoleBinding
  is missing.

## Residual Risks

- Controller ClusterRole still reads Secrets cluster-wide because current
  `Model.spec.source.authSecretRef` allows namespace-local source auth
  projection. Narrowing that requires an API/UX slice, not a pre-prod hidden
  behavior change.
- Node-cache CSI runtime remains privileged with host mounts by design. Its
  risk boundary is the dedicated node-cache image/runtime and should be tested
  in the e2e/HA bundle.
