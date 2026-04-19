## 1. Заголовок

Прямой upload тяжёлых OCI blob-слоёв в backing storage `DMCR` под управлением
`DMCR`

## 2. Контекст

Текущий native `OCI` publisher уже убрал локальный workspace и умеет два
источника байтов для `HuggingFace`:

- `Mirror`: `HF -> source mirror -> native OCI publish`;
- `Direct`: `HF -> direct object source -> native OCI publish`.

Но дальше оба пути сходятся в один upload contract:

- publish-worker считает digest/size слоя;
- затем гонит весь слой через `POST/PATCH/GET/PUT /v2/.../blobs/uploads/...`
  в `DMCR`;
- `DMCR` уже сам пишет эти же байты в своё backing object storage.

Для больших моделей это оставляет `DMCR` в толстом пути передачи байтов и
делает registry HTTP upload потенциальным bottleneck, хотя сам модуль уже
контролирует:

- publisher adapter;
- внутренний `DMCR` binary и его pod shell;
- backing `S3`-совместимое object storage.

Первоначально этот bundle делался как dual-transport slice, но затем
пользователь ужесточил целевой runtime contract:

- для тяжёлых blob-слоёв публикации в `DMCR` больше не должно быть
  переключаемых режимов;
- модуль должен всегда использовать самый быстрый путь в backing storage
  `DMCR`, чтобы сам `DMCR` не становился bottleneck на толстом потоке байтов;
- `ModuleConfig` и runtime shell не должны держать upload-mode knobs там, где
  продукт больше не допускает альтернативного поведения.

## 3. Постановка задачи

Нужно зафиксировать один канонический upload contract для native `OCI`
publisher:

- publish-worker получает от `DMCR` внутреннюю upload session;
- пишет тяжёлый layer blob частями сразу в backing object storage `DMCR`;
- `DMCR` затем завершает multipart upload и materialize'ит repository blob
  link.

При этом нужно:

- не тащить новый knob в public `Model.spec`;
- убрать upload-mode knobs из `ModuleConfig`, controller flags/env и
  publish-worker shell;
- не превращать publisher в knowledge-holder внутренних путей `distribution`;
- оставить `config` blob и `manifest` publish через обычный registry path, если
  это снижает scope и риск текущего slice;
- доказать тестами, что canonical path для тяжёлых blob-слоёв больше не идёт
  через `PATCH` upload path реестра.

## 4. Scope

- internal `DMCR` helper/runtime boundary для direct blob upload;
- stateless or safely resumable upload-session contract между publisher и
  `DMCR` helper;
- direct multipart upload в backing storage для тяжёлых layer blobs;
- final blob link materialization под управлением `DMCR`;
- removal of obsolete upload-mode wiring through
  controller/sourceworker/publish-worker/runtime templates;
- обновление docs/evidence под новый byte path.

## 5. Non-goals

- не менять public `Model` / `ClusterModel` API;
- не менять artifact format, layer layout или current `tar`-based packaging;
- не переводить `config` blob и `manifest` publish на backing-storage direct,
  если для текущего slice это не нужно;
- не делать в этом bundle новый `DMZ` tier;
- не переводить `config` blob и `manifest` publish на backing-storage direct,
  если для текущего slice это не нужно;
- не менять `HuggingFace` acquisition modes; этот bundle про transport в
  `DMCR`, а не про ingest-source policy.

## 6. Затрагиваемые области

- `plans/active/dmcr-backing-storage-direct-upload/*`
- `images/dmcr/*`
- `templates/dmcr/*`
- `templates/controller/*`
- `openapi/values.yaml`
- `openapi/config-values.yaml`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/dataplane/publishworker/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

## 7. Критерии приёмки

- Публичный и runtime-visible upload-mode knob больше не существует.
- Для тяжёлых layer blobs publish-worker всегда:
  - не гонит bytes через registry `PATCH` upload path;
  - использует внутренний `DMCR` helper contract для direct multipart upload.
- `DMCR` helper завершает upload и materialize'ит repository blob link так,
  чтобы обычный registry `HEAD/GET` blob path видел blob как published.
- Новый direct-upload helper живёт в `DMCR`-owned runtime boundary, использует
  тот же TLS/auth perimeter и не требует выдавать backing-storage write creds
  publish-worker'у напрямую.
- `config`/`manifest` path и remote inspect после publish остаются корректными.
- Есть узкие тесты на:
  - direct-upload session/auth/token contract;
  - link materialization/idempotency;
  - canonical publisher behavior без registry `PATCH` для тяжёлых layer blobs;
  - render/runtime wiring нового knob.
- Перед завершением проходит `make verify`.

## 8. Риски

- легко сделать хрупкий обход внутренних путей `distribution`, если publisher
  сам начнёт вычислять storage paths без `DMCR`-owned boundary;
- direct multipart upload требует аккуратного resume/replay contract, иначе
  частично загруженные blob sessions станут новым hidden failure mode;
- multi-replica `DMCR` нельзя завязывать на in-memory session state одного pod;
- docs и test evidence уже описывают `Mirror/Direct` acquisition, поэтому
  нужно не спутать source acquisition mode и direct upload transport в
  `DMCR`.
