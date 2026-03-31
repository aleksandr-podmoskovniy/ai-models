# REVIEW

## Findings
- Критичных замечаний по текущему slice нет.

## Что проверено
- user-facing contract не раздут: наружу добавлен только `artifacts.caSecretName`,
  а reuse `credentialsSecretName` для `ca.crt` остаётся fallback, а не новым
  параллельным storage path;
- backend launcher и in-cluster HF import Job используют один и тот же trust
  material contract через `AI_MODELS_S3_CA_FILE` и merged trust bundle;
- steady-state path остаётся phase-1-friendly: direct-to-S3 import не требует
  новых cluster-wide hooks, CRD или публичных backend entities;
- прогнаны:
  - `python3 -m py_compile images/backend/scripts/ai_models_backend_runtime.py images/backend/scripts/ai-models-backend-hf-import.py`
  - `bash -n tools/run_hf_import_job.sh`
  - `make helm-template`
  - `make verify`

## Residual risks
- чтобы warnings реально исчезли на кластере, нужен новый rollout модуля и
  перевод runtime на `artifacts.insecure: false` вместе с корректным `ca.crt`
  Secret в `d8-ai-models`;
- fallback на `credentialsSecretName` предполагает, что existing Secret уже
  содержит `ca.crt`; если нет, нужно задать отдельный `artifacts.caSecretName`;
- текущий slice не проверяет live S3 handshake сам по себе: окончательный
  сигнал появится только после повторного запуска import Job на кластере.
