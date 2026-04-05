# Implement Safe HTTP Source Publication

## Контекст

Текущий phase-2 controller уже работает по `pod/session`-архитектуре:

- `modelpublish` владеет только public lifecycle и status;
- `publicationoperation` владеет durable execution boundary;
- live path уже есть для `HuggingFace` и `Upload(HuggingFaceDirectory)`;
- public API уже допускает `spec.source.type=HTTP`.

До этого `HTTP` оставался intentionally disabled, потому что предыдущий live
path оказался небезопасным: archive extraction в backend worker была уязвима к
link/path escape и не подходила для production baseline.

## Постановка задачи

Добрать следующий bounded slice:

1. жёстко безопасно включить live `HTTP` path на текущей `pod/session`
   architecture;
2. не возвращаться к fat reconciler или batch-job semantics;
3. не тащить в этот slice `authSecretRef`, `KitOps/OCI` или новый runtime
   materializer.

## Scope

- зафиксировать bundle под safe `HTTP` source slice;
- harden-ить backend archive extraction для HTTP/upload archive reuse;
- включить `HTTP` source в current worker pod path;
- передавать `http.url` и optional `http.caBundle` в backend worker;
- обновить tests и docs под новый live scope.

## Non-goals

- Не реализовывать `http.authSecretRef` в этом slice.
- Не менять current backend artifact plane на `KitOps` / OCI.
- Не расширять `Upload` beyond current `HuggingFaceDirectory` scope.
- Не переделывать public status shape.
- Не чинить здесь старый `ai-models-backend-hf-import.py`.

## Затрагиваемые области

- `images/backend/scripts/*source-publish*`
- `images/controller/internal/sourcepublishpod/*`
- `images/controller/internal/publicationoperation/*`
- `images/controller/README.md`
- `docs/CONFIGURATION*`
- `plans/active/implement-safe-http-source-publication/*`

## Критерии приёмки

- `spec.source.type=HTTP` больше не падает сразу как intentionally disabled.
- Controller создаёт worker `Pod` для `HTTP` source без special casing в public
  reconciler.
- Backend worker безопасно распаковывает tar/zip archive без link/path escape.
- `HTTP` source с optional `caBundle` доходит до current artifact plane и пишет
  structured result в operation `ConfigMap`.
- `http.authSecretRef` при наличии завершается явным controlled failure, а не
  silent ignore.
- Узкие tests для controller и backend helper проходят.

## Риски

- Слишком широкий security slice быстро расползётся в полноценный downloader
  subsystem; здесь нужен минимальный production-worthy hardening, не redesign.
- Если quietly включить `HTTP`, но оставить `extractall`, мы зацементируем
  небезопасный baseline.
- `Upload` переиспользует тот же archive-unpack helper, так что правка должна
  не сломать текущий live upload path.
