# NOTES

- Workspace surface в upstream MLflow сам по себе не равен готовому RBAC и требует
  отдельного workspace provider / app-level ownership model.
- Официальная документация MLflow по workspaces подтверждает, что это отдельный
  capability backend, а не замена platform auth boundary:
  - https://mlflow.org/docs/latest/self-hosting/workspaces/
- Официальная документация MLflow по SSO подтверждает, что правильный path для
  user identity внутри backend — это app-native OIDC login flow, а не просто
  reverse proxy перед UI:
  - https://mlflow.org/docs/latest/self-hosting/security/sso/
- Deckhouse reference `istio` показывает другой паттерн:
  - `DexAuthenticator` + ingress external auth + allowed groups;
  - это хороший UI gate, но не app-native authz model внутри backend.
- Для `ai-models` важно не смешивать:
  - cluster-level SSO gate;
  - app-native user identity;
  - workspace membership sync.
- Официальная документация MLflow по basic auth подтверждает, что auth baseline
  конфигурируется через `basic_auth.ini` / `MLFLOW_AUTH_CONFIG_PATH`, а users /
  permissions дальше уже управляются через Python/REST API (`AuthServiceClient`):
  - https://mlflow.org/docs/latest/self-hosting/security/basic-http-auth/
- Официальная документация MLflow по workspaces подтверждает, что:
  - workspace selection есть в Python API (`mlflow.set_workspace()`,
    `mlflow.list_workspaces()`);
  - workspace permissions управляются через workspace API;
  - сами workspaces не являются hard isolation boundary:
  - https://mlflow.org/docs/latest/self-hosting/workspaces/
  - https://mlflow.org/docs/latest/self-hosting/workspaces/getting-started/
- Для больших HF-моделей важно не путать:
  - “не тащить data plane через ноутбук”;
  - “вообще не иметь локального spool/cache на import worker”.
- Официальная документация Hugging Face Hub подтверждает, что
  `snapshot_download()` уже использует concurrent download workers, а для
  ускорения крупных загрузок рекомендует `hf_xet`:
  - https://huggingface.co/docs/huggingface_hub/guides/download
  - https://huggingface.co/docs/huggingface_hub/guides/download#faster-downloads
- KServe важно рассматривать как consumer/serving plane, а не как замену registry.
- Официальная документация KServe подтверждает широкий storage surface для model
  serving, включая `storageUri`, PVC, Hugging Face Hub и OCI/Modelcar:
  - https://kserve.github.io/website/docs/model-serving/storage/overview
  - https://kserve.github.io/website/docs/model-serving/storage/providers/huggingface
  - https://kserve.github.io/website/docs/model-serving/generative-inference/modelcache/localmodel
