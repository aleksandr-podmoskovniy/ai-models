## 1. Current phase

Этап 2. Это reset/realignment уже landed publication/runtime baseline к cleaner
production-ready target. Public API не меняется; чистится и упрощается
runtime/distribution shell вокруг него.

## 2. Orchestration

`full`

Причина:

- задача меняет runtime code, docs, plans и CI wording сразу в нескольких
  областях;
- она требует архитектурного решения по node-cache contract;
- по правилам репозитория сюда просилась бы read-only delegation, но в текущей
  сессии execution policy не даёт запускать subagents без явного запроса
  пользователя, поэтому findings и decisions фиксируются прямо в bundle.

## 3. Slices

### Slice 1. Зафиксировать compact reset bundle и bounded target contract

Цель:

- явно зафиксировать, что target runtime baseline больше не держит
  per-node-intent `ConfigMap` mirror;
- определить replacement contract с минимальным количеством ownership seams.

Файлы/каталоги:

- `plans/active/runtime-baseline-prod-reset/*`
- `plans/active/node-local-cache-runtime-delivery/*`
- `plans/active/phase2-model-distribution-architecture/*`
- `plans/archive/2026/phase2-runtime-followups/*`
- `plans/archive/2026/publication-source-acquisition-modes/*`

Проверки:

- manual consistency review

Артефакт результата:

- compact current bundle и clear reset target.

### Slice 2. Убрать `nodecacheintent` controller/ConfigMap contract

Цель:

- заменить persisted per-node intent `ConfigMap` на direct runtime-side desired
  extraction from live managed Pods on the current node;
- убрать residual helper/package naming drift after the contract removal.

Файлы/каталоги:

- `images/controller/internal/nodecacheintent/*`
- `images/controller/internal/controllers/nodecacheintent/*`
- `images/controller/internal/adapters/k8s/nodecacheintent/*`
- `images/controller/internal/nodecache/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/bootstrap/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `templates/controller/rbac.yaml`
- `templates/node-cache-runtime/*`

Проверки:

- `cd images/controller && go test ./cmd/ai-models-artifact-runtime ./internal/nodecache ./internal/controllers/nodecacheruntime ./internal/adapters/k8s/nodecacheruntime ./internal/bootstrap`

Артефакт результата:

- dedicated intent controller and ConfigMap contract removed;
- runtime agent loads desired artifacts directly from live cluster truth on its
  node;
- live code no longer keeps `internal/nodecacheintent` as a fake surviving
  boundary.

### Slice 3. Синхронизировать runtime docs, structure и evidence

Цель:

- привести docs к фактическому live runtime baseline after slice 2.

Файлы/каталоги:

- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `docs/README.md`
- `docs/README.ru.md`
- `README.md`
- `README.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `make helm-template`
- `make kubeconform`

Артефакт результата:

- current docs enumerate only live boundaries and contracts.

### Slice 4. Нормализовать stale active bundles и CI wording

Цель:

- убрать stale runtime narrative из active plans и CI hints.

Файлы/каталоги:

- `.gitlab-ci.yml`
- `plans/active/node-local-cache-runtime-delivery/*`
- `plans/active/phase2-model-distribution-architecture/*`
- `plans/archive/2026/publication-source-acquisition-modes/*`
- `plans/archive/2026/phase2-runtime-followups/*`
- `plans/archive/2026/*`

Проверки:

- `rg -n "PostgreSQL|KitOps|DaemonSet|intent ConfigMap" .gitlab-ci.yml plans/active/node-local-cache-runtime-delivery plans/active/phase2-model-distribution-architecture plans/archive/2026/publication-source-acquisition-modes`

Артефакт результата:

- active plan surfaces no longer contradict landed baseline;
- stale historical bundle no longer stays in `plans/active`.

### Slice 5. Repo-level validation

Цель:

- подтвердить, что reset landed without hidden drift.

Файлы/каталоги:

- all touched surfaces in this bundle

Проверки:

- `make fmt`
- `make test`
- `make verify`
- `git diff --check`

Артефакт результата:

- bounded prod-baseline reset with green repo guards.

## 4. Rollback point

После Slice 2: если direct runtime desired extraction окажется неверным, можно
остановиться до docs/plan cleanup и вернуть только node-cache contract without
retaining stale wording changes.

## 5. Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
- `review-gate`
