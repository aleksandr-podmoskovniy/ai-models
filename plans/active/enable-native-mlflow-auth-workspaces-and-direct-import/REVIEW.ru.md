# REVIEW

## Findings

Критичных замечаний по текущему slice не осталось.

## Проверка scope

- Backend переведён на upstream-native MLflow basic auth и workspaces без
  отдельного custom auth layer.
- Heavy HF import path оставлен внутри кластера и переведён на direct artifact
  access через `--no-serve-artifacts`.
- Values/OpenAPI, templates, backend image scripts, fixtures и docs обновлены
  согласованно.

## Проверки

- `python3 -m py_compile images/backend/scripts/ai-models-backend-render-db-uri.py images/backend/scripts/ai-models-backend-render-auth-config.py images/backend/scripts/ai-models-backend-hf-import.py tools/upload_hf_model.py`
- `bash -n tools/run_hf_import_job.sh`
- `make lint`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`

## Residual risks

- Текущий phase-1 auth baseline даёт native backend isolation и workspace
  boundary, но user/group provisioning поверх MLflow auth пока не platformized;
  bootstrap идёт через internal admin secret.
- Direct artifact access теперь быстрее и ближе к upstream, но для очень
  больших моделей всё ещё нужно следить за `ephemeral-storage` import Job.
- Переход с ingress-only auth на native MLflow auth меняет пользовательский
  login flow; эксплуатационные docs это отражают, но rollout стоит проверить на
  живом кластере отдельно.

## Reviewer

Режим задачи `solo`, дополнительный `reviewer` pass не обязателен.
