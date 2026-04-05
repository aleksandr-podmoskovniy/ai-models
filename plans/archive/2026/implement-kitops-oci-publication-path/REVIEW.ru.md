# Review

## Findings

- Critical blockers не найдены.

## Missing checks

- Не было live smoke against a real OCI registry from inside the backend worker
  image. Current confidence built from unit tests, controller integration tests,
  `kit` CLI help/runtime probes, and repo-level validation.
- `Upload(ModelKit)` и `HTTP/HF authSecretRef` по-прежнему не входят в live
  scope этого slice.

## Residual risks

- `kit init` classification зависит от реального checkpoint contents; для
  exotic model layouts может понадобиться более строгий controller-owned
  `Kitfile` generation вместо current auto-generated baseline.
- Cleanup path теперь реализован через `kit remove --remote`, но без live smoke
  against a real registry confidence пока строится на unit/integration checks и
  runtime wiring, а не на end-to-end delete against a registry backend.
- Publication worker пишет public artifact URI и internal cleanup reference как
  immutable digest reference. Это правильно для public UX, но оставляет риск
  registry-specific delete semantics до live smoke against a real OCI backend.
- Module config `publicationRegistry` теперь обязателен для phase-2 publish
  plane; без него controller runtime не поднимет live publication workers.
