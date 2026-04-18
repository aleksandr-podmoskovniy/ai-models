## 1. Заголовок

Phase reset continuation: нормализация live baseline, bundle hygiene и
controller test evidence

## 2. Контекст

Reset workstream вокруг ai-models-owned publication/runtime baseline уже landed:

- `MLflow`, `KitOps` и retired `postgresql` shell вычищены из live runtime;
- publication path переведён на native OCI publisher;
- `HF` и upload byte paths доведены до streaming-first semantics.

Но canonical active surface ушёл в structural drift:

- bundle `phase-reset-own-modelpack-and-remove-mlflow` разросся в historical
  log и перестал быть рабочей поверхностью;
- `images/controller/TEST_EVIDENCE.ru.md` смешивает current coverage inventory,
  historical slices и уже неактуальные transition assertions;
- `images/controller/STRUCTURE.ru.md` и related repo-local docs нужно
  пересверить с текущим live tree после reset.

Пользовательский запрос сейчас не про новую feature, а про системную
нормализацию: убрать легаси narrative, привести bundle/docs/evidence в
компактный и defendable current-state вид и не оставлять patchwork.

## 3. Постановка задачи

Нужно:

- архивировать oversized predecessor bundle и завести компактный continuation
  bundle для текущего live baseline;
- переписать controller evidence surface как current-state document, а не как
  slice log;
- актуализировать `STRUCTURE.ru.md` и связанные layout/discipline assertions под
  уже landed native publisher, streaming publish и test split;
- убрать неактуальные или misleading historical формулировки там, где они
  конфликтуют с текущим live state.

## 4. Scope

- `plans/active/*`
- `plans/archive/2026/*`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`
- при необходимости `docs/development/REPO_LAYOUT.ru.md`

## 5. Non-goals

- не менять runtime/API/values contracts;
- не запускать новый distribution/runtime workstream (`DMZ`, node-local cache);
- не переписывать все historical archived bundles под новый стиль;
- не делать cosmetic wording churn вне touched surfaces.

## 6. Затрагиваемые области

- `plans/archive/2026/phase-reset-own-modelpack-and-remove-mlflow/*`
- `plans/active/phase-reset-baseline-normalization/*`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`

## 7. Критерии приёмки

- oversized predecessor bundle убран из `plans/active/` и лежит в
  `plans/archive/2026/phase-reset-own-modelpack-and-remove-mlflow/` как
  historical source of truth;
- в `plans/active/` есть новый compact continuation bundle с текущим baseline и
  без giant-slice log;
- `TEST_EVIDENCE.ru.md` описывает current live coverage inventory и testing
  discipline без stale transition claims, которые уже противоречат коду;
- `STRUCTURE.ru.md` и, если потребуется, `REPO_LAYOUT.ru.md` согласованы с
  текущим live tree и с reset baseline;
- touched surfaces не создают новый parallel source of truth о publication
  architecture;
- `make verify` проходит на финальном state.

## 8. Риски

- можно потерять полезный historical context при агрессивной чистке bundle;
- можно переписать evidence/documentation слишком общо и потерять defendable
  current-state claims;
- можно оставить противоречия между archived predecessor и new active bundle,
  если continuation не будет явно на него ссылаться.
