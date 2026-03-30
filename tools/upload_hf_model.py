#!/usr/bin/env python3

# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
Thin local wrapper around the backend-owned HF import entrypoint.

Recommended phase-1 paths:

1. Large models such as `openai/gpt-oss-20b`:
   run `tools/run_hf_import_job.sh` and keep the data plane inside the cluster.

2. Small models / local experiments:
   kubectl -n d8-ai-models port-forward svc/ai-models 5000:5000
   pip install mlflow transformers huggingface-hub sentencepiece safetensors
   python3 tools/upload_hf_model.py --hf-model-id distilgpt2 --task text-generation
"""

from __future__ import annotations

from pathlib import Path
import runpy


if __name__ == "__main__":
    script = (
        Path(__file__).resolve().parents[1]
        / "images"
        / "backend"
        / "scripts"
        / "ai-models-backend-hf-import.py"
    )
    runpy.run_path(str(script), run_name="__main__")
