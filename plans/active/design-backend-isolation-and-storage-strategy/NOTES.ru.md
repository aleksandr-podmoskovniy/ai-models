# NOTES

- Workspace surface в upstream MLflow сам по себе не равен готовому RBAC и требует
  отдельного workspace provider / app-level ownership model.
- Официальная документация MLflow по workspaces подтверждает, что это отдельный
  capability backend, а не замена platform auth boundary:
  - https://mlflow.org/docs/latest/self-hosting/workspaces/
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
