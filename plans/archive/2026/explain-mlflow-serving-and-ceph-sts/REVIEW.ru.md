## Findings
- Критичных замечаний нет.

## Checks
- Official Ceph docs for Squid STS / OIDC web identity
- Official MLflow docs for tracking, evaluation, and model registry workflows
- Official KServe docs for S3 storage credentials
- Current repo importer / cleanup workflow

## Residual risks
- Ceph docs explicitly mention OIDC/OAuth2 and Keycloak as the tested integration; Dex is not called out explicitly, so Dex->RGW web-identity STS should be treated as a PoC/integration task, not an already-proven contract.
- Current ai-models phase-1 contract still exposes raw MLflow concepts; the user-facing simplification belongs to future `Model` / `ClusterModel`.
