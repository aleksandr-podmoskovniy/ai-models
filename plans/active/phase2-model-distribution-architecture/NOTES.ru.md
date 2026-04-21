## Каноническая целевая картина

Ниже зафиксированы два решения, которые считаются целевыми для этого
workstream и не должны размываться временными промежуточными вариантами.

### 1. Единственный правильный publication protocol

Целевой publication path для больших моделей:

1. Контроллер открывает publication session в `DMCR`.
2. Модель разбирается не как один giant blob, а как набор publication units:
   - большой исходный файл модели (`.safetensors`, `.gguf`, `.bin`) -> отдельный
     raw blob;
   - мелкие служебные файлы -> ограниченные bundle-слои, если это выгодно.
3. Для каждого publication unit создаётся отдельная blob-session.
4. После sealing всех нужных blob-ов публикуется маленький `config/manifest`,
   который описывает:
   - materialized root модели;
   - file-to-blob mapping;
   - runtime-visible layout.
5. Только после публикации `config/manifest` артефакт становится опубликованным
   по digest.

Что это означает practically:

- digest-first helper, который требует знать итоговый digest до старта upload,
  не является целевой архитектурой;
- full-model repack в один 500 GB layer не является целевым поведением;
- единицей возобновления, дедупликации и node-cache reuse должен быть отдельный
  blob, а не весь артефакт целиком;
- вся дальнейшая работа по publisher должна строиться вокруг этого протокола,
  а не вокруг старой digest-first схемы.

### 1.1. Что именно должно поменяться внутри `DMCR`

Ключевая развилка здесь такая:

- published digest обязан остаться стабильным внешним контрактом;
- published artifact не обязан совпадать с одним физическим blob-ом.

Значит целевая внутренняя схема должна быть такой:

1. `DMCR` хранит реестр sealed blob-ов:
   - `digest`
   - `physicalKey`
   - `size`
   - `mediaType`
   - `state`
2. `DMCR` хранит publication session:
   - список файлов модели;
   - соответствие `file -> blob`;
   - materialized root;
   - состояние сборки published artifact.
3. Published artifact собирается как обычный `OCI` manifest/config над уже
   sealed blob-set.
4. Consumer снаружи по-прежнему видит immutable artifact by digest, а не
   внутренние session/object id.

Практический смысл:

- большие файлы можно повторно использовать между публикациями;
- dedupe и cache reuse происходят на уровне blob-а;
- published contract остаётся прежним, потому что consumer по-прежнему работает
  с immutable artifact by digest.

### 1.2. Идеальный production-ready вариант

Если нужен действительно лучший вариант без двойного чтения источника и без
лишнего усложнения `DMCR`, то целевая схема такая:

1. Для trusted internal sources:
   - `publisher` читает конкретный большой файл один раз;
   - считает digest на своей стороне;
   - льёт части напрямую в object storage под signed blob-session;
   - завершает blob-session с `digest/size/parts`;
   - `DMCR` запечатывает blob и даёт его использовать в manifest.
2. Для untrusted user upload:
   - внешний клиент загружает файл в временный staged object с возобновлением;
   - внутренний sealer читает staged object один раз;
   - считает digest и переводит тот же физический объект в состояние sealed
     blob без полной переписи байтов в новый объект.
3. После готовности всех blob-ов `publisher` публикует маленький
   `config/manifest`.

Почему именно так:

- `DMCR` не становится обязательным relay для всех тяжёлых байтов;
- возобновление идёт по файлу и по части файла, а не по монолитному артефакту;
- dedupe становится полезным не только между повторами одной публикации, но и
  между версиями близких моделей;
- node-local cache сможет реиспользовать те же blob-ы, а не только целые
  архивы.

Это и есть целевая комбинация:

- one-file/one-blob для тяжёлых model files;
- один опубликованный артефакт как manifest-over-blob-set;
- минимум тяжёлого трафика через `DMCR`;
- published contract снаружи не меняется.

### 1.3. Что остаётся неизбежной ценой

Полностью "без минусов" физически не получится. Остаётся один осознанный
компромисс:

- metadata/index и session-state становятся заметно сложнее, чем при схеме
  "один upload -> один blob".

Но это лучший из вариантов, потому что альтернативы хуже:

- либо `DMCR` становится гигантским сетевым насосом;
- либо публикация живёт монолитными 500 GB объектами;
- либо возобновление и дедупликация работают слишком грубо;
- либо node-cache не сможет нормально делить модель на переиспользуемые
  единицы.

### 1.4. Как именно пойдут данные

#### Путь из `Hugging Face`

Если модель состоит, например, из `100` файлов `safetensors` суммарно на
`500 GB`, то идеальный путь такой:

1. Контроллер получает список файлов `HF snapshot`.
2. Каждый большой файл открывается как отдельный blob-session.
3. Для каждого файла:
   - читаем `HF` один раз;
   - пишем этот файл в multipart object storage;
   - считаем digest этого файла;
   - seal'им один blob.
4. В итоге получаем:
   - около `100` больших sealed blob-ов;
   - несколько маленьких blob-ов под config/tokenizer/service files;
   - один маленький published manifest.

Трафик:

- `HF -> publisher`: около `500 GB`;
- `publisher -> object storage`: около `500 GB`;
- `DMCR` занимается в основном session/index metadata, а не прокачкой всех
  `500 GB` через себя.

#### Путь пользовательской загрузки

Для внешнего пользователя trust level другой, поэтому путь должен быть иным:

1. Пользователь грузит файл в temporary staged object с возобновляемой
   загрузкой.
2. После завершения загрузки internal sealer читает staged object один раз.
3. На этом чтении:
   - считает digest;
   - проверяет размер/состояние;
   - переводит тот же физический объект в sealed blob registry.
4. После sealing всех нужных файлов публикуется маленький manifest.

Трафик:

- `user -> staged object`: полный размер файлов;
- `staged object -> sealer`: ещё один полный read;
- без обязательной второй полной записи тех же байтов в новый canonical object.

### 2. Published contract остаётся тем же, внутренности можно менять радикально

Снаружи для consumers должны сохраняться только стабильные инварианты:

- опубликованный артефакт остаётся immutable OCI artifact by digest;
- materialization даёт стабильный consumer-facing entrypoint модели;
- workload получает тот же стабильный runtime contract, а не knowledge о
  внутренней раскладке слоёв;
- публичный `Model` / `ClusterModel` контракт не получает storage/runtime
  внутренности.

При этом внутри implementation allowed:

- менять число слоёв;
- менять порядок слоёв;
- выносить `weight.config` в отдельный слой;
- дробить веса на несколько внутренних слоёв;
- менять internal runtime/distribution topology;
- полностью убирать старые промежуточные delivery schemes после cutover.

Ключевое правило:

- published contract stabilizes the outside;
- internal artifact layout and runtime topology remain replaceable.

### 3. Единственная целевая runtime/distribution topology

Целевой runtime path для доставки опубликованной модели:

- node-local shared cache;
- node-owned runtime agent;
- общий mount модели в workload;
- повторное использование одного digest на одной ноде многими workload'ами;
- eviction/refresh внутри node cache plane.

Что не считается частью финальной схемы:

- per-workload materialize в отдельный том как долгоживущий product mode;
- постоянное coexistence двух равноправных delivery modes;
- operator-facing narrative, где fallback считается нормальной целевой
  эксплуатационной схемой.

Если временный bridge нужен для миграции live кода, он должен быть описан как
текущая переходная мера, а не как часть target architecture.

### 4. Границы, которые нельзя размывать

`DMCR`:

- владеет publication backend и финальной digest-addressed фиксацией артефакта;
- не должен оставаться узким местом из-за старого digest-first upload
  протокола.

`sds-node-configurator`:

- даёт storage substrate;
- не является cache manager, registry pull manager или workload mount plane.

`DMZ` registry:

- отдельный distribution tier над уже опубликованным OCI artifact;
- не новый source-ingest contract.

### 5. Практическое следствие для implementation bundles

Из этих решений следуют отдельные рабочие slices:

1. `dmcr-direct-upload-v2`
   - временные upload sessions;
   - поздняя фиксация digest;
   - cleanup/TTL незавершённых сессий.
2. `stream-capable-modelpack-publisher`
   - one-pass source read;
   - hash-while-upload;
   - публикация без full local restaging.
3. `node-cache-runtime-delivery`
   - единственная целевая runtime topology;
   - cutover и удаление long-lived fallback narrative.
