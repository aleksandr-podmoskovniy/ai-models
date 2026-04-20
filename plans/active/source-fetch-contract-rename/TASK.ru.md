# Source fetch contract rename

## 1. Заголовок

Переименовать confusing contract `sourceAcquisitionMode` в более понятный
`sourceFetchMode` без dual-name legacy

## 2. Контекст

В current publication baseline уже landed:

- два явных режима remote `source.url`: `Direct` и `Mirror`;
- отдельный upload-session path для `spec.source.upload`;
- controller-owned publication во внутренний `DMCR`.

Но user-facing и runtime-facing vocabulary вокруг remote ingest сейчас
перегружен словом `Acquisition`, хотя реальная семантика у параметра уже уже:

- он управляет только тем, как controller fetch'ит remote `source.url`;
- он не управляет upload-session path;
- он не является generic acquisition contract для всех источников.

Из-за этого текущий контракт читается тяжелее, чем сама функция настройки.
Пользователь прямо указал, что `AcquisitionMode` звучит сложно и непонятно, и
попросил переписать контракт в коде.

## 3. Постановка задачи

Нужно сделать hard-cut rename текущего контракта:

- user-facing values/OpenAPI поле `artifacts.sourceAcquisitionMode` заменить на
  `artifacts.sourceFetchMode`;
- во внутренних controller/runtime surfaces заменить
  `SourceAcquisitionMode`/`source-acquisition-mode`/related env names на
  `SourceFetchMode`/`source-fetch-mode`;
- синхронизировать docs и формулировки так, чтобы явно читалось:
  параметр относится только к remote `source.url`, а upload-session остаётся
  отдельным staged path.

## 4. Scope

- новый bundle `plans/active/source-fetch-contract-rename/*`;
- `docs/CONFIGURATION*.md`;
- `openapi/*`;
- `templates/*`, которые проецируют module values в controller runtime;
- `images/controller/cmd/*`;
- `images/controller/internal/ports/publishop/*`;
- `images/controller/internal/adapters/k8s/sourceworker/*`;
- `images/controller/internal/dataplane/publishworker/*`;
- controller test evidence и связанные тесты по touched packages.

## 5. Non-goals

- не менять значения режимов `Direct` / `Mirror`;
- не менять upload-session/runtime semantics для `spec.source.upload`;
- не вводить временный backward-compatible alias поверх старого имени;
- не проектировать новый third mode или новый public API для sources;
- не трогать `vLLM` и исследовательские документы.

## 6. Затрагиваемые области

- public module values/OpenAPI contract;
- controller flag/env/runtime wiring;
- publication port naming;
- remote source ingest docs и test evidence.

## 7. Критерии приёмки

- В user-facing contract больше нет поля `artifacts.sourceAcquisitionMode`;
  каноническое имя — `artifacts.sourceFetchMode`.
- OpenAPI/examples/templates/controller wiring используют только новое имя.
- Internal Go/runtime contract больше не использует `SourceAcquisitionMode`
  vocabulary в live code paths, flags и env names.
- Docs явно фиксируют, что `sourceFetchMode` применяется только к remote
  `source.url`, а `spec.source.upload` остаётся отдельным staged upload path.
- Нет dual-name drift между docs, OpenAPI, templates и controller code.
- Проходят:
  - `cd images/controller && go test ./internal/ports/publishop ./internal/dataplane/publishworker ./internal/adapters/k8s/sourceworker ./cmd/ai-models-controller`
  - `make verify`
  - `git diff --check`

## 8. Риски

- можно оставить hidden drift, если переименовать только values/docs, но не
  runtime/env/flags;
- можно неочевидно сломать chart/template wiring controller runtime;
- можно оставить старое слово в test evidence и тем самым закрепить dual
  vocabulary;
- hard-cut rename меняет values contract, поэтому нужно держать все touched
  surfaces строго синхронизированными.
