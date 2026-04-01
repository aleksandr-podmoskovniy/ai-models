## 1. Заголовок
Починить repo verify без зависимости от ripgrep в CI.

## 2. Контекст
`make verify` падает в GitHub Actions на таргете `backend-runtime-entrypoints-check`, потому что он вызывает `rg`, а в runner image `ripgrep` не установлен.

## 3. Постановка задачи
Убрать из verify-path необязательную зависимость на `rg` и оставить ту же проверку наличия cleanup entrypoint в build wiring.

## 4. Scope
- Обновить `Makefile`.
- Сохранить смысл `backend-runtime-entrypoints-check`.
- Проверить таргет локально без дополнительных предположений о наличии `rg`.

## 5. Non-goals
- Не менять build/layout OIDC auth.
- Не расширять verify другими unrelated checks.

## 6. Затрагиваемые области
- `Makefile`
- `plans/active/fix-verify-without-rg/`

## 7. Критерии приёмки
- `backend-runtime-entrypoints-check` не зависит от `rg`.
- `make verify` больше не падает на `rg: command not found`.

## 8. Риски
- Слишком грубая замена может ослабить проверку; нужно сохранить тот же literal-match intent.
