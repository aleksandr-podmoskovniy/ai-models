# Durable Source Mirror

## Контекст

Текущий `HuggingFace` ingest path в phase-2 runtime остаётся
restart-unsafe:

- per-file `GET` во временный pod workspace;
- no `Range` resume;
- no persisted progress state;
- no durable source snapshot before `ModelPack` publication.

Это уже недостаточно для больших моделей и в будущем станет общим риском для
других registry-like sources.

## Решение

Canonical publication contract не меняем:

- published truth остаётся:
  - immutable OCI `ModelPack`
  - internal `DMCR`;
- runtime delivery остаётся отдельным concern.

Меняем ingress boundary:

- source ingest должен идти через **durable source mirror** в object storage;
- mirror хранит:
  - immutable snapshot manifest
  - mutable progress/state
  - затем mirrored files;
- local pod workspace перестаёт быть единственным source of truth для
  download progress.

## Нормализованный target flow

1. Resolve source snapshot
   - provider
   - external subject
   - exact resolved revision
   - file manifest
2. Persist mirror manifest outside pod
3. Persist mirror download state outside pod
4. Mirror bytes into object storage
5. Materialize bounded local workspace only after mirror completion
6. Validate/profile/package/publish into `DMCR`

## Первый landed slice

Первый bounded slice не пытается сразу заменить весь download path.

Он вводит:

- отдельный `sourcemirror` port contract;
- object-storage-backed manifest/state adapter;
- path semantics, которые можно потом переиспользовать и для `HuggingFace`, и
  для других registry-like sources.

Это нужно, чтобы resume logic не оказался снова размазан между:

- `sourcefetch`
- `publishworker`
- `uploadstaging`

без явного runtime seam.

## Следующий landed slice

Второй bounded slice уже использует этот seam в live byte path:

- `HuggingFace` bytes mirrorятся в object storage до local materialization;
- mirror upload идёт через object-storage multipart upload;
- retry после pod restart продолжает download через `Range` от уже
  подтверждённого byte offset;
- local checkpoint materialization идёт уже из mirrored files, а не из
  единственной pod-local truth;
- backend cleanup ownership теперь удаляет и registry metadata, и source mirror
  prefix.

Что ещё сознательно не claimed:

- file-level parallelism;
- intra-file parallel chunking;
- throughput tuning и background prefetch.
