# Backend Patch Bundle

Use this directory for upstream backend engine patches.

The upstream source is fetched into build-only directories and does not live in the git tree of this module.

For local development fetch it into `.cache/backend-upstream` with:

```bash
bash images/backend/scripts/fetch-source.sh --metadata images/backend/upstream.lock --dest .cache/backend-upstream
```

If you already have a local upstream checkout, override the external fetch source with:

```bash
BACKEND_SOURCE_DIR=/path/to/backend-source bash images/backend/scripts/fetch-source.sh --dest .cache/backend-upstream
```

Pinned upstream metadata is tracked in `images/backend/upstream.lock`.

## Required Metadata

- Upstream repository:
- Pinned tag or commit:
- Why the patch exists:
- Expected upstreaming or removal path:

## Rebase Procedure

1. Fetch the pinned upstream revision.
2. Re-apply the local patch series in order.
3. Re-run the local validation loop.
4. Update this README if the rationale or patch order changes.

## Validation

- `make lint`
- `make test`
- `make backend-shell-check`
- `make backend-build-local`

Do not keep ad-hoc local modifications to fetched upstream sources outside the patch bundle.
