# REVIEW

## Findings

Критичных замечаний по текущему diff нет.

## Что подтверждено

- defaults для managed/external PostgreSQL database и user переведены в
  RFC1123-safe `ai-models`;
- managed `Postgres` больше не хардкодит `cluster.topology: Zonal`, а берёт
  `defaultTopology` выбранного `PostgresClass` через `lookup` с безопасным
  fallback `Ignored`;
- создаваемый модулем `PostgresClass` с пустым `allowedZones` теперь тоже
  использует `defaultTopology: Ignored`;
- local render validator теперь ловит invalid RFC1123 names и
  `PostgresClass/defaultTopology` drift;
- `make verify` проходит.

## Residual risks

- если в другом кластере выбранный `PostgresClass` отсутствует в момент render,
  модуль использует fallback `Ignored`; это безопасно для текущего кластера, но
  topology semantics в других окружениях зависят от contract конкретного
  `PostgresClass`;
- существующие пользовательские overrides с `_` в database/user по-прежнему
  будут отклоняться admission webhook'ом, и это корректно.
