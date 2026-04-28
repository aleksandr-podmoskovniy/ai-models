# Diffusers layout and generation capabilities

## 1. Заголовок

Расширить catalog publication на Diffusers layout и видео/audio generation
capabilities.

## 2. Контекст

Предыдущий profile slice расширил `status.resolved.supportedEndpointTypes` и
`status.resolved.supportedFeatures`, но Diffusers-подобный layout всё ещё
проходит как `Safetensors`. Это смешивает transformer checkpoint format и
multi-component diffusion pipeline layout. Также нет явного `VideoGeneration`,
а `text-to-audio` сейчас смешан с TTS.

## 3. Постановка задачи

Сделать следующий минимальный production slice:

- ввести публичный `format=Diffusers`;
- принимать HF/Upload archives с `model_index.json` и `.safetensors` weights;
- определять image/video/audio generation capabilities из надёжного task или
  Diffusers pipeline class;
- не обещать MCP/runtime launch support в catalog;
- обновить CRD, docs и targeted tests.

## 4. Scope

- `api/core/v1alpha1` enums для `Diffusers`, `VideoGeneration`,
  `AudioGeneration`, `VideoInput`, `VideoOutput`.
- `modelformat` rules/detection для Diffusers layout.
- `sourcefetch` summary для remote/archive Diffusers.
- `modelprofile/diffusers` profile resolver.
- `publishworker` HF/upload archive support.
- `publishstate` public allowlists.
- docs and CRD regeneration.

## 5. Non-goals

- Не добавлять runtime launch profiles, GPU/MIG/MPS или ai-inference planner.
- Не утверждать, что `ToolCalling` означает MCP support.
- Не поддерживать unsafe Python/custom code execution из HF repo.
- Не добавлять прямую публикацию single `.safetensors` Diffusers component без
  `model_index.json`.
- Не реализовывать actual image/video generation serving runtime.

## 6. Затрагиваемые области

- `api/core/v1alpha1`
- `crds/`
- `images/controller/internal/adapters/modelformat`
- `images/controller/internal/adapters/modelprofile`
- `images/controller/internal/adapters/sourcefetch`
- `images/controller/internal/dataplane/publishworker`
- `images/controller/internal/domain/*`
- `docs/development/MODEL_METADATA_CONTRACT.ru.md`

## 7. Критерии приёмки

- HF repo / upload archive с `model_index.json` и `.safetensors` выбирается как
  `Diffusers`, а не `Safetensors`.
- Diffusers text-to-image публикует `ImageGeneration + ImageOutput`.
- Diffusers text-to-video публикует `VideoGeneration + VideoOutput`.
- Diffusers image-to-video публикует `VideoGeneration + VisionInput +
  VideoOutput`.
- Audio generation не смешивается с TTS.
- Unknown Diffusers pipeline без reliable task не публикует endpoint badges.
- CRD содержит новые enum values.
- Targeted Go tests проходят.

## 8. Риски

- HF task taxonomy шире текущего endpoint API; нужно добавлять только
  capability-grade значения, а не все возможные tags.
- Diffusers repo может содержать `.bin` weights; этот slice намеренно
  fail-closed и принимает только safetensors.
- Video/audio generation runtime compatibility останется задачей ai-inference.
