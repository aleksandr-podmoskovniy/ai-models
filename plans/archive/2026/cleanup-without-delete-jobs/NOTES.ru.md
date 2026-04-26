# Notes

## Проверка virtualization

В `virtualization` DVCR GC устроен не как `Job` на каждую удаляемую сущность.
Основные элементы:

- `templates/dvcr/deployment.yaml` держит `dvcr-garbage-collection` как
  долгоживущий container внутри DVCR Deployment.
- `dvcr-garbage-collection` следит за Secret `dvcr-garbage-collection`,
  переводит registry в GC mode, пишет результат в Secret и ждёт завершения.
- Controller lifecycle читает Secret, ждёт provisioners, ставит Deployment
  condition `GarbageCollection`, удаляет Secret после persisted result.
- Новые provisioning/import операции откладываются, пока Secret показывает
  активный GC lifecycle.

Вывод: для `ai-models` отдельный Kubernetes `Job` на каждый delete выбивается
из паттерна. Ближайший target pattern: `catalogcleanup` выполняет lightweight
artifact/staging delete внутри controller-owned operation, сохраняет marker в
cleanup Secret и только затем использует уже существующий DMCR GC lifecycle.
