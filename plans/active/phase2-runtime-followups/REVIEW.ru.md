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

## Slice 12. Controller entrypoint shell split

### Что проверено

- `cmd/ai-models-controller` больше не держит env contract, quantity parsing и
  bootstrap option shaping в одном oversized `run.go`;
- новые files имеют defendable boundaries:
  - `env.go` — process env contract and pass-through helpers
  - `config.go` — parsed manager config and bootstrap option shaping
  - `resources.go` — publication worker resource/work-volume parsing
  - `run.go` — thin execution flow
- split не меняет runtime semantics controller startup path.

### Findings

Blocking findings нет.

### Residual risk

- split уменьшает monolith, но не заменяет будущую repo-wide structural ревизию
  за пределами controller runtime;
- отдельный runtime delivery wiring workstream всё ещё остаётся открытым.

## Slice 13. Upload-session service split

### Что проверено

- `internal/adapters/k8s/uploadsession/service.go` больше не смешивает
  orchestration, secret lifecycle и handle/token projection в одном file;
- новые files имеют defendable boundaries:
  - `service.go` — constructor plus `GetOrCreate` orchestration
  - `lifecycle.go` — session secret lifecycle and explicit expiration sync
  - `handle.go` — runtime handle shaping and active token resolution
- split не вернул controller-local обход `uploadsession` seam через прямые
  `Secret` mutations outside adapter package.

### Findings

Blocking findings нет.

### Residual risk

- package стал чище, но сам `uploadsession` boundary остаётся важным K8s
  runtime hotspot и дальше не должен обрастать adapter-local policy;
- следующий cleanup надо продолжать по usage graph, а не превращать текущий
  split в повод для искусственного дробления package.

## Slice 14. Sourcefetch archive/materialization split

### Что проверено

- `internal/adapters/sourcefetch/archive.go` больше не смешивает archive
  dispatch, extraction safety и single-file materialization в одном file;
- новые files имеют defendable boundaries:
  - `archive.go` — input dispatch plus archive entrypoint
  - `archive_extract.go` — tar/zip extraction safety and extracted-root logic
  - `materialize.go` — single-file materialization and file IO helpers
- split не меняет acquisition/runtime semantics `PrepareModelInput`.

### Findings

Blocking findings нет.

### Residual risk

- `sourcefetch/` всё ещё остаётся крупным adapter boundary и дальше не должен
  превращаться в контейнер для format/status policy;
- следующий cleanup уже надо выбирать по usage graph внутри `huggingface.go`
  или на repo-wide structural surface, а не дробить archive path бесконечно.

## Slice 15. Sourcefetch HuggingFace split

### Что проверено

- `internal/adapters/sourcefetch/huggingface.go` больше не смешивает HF info
  API helpers, snapshot orchestration и staging/materialization в одном file;
- новые files имеют defendable boundaries:
  - `huggingface.go` — top-level HF fetch orchestration
  - `huggingface_info.go` — HF info API and repo/revision helpers
  - `huggingface_snapshot.go` — snapshot staging/materialization helpers
- split не меняет current public HF source contract или source-mirror behavior.

### Findings

Blocking findings нет.

### Residual risk

- `sourcefetch/` всё ещё остаётся крупным acquisition boundary и следующий
  cleanup надо уже выбирать по usage graph внутри mirror transport or broader
  repo structure;
- live `Gemma` validation всё ещё требует fresh rollout, этот slice сам по
  себе cluster proof не заменяет.
