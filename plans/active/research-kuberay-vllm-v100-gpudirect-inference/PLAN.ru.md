# PLAN

## Current phase

Исследовательский pre-implementation slice. Это не phase-1 runtime change и не
phase-2 API change в `ai-models`, а отдельное R&D по будущему distributed
inference profile поверх `KubeRay + vLLM`.

## Orchestration

- mode: `solo`
- reason:
  - задача исследовательская и docs-first;
  - главный риск сейчас в корректной compatibility/memory модели, а не в
    реализации кода;
  - отдельные subagents не разрешены пользователем явно.

## Slice 1. Reconstruct the hardware and runtime constraints

Цель:

- собрать ограничения, которые накладывают:
  - `V100`;
  - `compute capability 7.0`;
  - текущий `RDMA/GPUDirect` baseline;
  - layout на 3 GPU.

Затрагиваемые области:

- archived predecessor bundle
- official docs for `vLLM` and `KubeRay`

Проверки:

- facts must be separated from inference
- only primary or official sources

Артефакт:

- compact constraint baseline for the current stand.

## Slice 2. Determine the viable `vLLM` engine/runtime profile

Цель:

- понять, как `V100` влияет на выбор `vLLM` engine;
- определить, нужен ли явный `V0` pin/flag;
- понять, какие версии и launch knobs надо фиксировать.

Затрагиваемые области:

- active bundle docs
- official `vLLM` docs / issues / release references

Проверки:

- source-backed compatibility notes

Артефакт:

- exact engine/runtime verdict for `Volta`.

## Slice 3. Choose the model and parallelism strategy

Цель:

- подобрать основной модельный профиль;
- сравнить `TP=3`, `PP=3`, либо смешанный профиль;
- не выбрать конфигурацию, которая ломается на делимости голов, памяти или
  архитектурных ограничениях.

Затрагиваемые области:

- active bundle docs
- official model configs/cards
- official `vLLM` docs

Проверки:

- explicit memory math
- explicit model architecture constraints

Артефакт:

- primary recommendation and fallback options.

## Slice 4. Define the `KubeRay + vLLM` deployment shape

Цель:

- превратить выбранный профиль в практический launch plan:
  - pod layout;
  - head/worker shape;
  - GPU assignment;
  - env vars;
  - launch command.

Затрагиваемые области:

- active bundle docs
- official `KubeRay` docs
- official `vLLM` docs

Проверки:

- launch shape must be consistent with the chosen parallelism mode
- no contradiction with the known GPUDirect baseline

Артефакт:

- concrete deployment recipe draft.

## Slice 5. Record the recommendation

Цель:

- оформить итог в bundle как operator-facing technical note;
- оставить короткую actionable summary для следующего шага.

Затрагиваемые области:

- `plans/active/research-kuberay-vllm-v100-gpudirect-inference/*`

Проверки:

- `git diff --check`
- manual consistency review of the bundle

Артефакт:

- `RECOMMENDATION.ru.md` with:
  - recommended model;
  - recommended engine;
  - recommended parallelism;
  - deployment shape;
  - residual risks.

## Rollback point

Если по итогам выяснится, что на `3x V100` жизнеспособен только слишком
ограниченный профиль:

1. не создавать ложное impression, что full distributed profile уже выбран;
2. оставить bundle как честный compatibility verdict;
3. выделить next-step отдельно: либо smaller model, либо иной runtime, либо
   другой hardware target.

## Final validation

- `git diff --check`
- manual bundle review against scope and factual consistency
