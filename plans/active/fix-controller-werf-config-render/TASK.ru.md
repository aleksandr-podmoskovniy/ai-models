## 1. Заголовок

Починить `werf config render` для controller image shell

## 2. Контекст

Текущий live baseline уже использует `images/controller/werf.inc.yaml` как
канонический build shell для `controller` и `controller-runtime`.

В CI упал `deckhouse/modules-actions/build@v4` на шаге `werf ci-env` с ошибкой:

- `unable to load werf config: yaml: unmarshal errors`
- duplicate keys `image`, `final`, `fromImage`, `import`

Rendered fragment показывает, что `image: controller-build-artifact` и
`image: controller` оказались в одном YAML document. Это regression не
архитектуры публикации, а document-boundary в live build shell.

## 3. Постановка задачи

Нужно восстановить корректный YAML document boundary в
`images/controller/werf.inc.yaml`, чтобы `werf config render` и GitHub Actions
build снова проходили.

## 4. Scope

- `images/controller/werf.inc.yaml`
- `plans/active/fix-controller-werf-config-render/*`

## 5. Non-goals

- не менять controller runtime architecture;
- не менять image names, stages или final image contents;
- не трогать unrelated `werf` surfaces в `images/dmcr` или других image trees;
- не продолжать здесь publication/runtime refactor.

## 6. Затрагиваемые области

- `images/controller/werf.inc.yaml`
- `plans/active/fix-controller-werf-config-render/TASK.ru.md`
- `plans/active/fix-controller-werf-config-render/PLAN.ru.md`

## 7. Критерии приёмки

- `images/controller/werf.inc.yaml` снова рендерится как отдельные YAML
  documents для `controller-build-artifact`, `controller` и
  `controller-runtime`;
- `werf config render` больше не падает на duplicate-key error;
- `make verify` проходит на финальном состоянии.

## 8. Риски

- можно случайно изменить не только document separator, но и stage semantics;
- можно починить локальный render, но оставить другой syntax drift, если не
  прогнать repo-level verify.
