# Source capability taxonomy and Ollama source support

## 1. Заголовок

Выровнять model capability taxonomy и добавить проработку Ollama как source
provider.

## 2. Контекст

Старый `status.resolved` одновременно показывал:

- `task`, например `text-ranking`;
- `supportedEndpointTypes`, например `Rerank`;
- `supportedFeatures`, например `VisionInput` или `ToolCalling`.

Это не одно и то же, и scalar `task` выглядел как дублирование. Целевое
решение: source/provider taxonomy живёт только в
`status.resolved.sourceCapabilities`, а `supportedEndpointTypes` остаётся
normalized serving-facing taxonomy для будущего `ai-inference`.

Параллельно нужен новый source provider: Ollama library/registry. Для GGUF
моделей Ollama отдаёт полезную metadata: format, family, parameter size,
quantization, capabilities и подробный `model_info`.

## 3. Постановка задачи

Сделать контракт таким, чтобы catalog metadata была полезной и не шумной:

- чётко разделить source-declared task/capability и ai-models serving
  endpoint capability;
- не плодить два равнозначных поля про "задачу";
- зафиксировать, какие HF/Ollama поля считаются public fact, а какие остаются
  internal evidence;
- спроектировать и реализовать Ollama source без превращения sourcefetch в
  монолит.

## 4. Scope

- `api/core/v1alpha1/*`
- `crds/*`
- `docs/development/MODEL_METADATA_CONTRACT.ru.md`
- `images/controller/internal/domain/modelsource/*`
- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/adapters/modelprofile/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/publishedsnapshot/*`
- tests for changed packages

## 5. Non-goals

- Не реализовывать `ai-inference` scheduler.
- Не хранить в public status provider-specific raw metadata целиком.
- Не добавлять runtime placement hints вроде GPU/MIG/MPS.
- Не обещать, что Ollama `capabilities` автоматически означают готовый
  production endpoint в нашем runtime.
- Не внедрять локальный Ollama daemon как dependency controller runtime.

## 6. Затрагиваемые области

- Public catalog metadata contract.
- Source URL validation and source type detection.
- Remote source fetch/mirror/publication path.
- GGUF profile extraction from source metadata.
- Documentation and e2e runbook follow-up.

## 7. Критерии приёмки

- В документации и коде есть один понятный source-of-truth:
  - source/provider task/capability используется как provenance/evidence;
  - `supportedEndpointTypes` остаётся normalized serving-facing summary;
  - `supportedFeatures` остаётся orthogonal feature summary.
- Public `status.resolved.task` удалён из CRD; CRD/codegen/docs/tests
  согласованы с `sourceCapabilities`.
- Ollama URL проходит source detection отдельно от Hugging Face.
- Ollama manifest/config/layers читаются через narrow adapter, а не через
  HuggingFace-specific code path.
- Ollama GGUF model layer публикуется как model payload, license/params/config
  не смешиваются с weights.
- Ollama metadata заполняет `format=GGUF`, `family`, `parameterCount`,
  `quantization`, `contextWindowTokens` только из надёжных registry/GGUF
  фактов. `architecture`, `supportedEndpointTypes` и `supportedFeatures` не
  заполняются из renderer/HTML/runtime-only hints.
- Для Ollama есть negative tests: неизвестный tag, manifest без model layer,
  неверный digest, unsupported media type.
- Проверки проходят: targeted tests, `api` tests/codegen/CRD verify,
  controller tests, `make verify` перед сдачей.

## 8. Риски

- Смена public field shape может быть breaking change даже на `v1alpha1`.
- Ollama public web pages не являются стабильным машинным API; использовать
  надо registry/API контракты, а не HTML scraping.
- Ollama `capabilities` отражают Ollama runtime, а не наш future
  `ai-inference` runtime; projection должна быть conservative.
