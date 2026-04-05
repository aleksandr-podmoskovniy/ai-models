# REVIEW

## Findings

- Нет блокирующих замечаний по scope этого slice.
- `status` rebased на agreed shape: `resolved` вместо `metadata`, `ObjectStorage` вместо `S3`, public phase/conditions очищены от internal-only orchestration markers.
- `publication` и `runtimedelivery` пересобраны в сторону agreed contracts: `ResolvedProfile`, `Artifact`, `AccessPlan`, `VerificationPlan`.

## Проверки

Выполнено:

- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `go test ./...` in `api`
- `go test ./...` in `images/controller`
- `make fmt`
- `make test`
- `make verify`
- `git diff --check`

## Residual risks

- Полный rebaseline `spec.source` и `spec.publish` всё ещё вне этого slice. В public API по-прежнему остаются старые элементы contract shape, включая `OCIArtifact` source variant и publish-shape, который ещё не доведён до обновлённого ADR.
- `publication.ResolvedProfile` использует internal non-pointer shape для части полей, тогда как public `status.resolved` хранит nullable значения. Для текущего planning slice это допустимо, но live reconciliation надо будет сделать явно и без неявных zero-value преобразований.
- `publication.Snapshot.Validate()` пока не валидирует сам `ResolvedProfile`. Для этого slice это не блокер, но live mirror/publication path должен получить явные invariants на enriched profile, а не полагаться на downstream consumers.
- `publication.Snapshot` пока не стал полным single source of truth для publication результата: cleanup остаётся отдельным internal handle. Это допустимо для текущего slice, но при следующем rebaseline publication lifecycle надо решить, остаётся ли cleanup out-of-band или переезжает в snapshot contract.
- `runtimedelivery.AccessPlan` и `VerificationPlan` пока только planning contract. Реальной выдачи OCI/S3 credentials, digest enforcement в agent и runtime mutation в этом slice ещё нет.
- `runtimedelivery.AccessPlan` уже различает artifact classes, но fallback access modes (`DockerConfigSecret`, `PresignedURL`) и lifecycle-поля (`TTLSeconds`, `Audience`, `NeedsCleanup`) пока не materialize'ятся реальной логикой.
