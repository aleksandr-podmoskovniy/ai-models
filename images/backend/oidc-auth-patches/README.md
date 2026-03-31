# MLflow OIDC Auth Patch Bundle

Use this directory for `mlflow-oidc-auth` patches carried by the `ai-models`
backend image.

Unlike `images/backend/patches/`, which is reserved for the MLflow upstream
engine itself, this bundle patches the separately installed OIDC auth plugin
that runs the in-app SSO browser surface.

The plugin source can be fetched into a build-only directory with:

```bash
bash images/backend/scripts/fetch-oidc-auth-source.sh \
  --metadata images/backend/oidc-auth.lock \
  --dest .cache/mlflow-oidc-auth-upstream
```

Then validate the patch queue with:

```bash
bash images/backend/scripts/apply-patches.sh \
  --check \
  .cache/mlflow-oidc-auth-upstream \
  images/backend/oidc-auth-patches
```

Or use the repo-level shortcut:

```bash
make backend-oidc-auth-patches-check
```

If you already have a local checkout of `mlflow-oidc-auth`, avoid a network fetch:

```bash
OIDC_AUTH_SOURCE_DIR=/path/to/mlflow-oidc-auth make backend-oidc-auth-patches-check
```

## Required Metadata

- Upstream repository: `https://github.com/mlflow-oidc/mlflow-oidc-auth.git`
- Pinned stable release tag: `v6.7.1`
- Resolved commit from that release: `f523c006952e5eb4b004070983a4e86f38e2e7f5`
- Why the patch exists:
  - upstream `mlflow-oidc-auth` FastAPI permissions UI routes do not resolve
    request workspace context when MLflow workspaces are enabled;
  - in `ai-models` this breaks `Experiments`, `Prompts`, `Models` and adjacent
    OIDC permissions tabs with `Active workspace is required`.
- Expected upstreaming or removal path:
  - propose upstream middleware equivalent in `mlflow-oidc-auth`;
  - drop the local patch once the upstream release includes workspace-aware
    request handling for FastAPI routes.

## Rebase Procedure

1. Fetch the pinned plugin revision.
2. Re-apply the local patch series in order.
3. Re-run the local validation loop.
4. Update this README if the rationale or patch order changes.

## Validation

- `bash images/backend/scripts/fetch-oidc-auth-source.sh --metadata images/backend/oidc-auth.lock --dest .cache/mlflow-oidc-auth-upstream`
- `bash images/backend/scripts/apply-patches.sh --check .cache/mlflow-oidc-auth-upstream images/backend/oidc-auth-patches`
- `make backend-oidc-auth-patches-check`
- `make verify`
