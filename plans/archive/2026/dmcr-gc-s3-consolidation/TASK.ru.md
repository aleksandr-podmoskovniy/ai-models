# DMCR GC S3 helper consolidation

## 1. Заголовок

Сократить и упростить S3 inventory/pagination код внутри `dmcr` garbage
collection без изменения GC policy и публичных контрактов.

## 2. Контекст

Предыдущий slimming bundle завершён и архивирован как handoff. Следующий
исполняемый slice из него — package-local consolidation в
`images/dmcr/internal/garbagecollection`: S3 object listing, multipart upload
listing и multipart part listing повторяют одинаковый pagination/error
handling skeleton.

Это stateful storage/GC код, поэтому сокращение допустимо только при сохранении
текущих fail-closed правил, direct-upload cleanup semantics, lease/gate/quorum
поведения и sealed S3 boundary.

## 3. Постановка задачи

Сделать один bounded cleanup/hardening slice:

- найти и схлопнуть повторяющиеся S3 pagination/helper конструкции внутри
  `garbagecollection`;
- добавить fail-closed forward-progress guards для S3-compatible pagination,
  чтобы GC не зависал внутри lease/gate на backend, который возвращает
  truncated page без корректного next cursor;
- не менять GC state machine, request phases, retention, lease или maintenance
  policy;
- не выносить helpers в controller или shared cross-image packages;
- сохранить текущую observability/error surface.

## 4. Scope

- `images/dmcr/internal/garbagecollection/storage_s3.go`
- `images/dmcr/internal/garbagecollection/storage_s3_multipart.go`
- adjacent package-local tests, если нужен regression proof.
- `plans/active/dmcr-gc-s3-consolidation/`

## 5. Non-goals

- Не менять DMCR API, storage key format, request Secret schema или cleanup
  phases.
- Не объединять GC executor lease и maintenance gate state machines.
- Не менять direct-upload cleanup policy, stale-age/protected-prefix policy или
  multipart gone-upload handling.
- Не трогать controller, API, RBAC, Helm templates или runtime entrypoints.
- Не делать cross-image deduplication между controller `modelpack/oci` и DMCR.

## 6. Критерии приёмки

- S3 object, multipart-upload и multipart-part pagination остаются отдельными
  package-local helpers с собственными cursor types.
- Каждый truncated page fail-closed проверяет непустой и продвигающийся next
  cursor.
- Повторяющаяся multipart target validation схлопнута в package-local helper.
- Файлы остаются ниже LOC<350.
- Ошибки S3 list/delete path остаются явными и fail-closed.
- Существующие GC tests проходят.
- Targeted package tests и race проходят.
- `git diff --check` проходит.
- Если code diff выходит за DMCR GC package, задача останавливается и
  пересматривается.

## 7. Риски

- Ошибка в pagination может silently пропустить объекты и оставить мусор в S3.
- Чрезмерно общий helper может скрыть различия object, multipart-upload и
  multipart-part inventories.
- Изменение request lifecycle под видом helper cleanup может сломать replay
  после рестарта.
