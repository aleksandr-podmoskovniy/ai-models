# KubeRay + vLLM на `3x V100 32GB`: рекомендуемый профиль

## Короткий ответ

Для текущего стенда с тремя `V100 32GB`, уже подготовленным `RDMA` и
подтверждённым `GPUDirect RDMA` практический профиль выглядит так:

- основной профиль:
  - `KubeRay` c тремя worker group по одной `GPU`;
  - pod-level `sdn` / `DRA`, без `hostNetwork`;
  - публичный образ `rayproject/ray-llm:2.54.0-py311-cu128`;
  - `Ray 2.54.0`;
  - `vLLM 0.15.0` внутри этого образа;
  - `V1-only`;
  - модель `deepseek-ai/DeepSeek-R1-Distill-Qwen-14B`;
  - `dtype=half`;
  - `tensor_parallel_size=1`;
  - `pipeline_parallel_size=3`;
- `max_model_len=32768`
- `max_num_seqs=5`
- запасной плотный профиль на том же runtime:
  - `Qwen/Qwen2.5-14B-Instruct`;
  - `dtype=half`;
  - `PP=3`, `TP=1`;

Важный вывод для этого железа:

- не надо начинать с `TP=3`;
- не надо планировать запуск в `BF16`;
- не надо начинать с публичных `Qwen3.5`- и больших `DeepSeek MoE`-профилей;
- не надо уходить в node-level `RDMA` workaround;
- не надо выдавать pod лишние Linux capabilities “на всякий случай”;
- не надо рассчитывать на `V0` как на deployment path: текущий внешний bundle
  уже сознательно зафиксирован на `V1-only` через public `ray-llm`.

Практический runtime verdict по текущему стенду:

- `Qwen/Qwen3-14B` на этом exact stack (`ray-llm 2.54.0` / `vLLM 0.15.0 V1` /
  `PP=3` / `3x V100`) доходит до рабочего `RDMA/NCCL`, но падает уже внутри
  `vLLM` на `initialize_attn_backend` с `KeyError` по
  `model.layers.<0|13|27>.self_attn.attn`;
- `DeepSeek-R1-Distill-Qwen-14B` на том же stack падает в том же классе
  ошибки, но уже на `model.layers.<0|16>.self_attn.attn`;
- текущий целевой baseline для `dvp` пока остаётся
  `DeepSeek-R1-Distill-Qwen-14B`: модель уже доведена до живого rollout, и
  дальше нужно чинить именно этот exact профиль, а не подменять модель без
  отдельного решения.

## Ограничения текущего стенда

Актуальная база:

- `3x NVIDIA Tesla V100-PCIE-32GB`;
- `compute capability = 7.0` (`Volta`);
- три хорошие пары `GPU <-> NIC`, уже выделенные в предыдущей работе по
  `RDMA/GPUDirect`:
  - `w1/root80`:
    - GPU `00000000:81:00.0`
    - NIC `0000:82:00.0`
    - Linux netdev `enp130s0np0`
    - verbs device `mlx5_1`
  - `w1/rootc0`:
    - GPU `00000000:C2:00.0`
    - NIC `0000:C1:00.0`
    - Linux netdev `enp193s0np0`
    - verbs device `mlx5_0`
  - `w3/root00`:
    - GPU `00000000:01:00.0`
    - NIC `0000:02:00.0`
    - Linux netdev `enp2s0np0`
    - verbs device `mlx5_0`
- pod-level `GPUDirect RDMA` на этой базе уже был подтверждён.

Практический смысл этих ограничений:

- три `GPU` лежат не на одной симметричной ноде, а на двух нодах;
- на `w1` две хорошие пары сидят в разных `root complex`;
- значит, здесь важна не абстрактная “любая тройка GPU”, а детерминированная
  привязка каждого worker к своей `GPU/NIC` паре.
- и эта привязка должна оставаться именно pod-level, через `sdn` и `DRA`, а не
  через вынос сети на уровень ноды.

## Вердикт по `vLLM` на `V100`

По текущим upstream-источникам:

- `vLLM` поддерживает `NVIDIA GPU` с `compute capability 7.0` и выше:
  <https://docs.vllm.ai/en/v0.17.1/getting_started/installation/gpu/>
- `BF16` в `vLLM` поддерживается только на `compute capability >= 8.0`, а на
  более старых картах нужно явно использовать `half`:
  <https://docs.vllm.ai/en/v0.17.1/getting_started/installation/gpu/>
- в `vLLM 0.10.2` upstream отдельно отметил, что `V1` был расширен на
  `compute capability < 8.0`:
  <https://github.com/vllm-project/vllm/releases/tag/v0.10.2>
- в исходниках `vLLM 0.10.2` `VLLM_USE_V1` по умолчанию равен `1`, а для
  старых `GPU` `V1` уходит в `FlexAttention`:
  - <https://raw.githubusercontent.com/vllm-project/vllm/v0.10.2/vllm/envs.py>
  - <https://raw.githubusercontent.com/vllm-project/vllm/v0.10.2/vllm/platforms/cuda.py>
- при этом upstream отдельно предупреждал, что `V0` уходит из поддержки и
  ставка на `V100` через один только `V0` не является долгосрочной стратегией:
  <https://github.com/vllm-project/vllm/issues/18571>

Из этого следует такой практический выбор:

- `vLLM 0.10.2` оставляется базовой версией, потому что она уже умеет `V1` на
  `cc < 8.0`, но ещё сохраняет ручной откат на `V0`;
- на `V100` нельзя рассчитывать на тот путь, который даёт на новых картах
  `FlashAttention` и `BF16`:
  - `BF16` запрещён самим upstream-проверками;
  - для старых `GPU` `V1` уходит не в `FlashAttention`, а в
    `FlexAttention`;
- значит, на `Volta` `V1` технически доступен, но не должен считаться
  стартовым “безопасным” режимом только потому, что он включён по умолчанию.

Практический выбор для этого стенда:

- основной bring-up режим:
  - `vLLM == 0.10.2`
  - `VLLM_USE_V1=0`
  - `dtype=half`
  - этот флаг надо выставлять явно, потому что в `0.10.2` `V1` включён по
    умолчанию
- второй этап после успешного запуска:
  - тот же `vLLM == 0.10.2`
  - `VLLM_USE_V1=1`
- не стоит начинать с `vLLM >= 0.11`, пока профиль на `0.10.2` не прошёл
  проверку на стенде:
  - `V0` там уже не является безопасным путём отката;
  - на `Volta` это убирает единственный дешёвый аварийный рычаг.

Вывод по движку:

- `V0` здесь не является “долгосрочной красивой целью”, но именно он нужен как
  основной первый режим на `V100`;
- `V1` на `Volta` имеет смысл проверять сразу после рабочего baseline, но уже
  как второй этап на том же `vLLM 0.10.2`;
- если `V1` на этом железе окажется достаточно стабильным, только тогда имеет
  смысл думать о дальнейшем уходе от `V0`.

## Что видно по `V1` в `0.15.1` и `0.17.1` на `V100/Volta`

Если смотреть уже не на `0.10.2`, а на более поздние ветки `V1`, то картина
такая.

### Что совпадает у `0.15.1` и `0.17.1`

В обеих версиях официальный CUDA install guide пишет одно и то же:

- `GPU: compute capability 7.0 or higher`
- `V100` указан прямо в списке примеров

Источники:

- `v0.15.1`:
  <https://raw.githubusercontent.com/vllm-project/vllm/v0.15.1/docs/getting_started/installation/gpu.cuda.inc.md>
- `v0.17.1`:
  <https://raw.githubusercontent.com/vllm-project/vllm/v0.17.1/docs/getting_started/installation/gpu.cuda.inc.md>

То есть формально обе ветки считают `V100` поддерживаемой CUDA-платформой.

В обеих версиях в коде остаётся тот же базовый вывод по типам данных:

- для `Pascal, Volta and Turing` поддерживаются только `float16` и `float32`
- `BF16` требует `compute capability >= 8.0`

Источники:

- `v0.15.1`:
  <https://raw.githubusercontent.com/vllm-project/vllm/v0.15.1/vllm/platforms/cuda.py>
- `v0.17.1`:
  <https://raw.githubusercontent.com/vllm-project/vllm/v0.17.1/vllm/platforms/cuda.py>

Практический вывод здесь жёсткий:

- для `V100` и на `0.15.1`, и на `0.17.1` надо идти только через
  `dtype=half`.

### Что важно по feature surface на `Volta`

В feature matrix у `0.15.1` и `0.17.1` для `Volta` одинаково указано:

- `Chunked Prefill`: `❌`
- `Automatic Prefix Caching`: `❌`
- `CUDA graph`: `✅`
- `LoRA`, `speculative decoding`, `beam search` и прочее — в целом `✅`

Источники:

- `v0.15.1`:
  <https://raw.githubusercontent.com/vllm-project/vllm/v0.15.1/docs/features/README.md>
- `v0.17.1`:
  <https://raw.githubusercontent.com/vllm-project/vllm/v0.17.1/docs/features/README.md>

Это важно для длинного контекста:

- `V1` на `Volta` не даёт нам ту же красивую картину по long-context
  оптимизациям, которую хочется ожидать по общим рассказам про `V1`;
- поэтому сам по себе переход на `0.15+` или `0.17+` не решает проблему
  длинного окна на `V100`.

### Что видно по backend'ам на `Volta`

В `0.17.1` backend table показывает:

- `FLASH_ATTN FA2` требует `>= 8.0`
- `FLASHINFER` заявлен как поддерживающий `7.x-9.x`
- `FLEX_ATTENTION` и `TRITON_ATTN` имеют более широкий диапазон совместимости

Источник:

- <https://raw.githubusercontent.com/vllm-project/vllm/v0.17.1/docs/design/attention_backends.md>

Практически это означает:

- на `V100` запуск теоретически возможен;
- но нельзя наивно предполагать, что любой автоматически выбранный backend
  будет одинаково хорошо работать на `sm70`.

### Что видно по коду `V1-only`

В `0.15.1` и `0.17.1` auto worker уже выставляется в
`vllm.v1.worker.gpu_worker.Worker`.

То есть это уже именно `V1`-ветки, а не dual-path с живым `V0`.

Источники:

- `v0.15.1`:
  <https://raw.githubusercontent.com/vllm-project/vllm/v0.15.1/vllm/platforms/cuda.py>
- `v0.17.1`:
  <https://raw.githubusercontent.com/vllm-project/vllm/v0.17.1/vllm/platforms/cuda.py>

Отдельно в `v0.17.1` guide по `V1` прямо написано:

- `We have fully deprecated V0`

Источник:

- <https://raw.githubusercontent.com/vllm-project/vllm/v0.17.1/docs/usage/v1_guide.md>

### Реальные upstream-сигналы по `V100`

Ниже важны не только docs, но и фактические issue reports.

Положительный сигнал:

- issue `#17808` описывает реальный запуск `vLLM 0.8.3` на `single V100 GPU`
  для двух моделей;
- проблема там не в том, что `V100` не поддерживается, а в некорректном
  поведении disaggregated prefill;
- это значит, что `V1`-runtime путь на `V100` в принципе живой.

Источник:

- <https://github.com/vllm-project/vllm/issues/17808>

Отрицательный, но очень полезный сигнал:

- issue `#26565` на `vLLM 0.11.0` показывает запуск на `V100 (sm70)`, где
  движок уже загружает модель и доходит до `v1` worker path, но затем падает
  в sampler path, потому что `FlashInfer requires GPUs with sm75 or higher`;
- то есть это не “`V100` в целом не поддерживается”, а конкретная проблема
  backend/sampler-выбора на старой архитектуре.

Источник:

- <https://github.com/vllm-project/vllm/issues/26565>

### Практический вывод по `0.15.1` и `0.17.1`

Для нашего стенда это сводится к такому:

- `0.15.1`:
  - уже настоящий `V1`-профиль;
  - формально поддерживает `V100`;
  - всё ещё живёт с теми же ограничениями `Volta` по `BF16`, `CP` и `APC`;
  - выглядит как более разумный первый кандидат, если хочется проверить
    `V1`-ветку после `0.10.2`.
- `0.17.1`:
  - тоже формально поддерживает `V100`;
  - тоже уже `V1-only`;
  - несёт те же ограничения `Volta`;
  - рискованнее как первый шаг, потому что отката на `V0` уже нет, а issue
    `#26565` показывает реальный backend landmine на `sm70`.

Итоговый порядок проверки на этом железе лучше делать таким:

1. `0.10.2` + `V0`
2. `0.10.2` + `V1`
3. `0.15.1` + `V1`
4. только потом `0.17.1` + `V1`

Если нужна одна короткая инженерная фраза, она такая:

- `0.15.1` и `0.17.1` на `V100` запускаться могут;
- но это не “полноценный modern V1 experience”, а `V1` с ограничениями
  `Volta` и с повышенным вниманием к тому, какой backend реально выбирается
  внутри рантайма.

## Версии `KubeRay`, `Ray` и образ

По текущим upstream-документам:

- `KubeRay` остаётся рекомендуемым способом запускать Ray на Kubernetes:
  <https://docs.ray.io/en/latest/cluster/kubernetes/index.html>
- текущие примеры в документации используют линейку `KubeRay 1.5.1`:
  <https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/upgrade-guide.html>
- `RayCluster` и `RayService` должны использовать контейнерные образы с той же
  версией `Ray`, что указана в `spec.rayVersion`, а в образе должен быть
  `wget`:
  <https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/config.html>

Практический pin для внешнего `k8s-config` deployment bundle:

- `KubeRay operator`: `1.5.1`
- `spec.rayVersion`: `2.54.0`
- образ: `rayproject/ray-llm:2.54.0-py311-cu128`
- фактический `vLLM` внутри этого образа: `0.15.0`

Для `k8s-dvp` действует дополнительное эксплуатационное ограничение: custom
image не используется, а baseline берётся напрямую с Docker Hub. Поэтому сами
Argo-манифесты сознательно выровнены под публичный `ray-llm` образ, а значит,
для этого конкретного deployment bundle мы уже живём на `V1-only` пути.

По образу практическая рекомендация теперь такая:

- не брать “голый” `vllm/vllm-openai` как есть;
- использовать один и тот же публичный `rayproject/ray-llm` образ для head и
  всех worker;
- следить, чтобы `spec.rayVersion` совпадал с версией `Ray` в образе;
- custom image с ручным pin на `vllm==0.10.2` оставлять только как отдельный
  лабораторный fallback, а не как основной операторский путь.

Причина:

- `KubeRay` ожидает согласованную версию `Ray` в `rayVersion` и в image;
- `vLLM` здесь является зависимостью Ray workload, а не заменой базового
  `Ray`-образа;
- отдельный ручной pin `NCCL` не нужен: безопаснее использовать тот `NCCL`,
  который приезжает внутри выбранного `PyTorch/vLLM` userspace, а не мешать
  его с host-side библиотеками.

## Почему здесь нужен `PP=3`, а не `TP=3`

Для этого железа и этих моделей начинать надо с `pipeline parallelism`.

Причины:

1. `TP=3` плохо сочетается с архитектурой популярных моделей такого класса.
   На кандидатах ниже:
   - у `Qwen3-32B` `num_attention_heads = 64`;
   - у `DeepSeek-R1-Distill-Qwen-32B` `num_attention_heads = 40`;
   - у `Falcon-40B` `num_attention_heads = 128`;
   ни одно из этих чисел не делится на `3` без остатка.
2. Upstream `vLLM` прямо рекомендует `pipeline parallelism` как способ
   использовать неравномерный набор `GPU`, когда модель нельзя красиво разрезать
   по `tensor parallelism`:
   <https://docs.vllm.ai/en/latest/serving/distributed_serving.html>
3. Наши три `GPU` уже сейчас разложены по двум нодам. При `TP=3` каждое
   декодирование будет тянуть межнодовый `all-reduce` на каждом слое. Для
   `Volta + PCIe + RDMA` это более рискованный путь, чем `PP=3`.
4. При `PP=3` каждая `GPU` становится отдельной stage:
   - одна `GPU` = один Ray worker bundle;
   - одна `GPU` = одна детерминированная `GPU/NIC` пара;
   - это естественно ложится на текущий `w1/w3` layout.

Итог:

- основной режим:
  - `tensor_parallel_size=1`
  - `pipeline_parallel_size=3`
- смешанный `TP x PP` здесь не нужен на первом проходе;
- `TP=3` не рекомендуется как стартовый профиль.

## Подбор модели

### Основной dense-профиль: `Qwen/Qwen3-32B`

Почему именно он:

- модель официально поддерживается `vLLM` через архитектуру `Qwen3ForCausalLM`:
  <https://docs.vllm.ai/en/latest/models/supported_models.html>
- это обычная dense text-модель, а не `MoE` и не `ConditionalGeneration`;
- в `config.json` модели:
  - `num_hidden_layers = 64`
  - `num_attention_heads = 64`
  - `num_key_value_heads = 8`
  - `hidden_size = 5120`
  - `intermediate_size = 25600`
  - `torch_dtype = bfloat16`
- в `model.safetensors.index.json` указан точный суммарный размер весов:
  - `total_size = 65524246528`
  - это около `61.02 GiB`.

Последняя строка важна:

- в Hub модель размечена как `BF16`;
- на `V100` это не подходит;
- поэтому для неё надо явно задавать `dtype=half`.

### Расчёт памяти для `Qwen3-32B`

Расчёт по точному размеру индекса весов:

- общий вес модели — около `61.02 GiB`;
- при `PP=3` это в среднем около `20.34 GiB` веса на одну `GPU`.

Это уже оставляет нормальный запас в `32 GiB`, даже с учётом:

- служебные накладные расходы `vLLM`;
- CUDA graphs / allocator fragmentation;
- NCCL buffers;
- KV cache.

Оценка `KV cache` для этой архитектуры:

- `head_dim = hidden_size / num_attention_heads = 5120 / 64 = 80`
- размер `KV` на один токен по всей модели:
  - `2 * layers * kv_heads * head_dim * 2 bytes`
  - `2 * 64 * 8 * 80 * 2`
  - `163840 bytes`
  - это примерно `160 KiB` на токен по всему кластеру
- одна последовательность длиной `8192` токена даёт около `1.25 GiB` KV cache
  на весь кластер.

Практическая интерпретация:

- это самый аккуратный dense-кандидат под `3x32 GiB` из текущего shortlist;
- у него заметно более дешёвый `KV cache`, чем у `Qwen2.5-32B` и
  `DeepSeek-R1-Distill-Qwen-32B`;
- поэтому именно его разумно брать первым.

Стартовые параметры для него:

- `max_model_len=8192`
- `max_num_seqs=4`
- `gpu_memory_utilization=0.90`

### Reasoning-альтернатива: `deepseek-ai/DeepSeek-R1-Distill-Qwen-32B`

Почему его стоит держать в shortlist:

- это не большой `DeepSeek MoE`, а dense-модель на архитектуре
  `Qwen2ForCausalLM`;
- именно поэтому она укладывается в те же аппаратные ограничения, что и
  `Qwen2.5-32B`;
- `vLLM` поддерживает `Qwen2ForCausalLM`:
  <https://docs.vllm.ai/en/latest/models/supported_models.html>
- в `config.json` модели:
  - `num_hidden_layers = 64`
  - `num_attention_heads = 40`
  - `num_key_value_heads = 8`
  - `hidden_size = 5120`
  - `intermediate_size = 27648`
  - `torch_dtype = bfloat16`
- в `model.safetensors.index.json` указан суммарный размер весов:
  - `total_size = 65527752704`
  - это около `61.03 GiB`.

Для `V100` здесь опять же нужен `dtype=half`.

Память для этого профиля:

- общий вес модели — около `61.03 GiB`;
- при `PP=3` это в среднем около `20.34 GiB` веса на одну `GPU`;
- `head_dim = 5120 / 40 = 128`;
- `KV cache` на токен:
  - `2 * 64 * 8 * 128 * 2`
  - `262144 bytes`
  - около `256 KiB` на токен по всему кластеру;
- одна последовательность длиной `8192` токена даёт около `2.0 GiB`
  `KV cache` на весь кластер.

Практический смысл:

- это хороший reasoning-ориентированный запасной профиль;
- по весам он столь же реалистичен, как `Qwen3-32B`;
- но по `KV cache` он тяжелее, чем `Qwen3-32B`, поэтому на роль первого
  запуска хуже.

### Совместимый запасной dense-профиль: `Qwen/Qwen2.5-32B-Instruct`

Этот вариант полезно держать в shortlist, но уже не как основной:

- архитектура такая же, как у `DeepSeek-R1-Distill-Qwen-32B` —
  `Qwen2ForCausalLM`;
- размер весов того же порядка:
  - `total_size = 65527752704`
  - около `61.03 GiB`;
- расчёт памяти и `KV cache` по сути совпадает с
  `DeepSeek-R1-Distill-Qwen-32B`.

Смысл этого профиля:

- это совместимый запасной dense-вариант, если именно на `Qwen3-32B` в
  связке `vLLM 0.10.2 + V100` всплывут отдельные проблемы;
- но с точки зрения памяти и общей формы он не даёт явного выигрыша над
  `Qwen3-32B`.

### Почему не `Qwen3.5` и не большие `DeepSeek`

Исключения лучше назвать прямо:

- `Qwen/Qwen3.5-9B` — это `Qwen3_5ForConditionalGeneration` с отдельными
  `text_config` и `vision_config`; это не тот dense text-only профиль, ради
  которого стоит использовать три `V100`;
- публичный `Qwen/Qwen3.5-35B-A3B` — это
  `Qwen3_5MoeForConditionalGeneration`, то есть уже `MoE`-линейка, а не
  простой dense `CausalLM`;
- `deepseek-ai/DeepSeek-V2.5` — это `DeepseekV2ForCausalLM` с
  `n_routed_experts = 160` и `num_experts_per_tok = 6`, то есть опять же
  `MoE`;
- `deepseek-ai/deepseek-llm-67b-*` как dense-класс на этом железе не подходит:
  - `pytorch_model.bin.index.json` даёт `total_size = 134850002944`
  - это около `125.59 GiB` сырого веса,
  - значит, без квантизации он не помещается в `3x32 GiB` ещё до накладных
    расходов рантайма.

### Тяжёлый профиль около `80 GB`: `tiiuae/falcon-40b`

Почему он вообще рассматривается:

- это один из самых прямых способов действительно подойти к классу
  “модель около `80 GB` в `FP16`” на `3x V100`;
- `vLLM` официально поддерживает `FalconForCausalLM`:
  <https://docs.vllm.ai/en/latest/models/supported_models.html>
- на странице модели указано `42B params`, а также ориентир по памяти
  `85-100GB`:
  <https://huggingface.co/tiiuae/falcon-40b>

Ключевые числа из конфигурации:

- `num_hidden_layers = 60`
- `num_attention_heads = 128`
- `num_kv_heads = 8`
- `hidden_size = 8192`
- `torch_dtype = bfloat16`
- `model.safetensors.index.json` даёт:
  - `total_size = 83671941120`
  - это около `77.93 GiB` веса.

Здесь снова нужен `dtype=half`, а не `BF16`.

### Расчёт памяти для `Falcon-40B`

Если брать расчёт по индексу весов:

- общий вес модели — около `77.93 GiB`;
- при `PP=3` это в среднем около `25.98 GiB` веса на одну `GPU`.

Оценка `KV cache`:

- `head_dim = 8192 / 128 = 64`
- размер `KV` на токен по всей модели:
  - `2 * 60 * 8 * 64 * 2`
  - `122880 bytes`
  - около `120 KiB` на токен по всему кластеру
- одна последовательность длиной `8192` токена даёт около `0.94 GiB` KV cache
  на весь кластер.

Здесь хорошо видно trade-off:

- весов почти на весь бюджет уже хватает;
- `KV cache` у модели сравнительно дешёвый;
- но запас под служебный overhead уже сильно меньше;
- плюс pipeline stages будут не идеально симметричны.

Поэтому `Falcon-40B` — это не профиль “запускаем по умолчанию”, а профиль
“выжать максимум из трёх `V100` и приблизиться к `80 GB`”.

Стартовый безопасный режим для него:

- `tensor_parallel_size=1`
- `pipeline_parallel_size=3`
- `dtype=half`
- `max_model_len=4096`
- `max_num_seqs=1` или `2`
- `gpu_memory_utilization=0.92` или `0.94`

## Длинный контекст: `32k/64k` и около `5` одновременных сессий

Здесь уже важнее не только вес модели, но и worst-case `KV cache`.

Ниже под `5` сессиями понимается именно жёсткий верхний сценарий:

- одновременно живут `5` независимых диалогов;
- каждый из них реально дорастает до `32k` или `64k` токенов;
- общие префиксы не переиспользуются;
- считать нужно полный budget, а не “средний по больнице”.

Для `PP=3` общий `KV cache` распределяется по трем stage. Поэтому практический
вопрос такой:

- сколько веса сидит на одной `GPU`;
- сколько `KV cache` приходится на одну `GPU` в худшем случае;
- сколько остаётся на runtime overhead при `gpu_memory_utilization`
  около `0.90`.

Если брать грубый рабочий budget:

- `32 GiB * 0.90` = около `28.8 GiB` на одну `GPU`;
- всё, что остаётся после весов и `KV cache`, забирают:
  - allocator fragmentation;
  - CUDA graphs;
  - служебные буферы рантайма;
  - NCCL и прочий служебный userspace.

### Что получается на `32B` классе

#### `Qwen3-32B`

- вес на весь кластер: `61.02 GiB`
- вес на одну `GPU` при `PP=3`: `20.34 GiB`
- `KV cache` при `32k x 5`: около `25.00 GiB` на кластер, то есть
  `8.33 GiB` на одну `GPU`
- `KV cache` при `64k x 5`: около `50.00 GiB` на кластер, то есть
  `16.67 GiB` на одну `GPU`

Практический вывод:

- `32k x 5` даёт около `28.67 GiB` на одну `GPU` только на веса и `KV`;
- это уже почти весь budget при `gpu_memory_utilization=0.90`, ещё до
  накладных расходов рантайма;
- `64k x 5` здесь уже не помещается.

Итог:

- `Qwen3-32B` хорош для `8k` и умеренного `16k`;
- для `32k x 5` он уже слишком плотный;
- для `64k x 5` его рассматривать не стоит.

#### `DeepSeek-R1-Distill-Qwen-32B`

- вес на весь кластер: `61.03 GiB`
- вес на одну `GPU`: `20.34 GiB`
- `KV cache` при `32k x 5`: около `40.00 GiB` на кластер, то есть
  `13.33 GiB` на одну `GPU`
- `KV cache` при `64k x 5`: около `80.00 GiB` на кластер, то есть
  `26.67 GiB` на одну `GPU`

Итог:

- даже `32k x 5` здесь уже не помещается;
- этот reasoning-профиль надо рассматривать только для меньшего окна или
  меньшей конкуренции.

### Реалистичный sweet spot для `32k x 5`

#### `Qwen/Qwen3-14B`

- архитектура: `Qwen3ForCausalLM`
- вес на весь кластер: `27.51 GiB`
- вес на одну `GPU`: `9.17 GiB`
- `KV cache` при `32k x 5`: около `25.00 GiB` на кластер, то есть
  `8.33 GiB` на одну `GPU`
- суммарно веса + `KV` на одну `GPU`: около `17.50 GiB`

Итог:

- это уже нормальный профиль для `32k x 5`;
- после весов и `KV cache` остаётся ещё заметный запас под runtime overhead;
- при этом модель всё ещё достаточно большая, чтобы по-настоящему использовать
  `PP=3` и межstage обмен.

Поэтому для long-context профиля на этом железе основной кандидат такой:

- `Qwen/Qwen3-14B`
- `vLLM 0.10.2`
- `VLLM_USE_V1=0`
- `dtype=half`
- `PP=3`, `TP=1`
- `max_model_len=32768`
- `max_num_seqs=5`
- `gpu_memory_utilization=0.90`

#### `deepseek-ai/DeepSeek-R1-Distill-Qwen-14B`

- вес на весь кластер: `27.51 GiB`
- вес на одну `GPU`: `9.17 GiB`
- `KV cache` при `32k x 5`: около `30.00 GiB` на кластер, то есть
  `10.00 GiB` на одну `GPU`
- суммарно веса + `KV` на одну `GPU`: около `19.17 GiB`

Итог:

- для `32k x 5` этот reasoning-профиль тоже реалистичен;
- но запас уже хуже, чем у `Qwen3-14B`;
- значит, его лучше держать как reasoning-альтернативу, а не как основной
  long-context профиль.

### Реалистичный sweet spot для `64k x 5`

На `14B` классе окно `64k` при `5` одновременных сессиях уже опять становится
слишком плотным:

- `Qwen3-14B`:
  - `KV cache` около `50.00 GiB` на кластер, то есть `16.67 GiB` на одну
    `GPU`
  - вместе с весами это около `25.84 GiB` на одну `GPU`
- `DeepSeek-R1-Distill-Qwen-14B`:
  - `KV cache` около `60.00 GiB` на кластер, то есть `20.00 GiB` на одну
    `GPU`
  - вместе с весами это около `29.17 GiB` на одну `GPU`

Первый вариант на практике уже слишком близок к пределу, второй хуже.

Поэтому для `64k x 5` разумно спускаться ещё на один класс ниже.

#### `Qwen/Qwen3-8B`

- вес на весь кластер: `15.26 GiB`
- вес на одну `GPU`: `5.09 GiB`
- `KV cache` при `64k x 5`: около `45.00 GiB` на кластер, то есть
  `15.00 GiB` на одну `GPU`
- суммарно веса + `KV` на одну `GPU`: около `20.09 GiB`

Итог:

- это уже реальный кандидат для `64k x 5`;
- запас под runtime остаётся заметно лучше, чем у `14B`;
- модель меньше, но всё ещё годится для распределённого `PP=3` сценария.

#### `deepseek-ai/DeepSeek-R1-Distill-Llama-8B`

- вес на весь кластер: `14.96 GiB`
- вес на одну `GPU`: `4.99 GiB`
- `KV cache` при `64k x 5`: около `40.00 GiB` на кластер, то есть
  `13.33 GiB` на одну `GPU`
- суммарно веса + `KV` на одну `GPU`: около `18.32 GiB`

Итог:

- это жизнеспособная reasoning-альтернатива для `64k x 5`;
- по памяти она даже комфортнее, чем `Qwen3-8B`.

### Практическая стратегия по окну и конкуренции

Если держаться цели “использовать три `V100` и не уткнуться в память”, то
профили лучше разделить так:

- основной quality-профиль:
  - `Qwen3-32B`
  - `8k` или умеренный `16k`
  - низкая или средняя конкуренция
- long-context профиль `32k x 5`:
  - `Qwen3-14B`
  - reasoning-альтернатива `DeepSeek-R1-Distill-Qwen-14B`
- long-context профиль `64k x 5`:
  - `Qwen3-8B`
  - reasoning-альтернатива `DeepSeek-R1-Distill-Llama-8B`

Если хочется сохранить именно `32B` класс и при этом поднимать окно, то лучше
снижать не модель, а конкуренцию:

- `Qwen3-32B` стоит пробовать скорее как:
  - `32k x 2..3`
  - или `16k x 5`
- а не как `32k x 5`.

## Что именно рекомендуется запускать

### Профиль 1. Основной

- модель: `Qwen/Qwen3-32B`
- `vLLM == 0.10.2`
- `VLLM_USE_V1=0`
- `dtype=half`
- `TP=1`
- `PP=3`
- `max_model_len=8192`
- `max_num_seqs=4`
- `gpu_memory_utilization=0.90`

### Профиль 2. Проверка `V1` на том же профиле

- та же модель `Qwen/Qwen3-32B`
- тот же `vLLM == 0.10.2`
- `VLLM_USE_V1=1`
- остальные параметры те же

Этот режим нужен не как первая ставка, а как controlled second step после
рабочего baseline на `V0`.

### Профиль 3. Reasoning-альтернатива

- модель: `deepseek-ai/DeepSeek-R1-Distill-Qwen-32B`
- `vLLM == 0.10.2`
- `VLLM_USE_V1=0`
- `dtype=half`
- `TP=1`
- `PP=3`
- `max_model_len=8192`
- `max_num_seqs=2..4`
- `gpu_memory_utilization=0.90`

### Профиль 4. Long-context `32k x 5`

- модель: `Qwen/Qwen3-14B`
- `vLLM == 0.10.2`
- `VLLM_USE_V1=0`
- `dtype=half`
- `TP=1`
- `PP=3`
- `max_model_len=32768`
- `max_num_seqs=5`
- `gpu_memory_utilization=0.90`

Reasoning-альтернатива для того же окна:

- `deepseek-ai/DeepSeek-R1-Distill-Qwen-14B`
- те же `PP=3`, `TP=1`
- `max_model_len=32768`
- `max_num_seqs=5`

### Профиль 5. Long-context `64k x 5`

- модель: `Qwen/Qwen3-8B`
- `vLLM == 0.10.2`
- `VLLM_USE_V1=0`
- `dtype=half`
- `TP=1`
- `PP=3`
- `max_model_len=65536`
- `max_num_seqs=5`
- `gpu_memory_utilization=0.90`

Reasoning-альтернатива для того же окна:

- `deepseek-ai/DeepSeek-R1-Distill-Llama-8B`
- те же `PP=3`, `TP=1`
- `max_model_len=65536`
- `max_num_seqs=5`

Отдельно, вне основного recommended набора, остаётся тяжёлый лабораторный
профиль:

- модель: `tiiuae/falcon-40b`
- `vLLM == 0.10.2`
- `VLLM_USE_V1=0`
- `dtype=half`
- `TP=1`
- `PP=3`
- `max_model_len=4096`
- `max_num_seqs=1..2`
- `gpu_memory_utilization=0.92..0.94`

## Рекомендуемая раскладка `KubeRay`

### Head

- отдельный `head` pod;
- без `GPU`;
- без участия в model execution.

### Workers

Не один “абстрактный пул GPU”, а три отдельные worker group:

1. `worker-w1-80`
   - `replicas: 1`
   - node `k8s-dvp-w1-gpu.apiac.ru`
   - GPU `00000000:81:00.0`
   - NIC `0000:82:00.0`
   - verbs `mlx5_1`
2. `worker-w1-c0`
   - `replicas: 1`
   - node `k8s-dvp-w1-gpu.apiac.ru`
   - GPU `00000000:C2:00.0`
   - NIC `0000:C1:00.0`
   - verbs `mlx5_0`
3. `worker-w3-00`
   - `replicas: 1`
   - node `k8s-dvp-w3-gpu.apiac.ru`
   - GPU `00000000:01:00.0`
   - NIC `0000:02:00.0`
   - verbs `mlx5_0`

Для каждой worker group подразумевается:

- GPU приезжает через GPU `DRA` / device plugin;
- NIC приезжает через `sdn` / `UnderlayNetwork` / network `DRA`;
- worker остаётся обычным pod, без `hostNetwork`.

Почему именно так, хотя Ray docs часто рекомендуют “one large pod per node”:

- обычный Ray действительно предпочитает крупные pods:
  <https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/config.html>
- но в нашем случае важнее не общий Ray throughput, а детерминированная
  `GPU/NIC` локальность для `GPUDirect RDMA`;
- `PP=3` как раз естественно раскладывается в `3 worker pod x 1 GPU`.

Это сознательное отклонение от общего правила Ray в пользу текущего
`GPUDirect` сценария.

Если на практике окажется, что текущие `KubeRay` templates ещё неудобно
сочетаются с exact pod-level `GPU/NIC` pairing через `sdn` и `DRA`, запасной
путь должен оставаться внутри той же модели безопасности:

- сохранить тот же `PP=3` профиль;
- сохранить ту же модель;
- сохранить pod-level `sdn` / `DRA`;
- упростить не сетевой слой, а packaging и scheduling:
  - три явные worker group;
  - три явные `UnderlayNetwork` / network claim;
  - три явные GPU class / claim;
  - ручной `placement_group` и жёсткий `nodeSelector`, если потребуется.

То есть fallback здесь orchestration-level, а не node-level сетевой обход.

## Минимальные допуски внутри pod

Целевой профиль для этого сценария такой:

- обычный pod;
- без `hostNetwork`;
- без `privileged`;
- без `hostPath`, кроме тех device mounts, которые приходят через штатные
  плагины `GPU` и `RDMA`;
- без дополнительных Linux capabilities, если `sdn` сам назначает IP на
  direct-интерфейс и контейнер не меняет сеть вручную.

Стартовая позиция по capabilities:

- `NET_ADMIN`: не нужен;
- `NET_RAW`: не нужен;
- `SYS_ADMIN` и подобные: не нужны;
- `IPC_LOCK`: тоже не надо добавлять по умолчанию.

`IPC_LOCK` допустим только как исключение, если на живом запуске окажется, что
конкретный `NCCL` / `verbs` профиль без него не работает. В этом случае его
надо добавлять как минимальное точечное послабление, а не как “базовый
набор для RDMA”.

Текущий live bring-up на `rayproject/ray-llm:2.54.0-py311-cu128` уже показал,
что такое исключение действительно может понадобиться. После исправления
`GLOO` bootstrap на `eth0` `vLLM V1` доходит до `NCCL NET/IB`, но на `w1`
worker падает с:

- `misc/ibvwrap.cc:203 NCCL WARN Call to ibv_create_cq failed with error Cannot allocate memory`
- `RuntimeError: NCCL error: unhandled system error`

В тех же pod видно очень маленький `RLIMIT_MEMLOCK`:

- `w1-c2`: `Max locked memory 65536 bytes`
- `w1-80`: `Max locked memory 65536 bytes`
- `w3-01`: `Max locked memory 8388608 bytes`

При этом baseline-policy в текущем кластере не разрешает `IPC_LOCK`, поэтому
прямое добавление capability в worker pod сейчас не проходит admission.

Текущий practical workaround для bring-up:

- namespace `kuberay-projects` переводится в
  `security.deckhouse.io/pod-policy: privileged`;
- в `ray-worker` добавляется только `IPC_LOCK`;
- остальные capability не добавляются.

Это уже не “идеальный минимальный” security profile, а осознанное временное
послабление, чтобы добить live запуск модели на `3x V100`. Более чистый
долгосрочный вариант остаётся прежним: поднять default `memlock` на
GPU-нодах/в runtime и потом убрать `IPC_LOCK` обратно.

Для текущего стенда это вынесено в отдельный GitOps-компонент рядом с
`KubeRay`:

- `argo-projects/k8s-dvp.apiac.ru/kuberay/argo-app/05-helm-kuberay-node-memlock.yaml`
- `argo-projects/k8s-dvp.apiac.ru/kuberay/charts/kuberay-node-memlock`

Он создаёт `NodeGroupConfiguration`, который вешает systemd drop-in c
`LimitMEMLOCK=infinity` на `containerd` только для
`k8s-dvp-w1-gpu.apiac.ru` и `k8s-dvp-w3-gpu.apiac.ru`.

Практический вывод:

- если `sdn` и `DRA` не дают нужный exact pairing или IPAM в рамках обычного
  pod, это надо чинить на уровне `sdn` / `DRA` / шаблонов `KubeRay`;
- уход в `hostNetwork` считается выходом за целевой контракт и в эту
  рекомендацию не входит.

## Что задаётся в `Ray Serve LLM` / `vLLM`

Ray docs по cross-node параллелизму для LLM прямо показывают:

- `pipeline_parallel_size`
- `tensor_parallel_size`
- `placement_group_config`
- `SPREAD` и `PACK` стратегии

Источник:
<https://docs.ray.io/en/latest/serve/llm/user-guides/cross-node-parallelism.html>

Для этого стенда рекомендуемый `engine_kwargs` такой:

```python
engine_kwargs = dict(
    distributed_executor_backend="ray",
    tensor_parallel_size=1,
    pipeline_parallel_size=3,
    dtype="half",
    gpu_memory_utilization=0.90,
    max_model_len=8192,
    max_num_seqs=4,
)
```

`deployment_config` на старте должен оставаться простым:

- `min_replicas=1`
- `max_replicas=1`

Здесь одна serve-реплика уже сама использует все три `GPU` через
`pipeline_parallel_size=3`. Масштабирование количества реплик надо поднимать
только после того, как базовый distributed profile уже прошёл проверку.

Для тяжёлого профиля на `Falcon-40B`:

```python
engine_kwargs = dict(
    distributed_executor_backend="ray",
    tensor_parallel_size=1,
    pipeline_parallel_size=3,
    dtype="half",
    gpu_memory_utilization=0.92,
    max_model_len=4096,
    max_num_seqs=1,
)
```

Рекомендуемая `placement_group_config`:

```python
placement_group_config = dict(
    bundles=[{"GPU": 1, "CPU": 8}] * 3,
    strategy="SPREAD",
)
```

Почему не `PACK`:

- upstream по умолчанию использует `PACK`:
  <https://docs.ray.io/en/latest/serve/llm/user-guides/cross-node-parallelism.html>
- но для этой задачи нам как раз важно использовать все три заранее
  подготовленные `GPU/NIC` пары, а не дать Ray попытаться максимально
  уплотниться в пределах одной ноды.

Отдельно важно помнить ограничение Ray Serve LLM:

- конкретное назначение rank -> GPU не задаётся через API напрямую;
- значит, детерминированность достигается не в `placement_group_config`,
  а на уровне `KubeRay worker group` и их `node/GPU/NIC` привязки.

## Эквивалентная команда `vllm serve`

Если нужен максимально прямой первичный запуск без дополнительных слоёв:

```bash
VLLM_USE_V1=0 \
vllm serve Qwen/Qwen3-32B \
  --distributed-executor-backend ray \
  --tensor-parallel-size 1 \
  --pipeline-parallel-size 3 \
  --dtype half \
  --gpu-memory-utilization 0.90 \
  --max-model-len 8192 \
  --max-num-seqs 4
```

Для проверки `V1` на том же профиле:

```bash
VLLM_USE_V1=1 \
vllm serve Qwen/Qwen3-32B \
  --distributed-executor-backend ray \
  --tensor-parallel-size 1 \
  --pipeline-parallel-size 3 \
  --dtype half \
  --gpu-memory-utilization 0.90 \
  --max-model-len 8192 \
  --max-num-seqs 4
```

Для reasoning-альтернативы:

```bash
VLLM_USE_V1=0 \
vllm serve deepseek-ai/DeepSeek-R1-Distill-Qwen-32B \
  --distributed-executor-backend ray \
  --tensor-parallel-size 1 \
  --pipeline-parallel-size 3 \
  --dtype half \
  --gpu-memory-utilization 0.90 \
  --max-model-len 8192 \
  --max-num-seqs 4
```

Для long-context профиля `32k x 5`:

```bash
VLLM_USE_V1=0 \
vllm serve Qwen/Qwen3-14B \
  --distributed-executor-backend ray \
  --tensor-parallel-size 1 \
  --pipeline-parallel-size 3 \
  --dtype half \
  --gpu-memory-utilization 0.90 \
  --max-model-len 32768 \
  --max-num-seqs 5
```

Для long-context профиля `64k x 5`:

```bash
VLLM_USE_V1=0 \
vllm serve Qwen/Qwen3-8B \
  --distributed-executor-backend ray \
  --tensor-parallel-size 1 \
  --pipeline-parallel-size 3 \
  --dtype half \
  --gpu-memory-utilization 0.90 \
  --max-model-len 65536 \
  --max-num-seqs 5
```

Для тяжёлого лабораторного профиля:

```bash
VLLM_USE_V1=0 \
vllm serve tiiuae/falcon-40b \
  --distributed-executor-backend ray \
  --tensor-parallel-size 1 \
  --pipeline-parallel-size 3 \
  --dtype half \
  --gpu-memory-utilization 0.92 \
  --max-model-len 4096 \
  --max-num-seqs 1
```

## Что надо выставить в окружении

На worker-подах:

- для основного профиля:
  - `VLLM_USE_V1=0`
- для отдельной проверки `V1`:
  - `VLLM_USE_V1=1`
- `CUDA_DEVICE_ORDER=PCI_BUS_ID`
- `NCCL_DEBUG=INFO`
- `GLOO_SOCKET_IFNAME=<underlay-iface>`
- `NCCL_SOCKET_IFNAME=<underlay-iface>`
- `NCCL_IB_HCA=<mlx5 device for this pod>`

Примеры:

- для `worker-w1-80`:
  - `GLOO_SOCKET_IFNAME=enp130s0np0`
  - `NCCL_SOCKET_IFNAME=enp130s0np0`
  - `NCCL_IB_HCA=mlx5_1`
- для `worker-w1-c0`:
  - `GLOO_SOCKET_IFNAME=enp193s0np0`
  - `NCCL_SOCKET_IFNAME=enp193s0np0`
  - `NCCL_IB_HCA=mlx5_0`
- для `worker-w3-00`:
  - `GLOO_SOCKET_IFNAME=enp2s0np0`
  - `NCCL_SOCKET_IFNAME=enp2s0np0`
  - `NCCL_IB_HCA=mlx5_0`

`VLLM_HOST_IP` надо использовать аккуратно:

- если Ray control-plane идёт по обычному `podIP`, лучше держать
  `VLLM_HOST_IP` равным именно ему;
- под direct `RDMA` путь здесь уводятся `NCCL/Gloo` интерфейсы, а не Ray actor
  discovery.

Если понадобится доказать, что используется именно `RDMA/GPUDirect` путь,
на время проверки можно поднять:

- `NCCL_DEBUG=TRACE`

И смотреть в логах признаки вроде `NET/IB/GDRDMA`. Это описано в vLLM docs по
distributed serving:
<https://docs.vllm.ai/en/latest/serving/distributed_serving.html>

## Что не надо делать на первом проходе

Не рекомендуется:

- начинать с `TP=3`;
- брать `BF16`;
- вручную фиксировать `VLLM_ATTENTION_BACKEND`, пока не доказано, что
  автоматический выбор backend действительно мешает;
- брать модель больше `40B` dense в `FP16`;
- начинать с `128k` или `32k` контекста;
- начинать с `vLLM >= 0.11`, если нет уже достаточной уверенности в `V1` на
  `Volta`;
- начинать с `MoE`-моделей и `expert parallelism`;
- начинать с публичных `Qwen3.5`-профилей как с первого dense text-only пути.

## Практический план запуска

### Шаг 1. Поднять основной профиль

Сначала запускается:

- `Qwen3-32B`
- `vLLM 0.10.2`
- `VLLM_USE_V1=0`
- `PP=3`, `TP=1`

Это и есть основной профиль, который должен стать базой для дальнейших тестов
через `KubeRay + vLLM`.

### Шаг 2. Проверить тот же профиль с `V1`

После живого baseline на `V0` проверяется тот же самый профиль, но уже с:

- `VLLM_USE_V1=1`
- той же моделью;
- тем же `PP=3`, `TP=1`.

Это позволяет отдельно оценить, насколько `V1/FlexAttention` на `Volta`
реально пригоден для этого стенда.

### Шаг 3. Проверить, что используется нужный path

Нужно подтвердить:

- все три worker действительно задействованы;
- `dtype=half`, а не `BF16`;
- в логах нет отката на CPU или случайного `single-node` профиля;
- `NCCL` видит `IB` path;
- под нагрузкой заняты все три `GPU`.

### Шаг 4. Только потом переходить к альтернативной модели и тяжёлому профилю

Сначала имеет смысл проверить:

- `DeepSeek-R1-Distill-Qwen-32B`
- тот же `V0`
- тот же `PP=3`, `TP=1`

И лишь после этого пробовать:

- `Falcon-40B`
- короткий контекст
- низкую конкуренцию

И уже им проверять потолок “насколько плотно можно набить три `V100`”.

## Итоговая рекомендация

Если цель — не просто “что-нибудь распределённое поднять”, а именно
реалистично использовать `3x V100` и текущий `GPUDirect RDMA` baseline, то
правильная стартовая ставка такая:

- модель:
  - `Qwen/Qwen3-32B`
- движок:
  - `vLLM 0.10.2`
  - `VLLM_USE_V1=0` на первом запуске
  - `VLLM_USE_V1=1` как отдельный следующий шаг проверки
- точность:
  - `FP16` через `dtype=half`
- схема:
  - `PP=3`
  - `TP=1`
- кластерная форма:
  - `1 head`
  - `3 worker group x 1 GPU`
  - каждая worker group pinned к своей `GPU/NIC` паре

Если нужен reasoning-профиль из семейства `DeepSeek`, то первым кандидатом
здесь выступает `DeepSeek-R1-Distill-Qwen-32B`, а не большие `DeepSeek MoE`
и не `67B` dense-модели.

Если нужен длинный контекст и около `5` одновременных сессий, то профиль надо
менять уже по размеру модели:

- для `32k x 5`:
  - `Qwen3-14B`
  - reasoning-альтернатива `DeepSeek-R1-Distill-Qwen-14B`
- для `64k x 5`:
  - `Qwen3-8B`
  - reasoning-альтернатива `DeepSeek-R1-Distill-Llama-8B`

Пытаться удержать при этом `32B` dense-класс невыгодно: там в потолок
упирается уже не вес модели, а `KV cache`.

Если нужен именно профиль “около `80 GB` FP16”, то его роль выполняет не
основной production-like путь, а тяжёлый лабораторный профиль на
`Falcon-40B`. Он возможен, но его надо рассматривать как третий этап после
успешного запуска `Qwen3-32B` и проверки `V1` на том же стенде.
