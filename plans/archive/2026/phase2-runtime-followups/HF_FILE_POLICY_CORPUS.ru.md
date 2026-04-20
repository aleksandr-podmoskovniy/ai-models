# HF corpus for Slice 33

Дата: `2026-04-16`

## Выборка

- corpus собран через `https://huggingface.co/api/models`
- просмотрено `85` repos с поддерживаемыми текущим contract форматами:
  - `53` `Safetensors`
  - `32` `GGUF`
- выборка включала top-download `text-generation`, `feature-extraction` и
  `GGUF` repos, плюс несколько manual official/community checkpoints.

Representative repos:

- `Qwen/Qwen3-8B`
- `google/gemma-3-12b-it`
- `meta-llama/Llama-3.1-8B-Instruct`
- `microsoft/Phi-3.5-mini-instruct`
- `mistralai/Mistral-7B-Instruct-v0.3`
- `BAAI/bge-large-en-v1.5`
- `Qwen/Qwen3-VL-Embedding-2B`
- `openai/gpt-oss-20b`
- `bartowski/Qwen2.5-7B-Instruct-GGUF`
- `Qwen/Qwen3-8B-GGUF`

## Что показал corpus

- под старой policy `47` из `85` repos имели false-reject path до download.
- самые частые ложные reject-классы:
  - nested or family-specific config companions:
    `1_Pooling/config.json`, `config_sentence_transformers.json`,
    `sentence_bert_config.json`, `video_preprocessor_config.json`
  - chat / metadata companions:
    `chat_template.json`, `modules.json`, `params.json`, `params`,
    `dtypes.json`, `generation_config_for_text_generation.json`
  - benign hidden/eval payloads:
    `.eval_results/*.yaml`
  - helper scripts shipped next to valid checkpoints:
    `configuration_phi3.py`, `modeling_deepseek.py`, `custom_st.py`
  - alternative export trees and weights:
    `onnx/*`, `original/*`, `metal/*`, `pytorch_model-*.bin`,
    `consolidated.00.pth`, `imatrix_unsloth.gguf_file`

## Пересобранная policy

- `keep`:
  - root required asset contract (`config.json` for `Safetensors`, `.gguf` for
    `GGUF`)
  - common companion configs and tokenizer/chat/module/pooling metadata,
    включая nested config files, `chat_template.json`, `modules.json`,
    `params(.json)`, `dtypes.json`
- `benign drop`:
  - docs, images, evaluation assets, helper scripts
  - hidden trees and known non-model subtrees
  - alternative export trees and alternative weight formats
- `hard reject`:
  - compiled/native payloads (`.so`, `.dll`, `.dylib`, `.exe`, `.jar`,
    `.class`, `.wasm`)
  - archive payloads (`.zip`, `.tar`, `.tgz`, `.gz`, `.bz2`, `.xz`, `.7z`,
    `.rar`)

## Why this boundary

- corpus показал, что repo-level failure на harmless side files ломает
  значимую долю реальных HF repos без пользы для безопасности;
- при этом helper scripts и alt-export bytes не должны попадать в final
  packaged model project, поэтому их лучше strip'ить, а не keep'ить;
- hard reject имеет смысл только там, где payload сам по себе несёт другой
  security/operational class, а не просто metadata noise.
