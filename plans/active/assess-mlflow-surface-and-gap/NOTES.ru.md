# NOTES: реальный MLflow surface и gap phase-1

## Cluster-side факты

### SSO path

- `https://ai-models.k8s.apiac.ru/` отвечает `302` на
  `/dex-authenticator/sign_in?...`.
- В namespace `d8-ai-models` есть `DexAuthenticator/ai-models` с:
  - `applicationDomain: ai-models.k8s.apiac.ru`
  - `highAvailability: true`
  - `sendAuthorizationHeader: true`
- `DexProvider` в кластере сейчас отсутствует (`items: []`).
- При этом в кластере есть локальные Deckhouse `User`/`Group` объекты, значит
  текущий login path может работать через cluster-local Deckhouse auth, даже без
  внешнего OIDC provider.

Вывод:
- ingress-level SSO gate работает;
- внешний OIDC provider cluster-wide ещё не подключён;
- parity с `n8n` по app-native OIDC client сейчас отсутствует.

### Runtime surface, реально видимый в backend

По live логам backend уже обслуживает:
- `runs/search`
- `registered-models/search`
- `logged-models/search`
- `traces/*`
- `datasets/search`
- `scorers/list`
- `gateway/endpoints/*`
- `gateway/secrets/config`

В логах также есть предупреждение:
- `MLFLOW_CRYPTO_KEK_PASSPHRASE not set`

Вывод:
- большой upstream surface действительно live в runtime;
- часть возможностей просто видна, но ещё не оформлена как supported contract;
- gateway secrets сейчас не должны считаться production-ready без отдельного
  hardening.

## Repo-side факты

### Что модуль реально wiring'ит

- ingress-level Dex SSO через `DexAuthenticator` и ingress auth annotations;
- global HTTPS/certificate policy от Deckhouse;
- managed/external PostgreSQL wiring;
- S3-compatible artifact storage wiring;
- базовый monitoring/logging;
- conservative backend runtime profile:
  - `workers=1`
  - `MLFLOW_SERVER_ENABLE_JOB_EXECUTION=false`

### Что модуль не делает

- не делает app-native OIDC client bootstrap как `n8n`;
- не делает platform-level governance/ownership для gateway/prompts/scorers/traces;
- не скрывает автоматически весь лишний upstream UI surface.

## Upstream MLflow факты

### Hugging Face / model registry

Upstream уже содержит:
- `mlflow.transformers.*`
- `mlflow.register_model(...)`
- `persist_pretrained_model(...)`
- Hugging Face dataset abstractions
- gateway provider для Hugging Face TGI

Практический вывод:
- штатный путь для Hugging Face model weights в `MLflow` — программный, а не
  browser-driven UI upload;
- типичный сценарий:
  1. `mlflow.transformers.log_model(...)`
  2. при необходимости `mlflow.transformers.persist_pretrained_model(...)`
  3. `mlflow.register_model(...)`

### Auth surface

Upstream auth app поддерживает `basic-auth`.
Для FastAPI routes (включая `/gateway/`) upstream прямо пишет, что custom
`authorization_function` не поддерживается, и там работает только default Basic
Auth path.

Вывод:
- делать parity с `n8n` через чистый upstream без кастомизации нельзя;
- если держим линию "без кастомов поверх upstream", остаёмся на ingress-level
  SSO через Dex.
