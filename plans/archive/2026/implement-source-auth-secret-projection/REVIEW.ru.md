# REVIEW

## Что сделано

- `application/publication` больше не режет `HuggingFace.authSecretRef` и
  `HTTP.authSecretRef` как `not implemented`: planner теперь резолвит
  `SourceAuthSecretRef`, нормализует namespace и fail-closed отклоняет
  cross-namespace refs для namespaced `Model`.
- `sourcepublishpod` получил controller-owned auth projection:
  - controller читает source Secret из source namespace;
  - копирует только минимально нужные keys в worker namespace;
  - `HuggingFace` нормализуется в projected key `token`;
  - `HTTP` принимает `authorization` или `username` + `password`.
- Worker Pod contract обновлён:
  - `HuggingFace` получает `HF_TOKEN` из projected Secret;
  - `HTTP` получает `--http-auth-dir` и mounted projected Secret directory.
- Backend worker `ai-models-backend-source-publish.py` теперь понимает
  `--http-auth-dir` и использует тот же minimal HTTP auth directory contract.
- Projected auth Secret удаляется через `sourcepublishpod.Service.Delete`, а
  owner reference на publication operation остаётся safety net.
- README и user-facing docs больше не утверждают, что `authSecretRef` остаётся
  controlled failure.

## Проверки

- `go test ./internal/application/publication ./internal/sourcepublishpod ./internal/publicationoperation` в `images/controller`
- `python3 -m py_compile images/backend/scripts/ai-models-backend-source-publish.py`
- `python3 -m unittest discover -s images/backend/scripts -p 'test_*.py'`
- controller quality gates
- `go test ./...` в `images/controller`
- `make helm-template`
- `make verify` запущен, но не удалось завершить честно: в этой среде процесс
  зависает на `kubeconform`, поэтому итоговый repo-level green signal по нему не
  подтверждён
- `git diff --check`

## Остаточные риски

- Текущий `HuggingFace` ingress contract допускает alias keys
  `token` / `HF_TOKEN` / `HUGGING_FACE_HUB_TOKEN`, но projected contract
  намеренно нормализуется только к `token`.
- `HTTP` slice всё ещё узкий: archive-based source, required
  `spec.runtimeHints.task`, optional `caBundle`, minimal auth keys only.
- Runtime materializer / `kitinit` path остаётся следующим workstream и не
  входит в этот slice.
