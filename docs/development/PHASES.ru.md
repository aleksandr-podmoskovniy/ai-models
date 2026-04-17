# Этапы разработки ai-models

## Этап 1. Phase reset to ai-models-owned publication baseline

### Что делаем
- фиксируем `Model` / `ClusterModel` как platform-facing contract;
- держим controller-owned publication в OCI `ModelPack` artifacts;
- используем внутренний `DMCR` как publication backend;
- вырезаем historical backend shell из live repo;
- заменяем transitional publisher на ai-models-owned implementation;
- удерживаем values, OpenAPI, templates и werf в DKP-манере без backend-driven
  contract drift.

### Что не делаем
- не проектируем здесь ещё `DMZ` registry tier, node-local cache daemon или
  lazy/FUSE runtime paths;
- не оставляем dual-stack fallback на historical backend или `KitOps`;
- не тащим runtime/distribution knobs в public `Model.spec`.

### Критерий выхода
- модуль можно включить;
- publication path controller-owned и explainable;
- backend-centric repo surfaces больше не определяют baseline;
- базовые smoke-проверки publish/runtime проходят;
- templates и values проходят рендер и валидацию.

## Этап 2. Distribution and runtime topology

### Что делаем
- добавляем `DMZ` registry tier поверх canonical OCI artifact publication;
- проектируем node-local cache и mount delivery для runtime workloads;
- закрываем large-model operator UX, cache topology и distribution observability;
- при достаточном сигнале исследуем advanced lazy-loading paths.

### Что не делаем
- не размываем publication contract деталями distribution topology;
- не обещаем generic `FUSE` / stream-to-VRAM path без defendable runtime proof.

### Критерий выхода
- distribution topology объяснима и наблюдаема;
- runtime delivery работает не только как per-pod materialization workaround;
- большие модели обслуживаются предсказуемо по storage/cache contract.

## Этап 3. Hardening and long-term support

### Что делаем
- controlled patching;
- distroless для собственного кода и затем, если оправдано, для remaining runtime packaging;
- CVE и dependency hygiene;
- upgrade and rollback discipline;
- supply-chain и security improvements.

### Критерий выхода
- проект выдерживает рост кода и релизов без хаотичного усложнения;
- правила rebase/patch/hardening задокументированы и воспроизводимы.
