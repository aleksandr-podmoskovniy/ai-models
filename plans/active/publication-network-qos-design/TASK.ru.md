## 1. Заголовок

Publication runtime network QoS design для publication worker

## 2. Контекст

После перехода на direct upload и повышения `maxConcurrentWorkers` до `4`
узким местом для publication runtime стала уже не память worker'ов, а сетевой
трафик:

- `Hugging Face -> publication worker -> S3`;
- `S3 -> DMCR` при seal/verify fallback;
- несколько параллельных worker pod'ов могут конкурировать за один uplink.

Пользовательский запрос: не городить отдельную самодельную сетевую подсистему,
а понять, какие штатные механизмы уже есть в `deckhouse` и в паттернах
`virtualization`/`dvcr`, и на их основе зафиксировать defendable target design.

## 3. Постановка задачи

Нужно исследовать и зафиксировать целевую картину ограничения трафика для
publication worker'ов:

- что реально умеют `NetworkPolicy`, `cni-cilium` и штатные pod bandwidth
  annotations в нашей платформенной базе;
- что уже используется в `virtualization` и почему это не равно bandwidth QoS;
- можно ли получить желаемое поведение "использовать до 70% канала и делить
  поровну между активными worker'ами" без отдельного инфраструктурного
  монстра;
- какой минимальный productized шаг можно добавить в `ai-models` уже сейчас,
  а что требует отдельного runtime coordination слоя.

## 4. Scope

В задачу входит:

- локальное исследование `deckhouse` по `cni-cilium`/bandwidth manager;
- локальное исследование `virtualization`/`dvcr` по network policy и bandwidth
  knobs;
- внешние референсы только из primary docs по Kubernetes/Cilium;
- фиксация target design и staged recommendation для `ai-models`;
- оформление task bundle для дальнейшей реализации.

## 5. Non-goals

В задачу не входит:

- немедленная реализация runtime throttling в этом diff;
- изменение public `Model` / `ClusterModel` API;
- обещание "идеального" динамического fair-share без доказанного coordination
  механизма;
- внедрение отдельного node daemon / privileged tc-controller без явного
  архитектурного решения.

## 6. Затрагиваемые области

- `plans/active/publication-network-qos-design/*`
- read-only inspection:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`
- read-only inspection:
  - `images/controller/internal/dataplane/publishworker/*`
  - `images/controller/internal/adapters/sourcefetch/*`
  - `images/controller/internal/adapters/modelpack/oci/*`
  - `templates/*`
  - `openapi/*`

## 7. Критерии приёмки

Задача считается завершённой, когда:

- явно зафиксировано, что `NetworkPolicy` не является механизмом ограничения
  полосы и не смешивается с bandwidth QoS;
- подтверждено, есть ли в `deckhouse` уже включённый платформенный bandwidth
  mechanism, который можно переиспользовать без отдельного инфраструктурного
  сервиса;
- показано, как `virtualization` решает близкую задачу: отдельный runtime knob
  для migration bandwidth плюс отдельные concurrency limits, а не попытка
  переложить всё на `NetworkPolicy`;
- сформулирован рекомендуемый target design для `ai-models`, разделённый на:
  - ближайший productized шаг;
  - follow-up для динамического fair-share;
  - явно отклонённые варианты с объяснением;
- bundle содержит проверяемую рекомендацию, на основе которой можно делать
  следующий implementation slice без повторного discovery.

## 8. Риски

- можно перепутать allow/deny network isolation и bandwidth shaping;
- можно опереться на feature Cilium, которую Deckhouse не экспонирует или не
  гарантирует как стабильную часть платформы;
- можно недооценить различие между статическим per-pod cap и реальным
  dynamic fair-share между живыми worker pod'ами;
- можно предложить coordination модель, которая создаст слишком много
  runtime-связности между controller и worker'ами.
