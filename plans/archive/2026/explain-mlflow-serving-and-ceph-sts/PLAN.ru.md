## 1. Current phase
Этап 1. Managed backend inside the module.

## 2. Slices

### Slice 1. Проверить STS support в Ceph RGW
- Цель: понять, поддерживает ли RGW 19.2.2 short-lived credentials и какие ограничения есть.
- Проверки:
  - официальная документация Ceph

### Slice 2. Зафиксировать serving URI contract
- Цель: объяснить, откуда брать URI для KServe/KubeRay в текущем модуле.
- Проверки:
  - текущий importer/runtime код

### Slice 3. Объяснить MLflow object model и promotion
- Цель: разложить `Experiment` / `Run` / `Logged Model` / `Registry` и их практический смысл.
- Проверки:
  - официальная документация MLflow

## 3. Rollback point
Диагностический/объяснительный срез без code/runtime changes.

## 4. Final validation
- Ответ опирается на official docs и текущий repo contract.
