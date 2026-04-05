# Rationale

## Почему platform path строим вокруг KitOps

### 1. Нам нужен OCI-native artifact plane, а не raw storage UX

Для платформы важен не только факт хранения весов, но и следующие свойства:

- стабильный artifact reference;
- digest-addressability;
- registry-native authz/RBAC;
- compatibility с уже знакомой OCI operational model;
- воспроизводимый handoff между publish и runtime.

`KitOps` даёт это поверх стандартного OCI registry вместо новой bespoke
`model://` инфраструктуры.

### 2. Source-first UX лучше сочетается с KitOps, чем artifact-first UX

Наша public модель уже построена вокруг:

- `spec.source`;
- controller-owned publication;
- saved artifact в `status`.

Это соответствует реальному platform UX:

- пользователь говорит, откуда взять модель;
- платформа сама нормализует package;
- платформа сама публикует и верифицирует артефакт;
- runtime потребляет уже готовый published artifact.

`KitOps` здесь выступает именно как packaging/delivery layer, а не как
пользовательская authoring-модель.

### 3. KitOps даёт нам не только pack/push, но и правильный runtime handoff

Для v0 нам нужен не «идеальный» materializer, а безопасный и понятный bridge от
OCI artifact к локальному пути в Pod.

Upstream `kit init` container уже даёт bounded primitive:

- pull published artifact;
- optional signature verification;
- unpack into shared volume / PVC;
- exit before main runtime starts.

Это достаточно сильный v0 adapter, чтобы не изобретать свой runtime puller
раньше времени.

### 4. KitOps подходит и для ModelKit, и для ModelPack

Для нас важно не зацементировать platform UX в частную реализацию упаковки.
`KitOps` CLI умеет работать и с `ModelKit`, и с `ModelPack`, а current public
API уже ориентирован на `ModelPack`.

Поэтому целевая формулировка такая:

- используем `KitOps` toolchain;
- публикуем OCI artifact platform-approved package format;
- runtime delivery использует `kit unpack`, который одинаково работает с обоими
  supported artifact types.

### 5. Почему не raw S3 path

Raw object-storage path неудобен как canonical public artifact plane:

- слабее addressability;
- неудобнее authz model на уровне отдельных моделей;
- сложнее runtime delivery contract;
- сложнее сделать единый path для разных consumers.

Object storage может оставаться internal storage backend где это нужно, но не
должен определять platform UX delivery.

## Почему KitOps недостаточно сам по себе

`KitOps` не снимает с нас ответственность за:

- source policy;
- auth projection;
- signing policy;
- delete guards;
- admission/runtime policy;
- cache/materialization lifecycle;
- ML-specific safety decisions.

То есть `KitOps` — это сильный packaging/delivery primitive, но не готовая
платформенная система управления рисками.

## Какие возможности KitOps реально используем

### V0 mandatory

- `kit pack`
- `kit push`
- `kit inspect --remote`
- immutable OCI digest refs
- `kit unpack` через upstream `kitops-init`
- selective unpack

### V1 target

- `Cosign` signing after publication
- `Cosign` verification before runtime unpack
- policy-driven unpack filters per runtime class

### Что не должно утекать в public API

- raw `kit` CLI flags;
- upstream init-container env names;
- internal registry auth mechanics;
- internal staging/promote details.
