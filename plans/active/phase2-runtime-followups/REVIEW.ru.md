# REVIEW

## Slice

Slice 8. Land resumable mirror-byte transport.

## Что проверено

- Изменение осталось в scope текущего bundle:
  - `sourcefetch`
  - `sourcemirror`
  - `publishworker`
  - `artifactcleanup`
  - structure/evidence docs
- Новый durable seam не создаёт второй published source of truth:
  - published truth остаётся `DMCR` + immutable OCI `ModelPack`
  - source mirror используется только как ingress/runtime-owned persisted mirror
- Cleanup ownership доведён до конца:
  - delete path удаляет registry metadata prefix
  - delete path удаляет source mirror prefix
- Текущий slice не вернул generic HTTP source и не размазал resume state по
  случайным пакетам.

## Проверки

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup ./internal/support/cleanuphandle ./internal/ports/sourcemirror`
- `make verify`
- `git diff --check`

## Findings

Blocking findings нет.

## Residual risk

- Mirror transport уже restart-safe, но пока остаётся последовательным:
  - нет parallel download по файлам
  - нет intra-file parallel chunking
  - нет throughput tuning
- Это не ломает correctness текущего slice, но остаётся следующим bounded
  performance/hardening workstream.

## Slice 9. Structural package-map cleanup

### Что проверено

- misleading K8s adapter name больше не конфликтует с реальными storage
  adapters:
  - old: `k8s/objectstorage`
  - new: `k8s/storageprojection`
- rename не меняет ownership:
  пакет остаётся только env/volume projection glue для pod shaping.
- `STRUCTURE.ru.md` снова описывает живое дерево, а не historical rename log,
  и больше не теряет `support/uploadsessiontoken/`.

### Findings

Blocking findings нет.

### Residual risk

- rename сам по себе не уменьшает byte-path complexity и не заменяет
  дальнейший source-ingest hardening;
- следующий structural drift теперь вероятнее всего будет уже не в naming, а в
  разрастании concrete adapter packages, если package map снова перестанут
  держать жёстко.

## Slice 10. Dead shared-result cleanup

### Что проверено

- `internal/ports/publishop` больше не держит мёртвый `Result`, который не
  участвовал в live runtime path;
- shared port package теперь несёт только реально используемые request/status
  contracts, а payload/result responsibility остаётся в:
  - `internal/publishedsnapshot/`
  - `internal/publicationartifact/`
- structural docs больше не описывают ложную общую boundary вокруг третьего
  `Result`.

### Findings

Blocking findings нет.

### Residual risk

- cleanup сам по себе не уменьшает adapter/package count;
- следующий structural drift надо продолжать давить по usage graph, а не по
  чисто текстовым rename-идеям.

## Slice 11. Source-mirror custom-CA trust fix

### Что проверено

- presigned multipart upload path больше не использует `http.DefaultClient`
  вслепую, если upload-staging adapter уже владеет CA-aware HTTP client;
- wiring не размазан через ad-hoc config duplicate:
  источник trust остаётся в S3 adapter, а source-mirror path только
  переиспользует тот же transport;
- regression coverage есть на двух уровнях:
  - custom-CA TLS presigned upload endpoint
  - publishworker wiring of the propagated HTTP client

### Findings

Blocking findings нет.

### Residual risk

- fix закрывает correctness на custom-CA endpoint, но не добавляет throughput
  tuning или file-level parallelism;
- live cluster still needs a fresh rollout before the `Gemma` smoke can be
  expected to pass with this fix.
