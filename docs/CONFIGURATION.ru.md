---
title: "Конфигурация"
menuTitle: "Конфигурация"
weight: 60
---

<!-- SCHEMA -->

Текущий конфигурационный контракт `ai-models` намеренно короткий.
На уровне модуля наружу выставляются только стабильные ai-models-specific настройки:
логирование, настройки Deckhouse SSO, wiring для PostgreSQL и S3-compatible artifact
storage, а также wiring для phase-2 publication OCI registry.

`postgresql.mode` поддерживает два phase-1 сценария:

- `Managed`: поднять внутренний PostgreSQL через Deckhouse `managed-postgres`;
- `External`: подключить ai-models к существующему PostgreSQL, используя пароль
  из уже созданного Secret.

Базовый managed profile намеренно маленький: по умолчанию он использует уже
существующий cluster-wide `PostgresClass`, запрашивает том на 5Gi и держит
минимальный resource profile для phase-1 metadata storage.
Имена database и user по умолчанию теперь `ai-models`, а HA topology для
managed `Postgres` берётся из `PostgresClass.defaultTopology`, а не
хардкодится на стороне модуля.
Для browser SSO и MLflow permissions модуль также использует отдельную logical
auth database в том же PostgreSQL instance. В режиме `Managed` модуль создаёт
эту вторую БД автоматически по шаблону `<database>-auth`. В режиме `External`
existing PostgreSQL должен уже содержать такую вторую БД.

`artifacts` определяет S3-compatible backend для артефактов ai-models: bucket,
path prefix, endpoint URL, region, TLS policy, addressing style и credentials.

Учётные данные для artifact storage можно задавать двумя способами:

- через `credentialsSecretName`, указывающий на уже существующий Secret в
  `d8-ai-models` с фиксированными ключами `accessKey` и `secretKey`;
- через inline `accessKey` и `secretKey` в ModuleConfig, после чего модуль сам
  создаёт внутренний Secret в `d8-ai-models`.

Custom CA для S3-compatible endpoint задаётся отдельно через
`artifacts.caSecretName`. Этот Secret должен находиться в `d8-ai-models` и
содержать ключ `ca.crt`. Если `caSecretName` пустой, ai-models сначала
автоматически reuse'ит `credentialsSecretName`, если тот же Secret также
содержит `ca.crt`, а иначе fallback'ится на общий platform CA, который уже
discovered для Dex или скопирован из global HTTPS `CustomCertificate` path.

`bucket`, `pathPrefix`, `endpoint`, `region` и флаги addressing/TLS не считаются
секретами и остаются обычной частью module configuration contract.

Для phase 2 используется отдельный module config `publicationRegistry`. Он
задаёт controller-owned OCI repository prefix для опубликованных артефактов
`Model` / `ClusterModel` и credentials, которые worker Pods используют для
текущего implementation adapter при login, push и inspect remote
`ModelPack`-artifacts.

Учётные данные publication registry можно задавать двумя способами:

- через `credentialsSecretName`, указывающий на уже существующий Secret в
  `d8-ai-models` с фиксированными ключами `username` и `password`;
- через inline `username` и `password` в ModuleConfig, после чего модуль сам
  создаёт внутренний Secret в `d8-ai-models`.

`publicationRegistry.caSecretName` может указывать на Secret в `d8-ai-models`
с ключом `ca.crt` для private registry trust. Если он не задан, ai-models
fallback'ится только на общий platform CA, который уже discovered для Dex или
скопирован из global HTTPS `CustomCertificate` path.
`publicationRegistry.insecure` поддерживается только как troubleshooting path
для plain-HTTP или broken-TLS lab registry и не считается целевым
steady-state режимом.

Режим доступности и HTTPS policy берутся из global Deckhouse configuration и
internal module wiring.
Текущий runtime ожидает:

- настроенный `global.modules.publicDomainTemplate`;
- глобально включённый HTTPS через Deckhouse module HTTPS policy
  (`CertManager` или `CustomCertificate`);
- модуль `managed-postgres`, если `postgresql.mode=Managed`.

Browser login теперь идёт через Deckhouse Dex OIDC SSO внутри самого MLflow.
Модуль автоматически настраивает:

- `DexClient` в `d8-ai-models` с redirect URI `https://<public-host>/callback`;
- public Dex discovery URL `https://dex.<cluster-domain>/.well-known/openid-configuration`;
- автоматическое platform CA trust wiring из discovered Dex CA или global HTTPS
  `CustomCertificate` path для TLS OIDC и S3;
- вход в MLflow через `mlflow-oidc-auth`;
- upstream-native MLflow workspaces.

Настройки `auth.sso.allowedGroups` и `auth.sso.adminGroups` определяют, какие
Deckhouse группы вообще могут заходить в ai-models и какие из них становятся
MLflow administrators после SSO login. Базовый default намеренно
консервативен: внутрь допускается только группа Deckhouse `admins`, и она же
становится MLflow admin group.

Модуль всегда создаёт внутренний auth Secret со:

- internal machine username в ключе `machineUsername`;
- стабильным сгенерированным machine password в ключе `machinePassword`;
- стабильным session secret для MLflow auth runtimes.

Этот Secret теперь остаётся только machine-only путём для `ServiceMonitor`,
in-cluster import Jobs и break-glass operations, а browser users идут через Dex SSO.

Из-за этого raw backend service больше не защищён только на ingress-уровне.
Даже при прямом доступе к service нужны MLflow machine credentials, а логическая
сегментация по-прежнему идёт через native MLflow workspaces.

Большие machine-oriented import flows теперь используют direct artifact access
вместо server-side artifact proxying. Backend запускается с
`--no-serve-artifacts`, а in-cluster import Jobs ходят в MLflow metadata APIs,
но пишут artifacts напрямую в S3. Backend и import Jobs используют один и тот
же merged trust bundle для Dex OIDC и S3 CA overrides, поэтому
`artifacts.insecure: true` остаётся только временным troubleshooting path, а не
целевым steady-state режимом.

Для phase 2 controller теперь владеет publication/runtime adapters для первых
live source paths. `Model` / `ClusterModel` с `spec.source.url` проходят через
controller-owned worker Pods, которые сами определяют, является ли ссылка
Hugging Face-источником или обычным HTTP-адресом, скачивают принятый source, генерируют
model-package description, упаковывают checkpoint в `ModelPack` через текущий
implementation adapter, пушат итоговый artifact в module-owned OCI publication
plane, инспектируют remote manifest и только после этого пишут в public
`status` ссылку на сохранённый artifact и обогащённый technical profile. Текущий
live scope для `HTTP` намеренно узкий: ожидается
archive с Hugging Face-compatible checkpoint, требуется
`spec.runtimeHints.task`, поддерживается inline `caBundle`, а `authSecretRef`
теперь проходит через controller-owned projection. Для `HuggingFace`
controller принимает source secret с одним из ключей `token`, `HF_TOKEN` или
`HUGGING_FACE_HUB_TOKEN` и нормализует его в projected worker token. Для
`HTTP` controller принимает либо `authorization`, либо пару
`username`+`password` и проецирует только эти ключи в worker namespace.
Controller-owned publication worker жёстко harden'ит tar/zip extraction и
отклоняет path traversal, symlink, hard link и другие специальные archive
entries вместо raw `extractall`.

`spec.source.upload` теперь идёт через controller-owned session flow, а не
через batch import. Controller создаёт worker Pod, ClusterIP Service и
short-lived auth Secret, после чего пишет helper command в
`status.upload.command` для local-machine upload через `kubectl port-forward`.
Текущий live controller path принимает:

- для `Safetensors`: архив с `config.json` и файлами весов;
- для `GGUF`: либо прямой `.gguf` файл, либо архив.

Дальше controller публикует их в тот же controller-owned `ModelPack`/OCI
artifact plane через текущий Go dataplane и `ModelPack` adapter.

`spec.inputFormat` трактуется как source-agnostic contract для валидации
состава модели на входе, а не как формат финального registry artifact.
Финальная опубликованная форма скрыта и фиксирована: `ModelPack` в OCI.
Независимо от того, пришли ли байты из Hugging Face, HTTP или локального
upload, controller теперь валидирует и санитизирует состав проекта до
packaging. Текущие live правила такие:

- `Safetensors`: нужен root `config.json`, хотя бы один `.safetensors`, разрешён
  известный config/tokenizer/index companion set, benign extras вроде
  `README.md` и картинок вычищаются, активные или неоднозначные файлы вроде
  `.py`, `.sh`, `.dll`, `.so` и прочий unsupported payload отклоняются.
- `GGUF`: нужен хотя бы один `.gguf`, benign extras вычищаются, те же активные
  или неоднозначные payload'ы отклоняются.

Для generic `HTTP` это означает:

- `Safetensors` сейчас ожидает архив;
- `GGUF` может приехать как прямой файл или как архив.

Если `spec.inputFormat` не указан, controller сначала пытается определить
формат сам:

- `GGUF` — по наличию `.gguf`;
- `Safetensors` — по `config.json` и `.safetensors`.

Если формат определить однозначно не удалось, публикация fail-closed
останавливается с ошибкой и требует явного `spec.inputFormat`.

После валидации controller дополнительно обогащает metamodel:

- для `Safetensors`
  - читает `config.json`
  - ищет размер контекстного окна по известным ключам
  - считает `parameterCount` сначала по явным полям, затем по размерам
    `.safetensors` shards
  - определяет `quantization` и `compatiblePrecisions`
  - строит `supportedEndpointTypes` из `task`
  - строит `minimumLaunch` как GPU baseline по реальному размеру весов
- для `GGUF`
  - читает имя и размер `.gguf` файла
  - выделяет family, quantization и приблизительный `parameterCount`
  - строит `supportedEndpointTypes` из `task`
  - строит `minimumLaunch` как GPU baseline по реальному размеру файла и
    quantization

Destructive cleanup тоже остаётся явным machine-oriented workflow. Phase-2
controller теперь хранит только internal backend cleanup handle и запускает
controller-owned one-shot Jobs через subcommand `artifact-cleanup` в
dedicated runtime image. Текущий live cleanup path логинится в publication OCI
registry с тем же controller-owned trust и credentials wiring и удаляет remote
artifact по сохранённой ссылке, не выводя backend internals в public status.

HF import path также оставляет в MLflow осмысленную metadata для production UX:

- run получает HF-related params и tags;
- registered model и model version получают description и tags;
- в run artifacts логируются `hf/model-info.json`,
  `hf/snapshot-manifest.json`, а также доступные `config.json`,
  `generation_config.json`, `tokenizer_config.json` и `model-card.md`.

Это не делает MLflow UI браузером сырых объектов в S3: интерфейс всё ещё
показывает только то, что importer явно залогировал как MLflow metadata и
artifacts. Для multimodal task types schema в UI по-прежнему зависит от
upstream `mlflow.transformers` support и может оставаться пустой без
task-specific signature.

Текущий phase-1 runtime profile намеренно консервативен:
каждый backend pod запускает один MLflow web worker, а MLflow server job
execution отключён. High availability backend достигается через Deckhouse
module HA и несколько pod replicas, а не через лишние in-process workers и
genai job consumers.

Backend также оставляет включённым upstream security middleware MLflow.
Модуль вычисляет `allowed-hosts` и same-origin CORS policy от публичного
ingress domain и при этом сохраняет private-network/service паттерны, нужные
для внутрикластерного доступа. Health probes используют upstream
неаутентифицированный `/health`, а `ServiceMonitor` ходит в `/metrics` через
внутренний machine account.

Модуль также создаёт внутренний Secret со стабильным значением
`MLFLOW_CRYPTO_KEK_PASSPHRASE` для upstream crypto-backed runtime features
MLflow. Это убирает небезопасный upstream default passphrase из shared cluster
deployments и при этом не выводит KEK в user-facing contract модуля.

`Model` и `ClusterModel` теперь входят в installation lifecycle модуля как CRD
и controller runtime wiring. Но их publication UX и финальный public contract
всё ещё находятся в активной phase-2 разработке, поэтому текущий API нужно
считать evolving, а не стабильным.
