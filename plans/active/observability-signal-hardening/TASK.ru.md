# Observability Signal Hardening

## 1. Заголовок

Упрочнить метрики и логи ai-models до эксплуатационного сигнала уровня DKP
module.

## 2. Контекст

В модуле уже есть catalog/runtime/storage metrics и structured slog output.
Проблема не в отсутствии метрик как таковых, а в качестве сигнала: если
collector частично не может прочитать Kubernetes API или ledger, Prometheus
получает неполную картину, а причина остаётся только в логах controller. Для
продуктового HA-модуля это слабый контракт: e2e и эксплуатация должны видеть
деградацию observability plane машинно.

Локальный ориентир по virtualization: collectors разделяют scrape/report
responsibility, используют стабильные low-cardinality labels и structured
logger attrs (`collector`, `err`, object identity только там, где это объектная
метрика).

## 3. Постановка задачи

Сделать первый hardening-slice для метрик/логов:

- зафиксировать observability-контракт;
- добавить машинный scrape-health сигнал для controller collectors;
- не повышать cardinality и не логировать секреты/сырой payload;
- покрыть это unit-тестами и узкими проверками.

## 4. Scope

- `images/controller/internal/monitoring/*`;
- план/заметки по observability contract;
- узкие Go-тесты controller monitoring packages.

## 5. Non-goals

- Не перепроектировать все алерты за один slice.
- Не менять public names существующих метрик без migration path.
- Не добавлять per-object/per-error высококардинальные labels.
- Не тащить logging facade во все packages одним большим проходом.
- Не менять RBAC/templates в этом slice.

## 6. Затрагиваемые области

- Controller monitoring collectors.
- Structured collector logs.
- Task bundle evidence for future e2e/alerting work.

## 7. Критерии приёмки

- У каждого controller collector есть единый scrape-health сигнал:
  `d8_ai_models_collector_up`,
  `d8_ai_models_collector_scrape_duration_seconds`,
  `d8_ai_models_collector_last_success_timestamp_seconds`.
- Labels у health metrics ограничены `collector`.
- Ошибка чтения одного источника данных делает collector `up=0`, но не
  ломает весь metrics endpoint.
- Успешный scrape обновляет last-success timestamp.
- Существующие domain metrics не переименованы и не расширены шумными labels.
- Тесты доказывают success/failure поведение shared health contract.

## 8. Риски

- Повторное описание одного health descriptor несколькими collectors должно
  оставаться совместимым с Prometheus registry.
- Нельзя маскировать настоящую ошибку scrape успешным `up=1`.
- Нельзя смешать scrape-health controller collectors с состоянием бизнес-объектов.
