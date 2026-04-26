## 1. Current phase

Этап 1. Это cleanup и tightening текущего publication baseline вокруг
user-facing values/runtime contract для remote source ingest.

## 2. Orchestration

`solo`

Причина:

- rename bounded и не требует нового architecture fork;
- меняются values/OpenAPI/templates/controller surfaces, но semantic target
  already clear: remote source fetch only;
- введение dual-name transition сейчас только закрепит legacy vocabulary.

## 3. Slices

### Slice 1. Зафиксировать canonical rename bundle

Цель:

- открыть отдельный compact bundle под hard-cut rename контракта без смешения с
  другими workstreams.

Файлы/каталоги:

- `plans/active/source-fetch-contract-rename/*`

Проверки:

- manual consistency review

Артефакт результата:

- executable bundle с scope, acceptance criteria, validations и rollback
  point.

### Slice 2. Переименовать user-facing contract

Цель:

- заменить confusing user-facing field name на `sourceFetchMode` и явно
  описать его ограниченную семантику.

Файлы/каталоги:

- `docs/CONFIGURATION*.md`
- `openapi/*`
- `templates/*`

Проверки:

- `make helm-template`
- `make kubeconform`

Артефакт результата:

- values/OpenAPI/templates используют только `sourceFetchMode`.

### Slice 3. Выровнять controller/runtime naming

Цель:

- убрать `Acquisition` vocabulary из live controller/runtime contract, где речь
  идёт только про remote source fetch behavior.

Файлы/каталоги:

- `images/controller/cmd/*`
- `images/controller/internal/ports/publishop/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/internal/dataplane/publishworker/*`

Проверки:

- `cd images/controller && go test ./internal/ports/publishop ./internal/dataplane/publishworker ./internal/adapters/k8s/sourceworker ./cmd/ai-models-controller`

Артефакт результата:

- types, vars, flags, env names, logging keys и tests используют только
  `source fetch` vocabulary.

### Slice 4. Repo-level validation

Цель:

- подтвердить, что hard-cut rename не оставил drift между contract surfaces.

Файлы/каталоги:

- все touched surfaces этого bundle

Проверки:

- `make verify`
- `git diff --check`

Артефакт результата:

- green repo-level guards после rename.

## 4. Rollback point

После Slice 2: values/OpenAPI/templates уже согласованы на новом имени, а
controller/runtime rename ещё не начат.

## 5. Final validation

- `make helm-template`
- `make kubeconform`
- `cd images/controller && go test ./internal/ports/publishop ./internal/dataplane/publishworker ./internal/adapters/k8s/sourceworker ./cmd/ai-models-controller`
- `make verify`
- `git diff --check`
- `review-gate`
