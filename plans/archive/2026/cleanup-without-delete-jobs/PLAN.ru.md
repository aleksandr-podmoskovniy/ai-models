# План: cleanup без per-delete Jobs

## Current phase

Этап 1: publication/runtime baseline. Задача укрепляет controller-owned
publication lifecycle и не добавляет phase-2 topology.

## Orchestration

Режим `solo`: текущий slice узко ограничен delete cleanup lifecycle, а
сравнение с `virtualization` уже даёт целевой паттерн. Сабагенты в текущем
запросе не требовались, поэтому не запускаются.

## Slices

1. Зафиксировать virtualization baseline.
   - Файлы: `plans/active/cleanup-without-delete-jobs/NOTES.ru.md`.
   - Проверка: notes описывают, почему per-delete Job удаляется.
   - Результат: documented decision.

2. Ввести in-process cleanup operation и persistent completion marker.
   - Файлы: `cleanupstate`, `artifactcleanup`, `catalogcleanup`,
     `application/deletion`.
   - Проверки: targeted Go tests для deletion decision и cleanup state.
   - Результат: delete reconcile выполняет cleanup без Kubernetes Job.

3. Обновить controller wiring и templates.
   - Файлы: `cmd/ai-models-controller`, `templates/controller`,
     `tools/helm-tests`.
   - Проверки: `make helm-template`, template validation.
   - Результат: controller получает нужный internal registry auth/CA без
     cleanup-job runtime surface.

4. Удалить устаревший job-specific код и тесты.
   - Файлы: `catalogcleanup/job.go`, job-specific tests/config references.
   - Проверки: `go test` по controller packages.
   - Результат: кодовая база меньше и не содержит active per-delete Job path.

## Rollback point

До удаления job-specific файлов можно вернуть старую ветку через `Job` path,
оставив completion marker неиспользованным. После удаления rollback делается
восстановлением `catalogcleanup/job.go` и прежнего `CleanupJobOptions`.

## Final validation

- `go test ./internal/controllers/catalogcleanup ./internal/application/deletion ./internal/adapters/k8s/cleanupstate ./internal/dataplane/artifactcleanup`
- `make helm-template`
- `make kubeconform`
- `git diff --check`
