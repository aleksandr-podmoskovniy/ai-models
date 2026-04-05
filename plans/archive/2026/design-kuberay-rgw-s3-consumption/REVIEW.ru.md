# Review

## Findings

- Критичных замечаний к выбранной границе нет: `ai-models` остаётся источником `artifactURI`, а `KubeRay` рассматривается только как внешний consuming serving plane.
- Текущий `ServiceAccount` в chart не решает задачу доступа к RGW; это чисто Kubernetes RBAC для monitoring/Grafana и не конфликтует с предложением использовать отдельный read-only S3 principal.
- Для current phase наиболее практичен статический read-only S3 credential path с `AWS_*` + `AWS_CA_BUNDLE` + shared AWS config для `path-style`.

## Missing checks

- Не выполнен live rollout внешнего `RayService` chart с `bucket_uri` из MLflow.
- Не подтверждён вживую `Dex -> RGW STS/WebIdentity` path; он должен считаться future hardening/PoC, а не базовым контрактом.

## Residual risks

- Если конкретная сборка `rayproject/ray-llm:2.54.0-py311-cu128` внутри image использует не тот AWS SDK path, который ожидается по документации, может понадобиться дополнительный smoke-test на custom endpoint + `path-style`.
- Для multimodal checkpoint'ов HF-compatible `model/` layout в S3 не гарантирует runtime-совместимость с текущим `Ray Serve LLM`; storage wiring и model/runtime compatibility нужно проверять отдельно.
