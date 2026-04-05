# REVIEW

## Рекомендация

### OCI / payload-registry

Базовый и предпочтительный путь:

- principal для доступа к артефакту должен быть Kubernetes `ServiceAccount`
  materialization agent;
- authorization должен оставаться registry-native и repo-scoped через
  `payload-registry` policy model, а не через отдельные object-storage creds;
- для пользователя и runtime это должно выглядеть как обычный in-cluster
  workload access path: pod использует свой `ServiceAccount`, а controller
  привязывает этот identity к нужному OCI repository.

Практический вывод:

- public contract не должен содержать registry credentials;
- `Model` / `ClusterModel` должны содержать только published OCI artifact ref;
- controller должен materialize access grant отдельно от public API;
- если registry client умеет работать с bearer identity этого registry, агент
  должен использовать short-lived pod/service-account identity;
- если tooling требует docker-style auth config, controller может выдавать
  short-lived `dockerconfigjson`, но только как internal compatibility layer и
  только для agent pod.

### S3-compatible storage

Базовый и предпочтительный путь:

- runtime container не должен получать постоянные S3 credentials;
- доступ к S3 нужен только materialization agent;
- credentials должны выдаваться динамически во время работы агента и иметь
  короткий TTL.

Предпочтительный auth model:

- projected `ServiceAccount` token -> STS / web identity exchange ->
  short-lived S3 credentials;
- scope credentials должен ограничиваться конкретным bucket/prefix/object set,
  который нужен для одной model materialization operation.

Fallback path, если object store не умеет federation:

- controller выдает presigned URL или ephemeral Secret для agent pod;
- secret живёт ограниченное время и удаляется после materialization;
- этот путь хуже, чем STS/web identity, но всё ещё лучше статических
  долгоживущих access key.

## Public contract vs internal detail

В public contract стоит фиксировать только:

- artifact kind/class (`OCI` или `ObjectStorage`);
- stable artifact URI/ref;
- digest и metadata/profile;
- readiness/conditions.

Не стоит фиксировать в public contract:

- registry login details;
- S3 access key / secret / session token;
- MLflow run/workspace/logged model;
- `PayloadRepositoryAccess` names;
- конкретную форму runtime injection (`init-container`, `sidecar`, agent).

## Почему так

- Для OCI/payload-registry локальный анализ уже показал, что сильная сторона
  этого backend — Kubernetes token auth и `PayloadRepositoryAccess`, а не
  внешние bucket policy.
- В virtualization похожий operational pattern уже используется: controller
  on-demand раздаёт auth material в нужный pod namespace и потом чистит его.
- Official Kubernetes guidance рекомендует short-lived projected
  `ServiceAccount` tokens вместо legacy static secret tokens.
- Official AWS/MinIO guidance подтверждает, что правильный путь для S3-like
  access — временные credentials через web identity / STS, а не вручную
  разложенные долгоживущие ключи.

## Residual risks

- Для payload-registry ещё надо подтвердить live client path: сможет ли наш
  конкретный materializer tooling ходить по service-account-based auth без
  промежуточного dockerconfigjson compatibility layer.
- Для S3-compatible backend federation story зависит от конкретного storage:
  AWS-style IRSA path стандартный, а для MinIO/других S3-compatible систем надо
  отдельно подтвердить usable OIDC/STS integration.
- Если federation path недоступен, presigned URL и ephemeral Secret остаются
  допустимым fallback, но не желательным базовым contract.

## Локальные референсы

- `virtualization` dataSource / upload / target pattern:
  [virtual_image.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/api/core/v1alpha2/virtual_image.go)
- `virtualization` on-demand DVCR auth distribution:
  [dvcr_auth.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/docs/internal/dvcr_auth.md)
- `payload-registry` evaluation summary:
  [REVIEW.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/plans/active/evaluate-kitops-with-dkp-registry/REVIEW.ru.md)
