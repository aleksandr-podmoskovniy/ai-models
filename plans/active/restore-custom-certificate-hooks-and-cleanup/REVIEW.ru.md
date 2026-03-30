# Review

## Что проверено
- `make lint`
- `make test`
- `make helm-template`
- `werf config render --dev --env dev` с проверкой, что bundle использует
  `batch-hooks` и `/hooks/batchhooks`, а старый `images/hooks` path больше не
  участвует в render output

## Findings
Критичных замечаний по текущему срезу не осталось.

## Остаточные риски
- `make kubeconform` в этом репозитории по-прежнему зависает на локальном
  проходе `kubeconform`; это старое поведение validate loop, не регресс от
  возврата `hooks/batch`.
- В рабочем дереве параллельно остаются несвязанные изменения по storage
  contract (`credentialsSecretName` и связанные docs/templates), поэтому
  финальный commit нужно собирать осознанно по нужному scope.
