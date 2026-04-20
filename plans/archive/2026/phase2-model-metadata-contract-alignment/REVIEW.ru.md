## Review gate

- Scope match:
  - landed exactly the intended public-contract cut: `ModelSpec` now keeps only
    `source`, generated CRDs match it, and controller/runtime no longer depend
    on removed spec metadata/policy knobs.
- Architecture:
  - no new controller/package drift was introduced;
  - dead policy helpers in `publishstate` were removed instead of being kept
    alive by synthetic tests.
- Consistency:
  - repo docs were synced to the source-only public contract;
  - `verify-crdgen.sh` was updated to assert the new immutable/public schema
    rules.
- Validations run:
  - `cd images/controller && go test ./...`
  - `cd api && go test ./...`
  - `cd api && bash ./scripts/verify-crdgen.sh`
  - `make verify`

## Residual risks

- `spec.source.upload` and `spec.source.authSecretRef` intentionally remain in
  public `spec`; upload still needs an explicit source discriminator and gated
  remote sources still need a user-provided auth secret.
- `status.resolved.task` is now best-effort calculated metadata; for formats
  where task cannot be inferred honestly, the field may stay empty instead of
  being forced from public input.
- Out-of-repo ADR `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`
  was not part of this repo slice and should be synced separately if it still
  describes the removed heavy spec.
