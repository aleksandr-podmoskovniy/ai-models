# Notes: external metadata references

## Hugging Face

- Docs: https://huggingface.co/docs/hub/model-cards
- Relevant contract:
  - model card metadata drives discovery/filtering;
  - `pipeline_tag` is a metadata field for intended task;
  - `pipeline_tag` is displayed on model page, used for task filtering and
    widget/API selection;
  - `library_name` is separate metadata and should not be collapsed into task.
- API docs expose `ModelInfo` fields such as `pipeline_tag`, `tags`,
  `siblings` and config-related metadata:
  https://huggingface.co/docs/huggingface_hub/v0.11.0/package_reference/hf_api

## Ollama

- Docs: https://docs.ollama.com/api-reference/show-model-details
- Relevant `show` response fields:
  - `details.format`
  - `details.family`
  - `details.parameter_size`
  - `details.quantization_level`
  - `capabilities`
  - `model_info.general.parameter_count`
  - `model_info.general.architecture`
  - architecture-specific context length fields
- API docs also describe `pull` as resumable shared download:
  https://github.com/ollama/ollama/blob/main/docs/api.md

Observed registry shape for `https://ollama.com/library/qwen3.6`:

```text
GET https://registry.ollama.ai/v2/library/qwen3.6/manifests/latest

schemaVersion: 2
mediaType: application/vnd.docker.distribution.manifest.v2+json
config:
  mediaType: application/vnd.docker.container.image.v1+json
layers:
  - mediaType: application/vnd.ollama.image.model
    size: 23938321664
  - mediaType: application/vnd.ollama.image.license
  - mediaType: application/vnd.ollama.image.params
```

Observed config for the same model:

```json
{
  "model_format": "gguf",
  "model_family": "qwen35moe",
  "model_families": ["qwen35moe"],
  "model_type": "36.0B",
  "file_type": "Q4_K_M",
  "renderer": "qwen3.5",
  "parser": "qwen3.5"
}
```

Design consequence:

- use registry manifest/config for controller-owned download/mirror/publication;
- do not scrape HTML;
- use bounded HTTP Range reads against the `application/vnd.ollama.image.model`
  GGUF layer to extract exact GGUF metadata before deciding public
  `contextWindowTokens`, `architecture`, modality and tool-calling facts;
- treat Ollama `capabilities` as provider evidence, then project to
  `supportedEndpointTypes` only via conservative mapping.
