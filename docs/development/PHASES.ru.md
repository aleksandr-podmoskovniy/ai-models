# Этапы разработки ai-models

## Этап 1. Managed backend inside the module

### Что делаем
- поднимаем внутренний backend как компонент модуля;
- подключаем PostgreSQL и S3-compatible storage;
- делаем values, OpenAPI, templates и werf в DKP-манере;
- включаем базовый monitoring и logging;
- обеспечиваем рабочий UI и базовый путь входа.

### Что не делаем
- не вводим ещё публичный каталог `Model` / `ClusterModel` как обязательную часть релиза;
- не делаем distroless и глубокий patching как обязательный результат;
- не смешиваем задачу backend rollout с задачами inference.

### Критерий выхода
- модуль можно включить;
- backend запускается и подключается к зависимостям;
- UI доступен;
- базовые smoke-проверки проходят;
- templates и values проходят рендер и валидацию.

## Этап 2. Model / ClusterModel

### Что делаем
- проектируем и реализуем `Model` и `ClusterModel`;
- вводим контроллер публикации и синхронизации с внутренним backend;
- формируем status, conditions и validation rules;
- делаем platform UX поверх backend `ai-models`.

### Что не делаем
- не меняем без нужды базовый deployment shape внутреннего backend;
- не раздуваем API деталями внутренних implementation choices.

### Критерий выхода
- можно опубликовать модель через DKP API;
- `status` объясняет состояние публикации;
- каталог интегрирован с внутренним backend и не требует прямой ручной работы в backend для обычного сценария.

## Этап 3. Hardening and long-term support

### Что делаем
- controlled upstream patching;
- distroless для собственного кода и затем, если оправдано, для backend packaging;
- CVE и dependency hygiene;
- upgrade and rollback discipline;
- supply-chain и security improvements.

### Критерий выхода
- проект выдерживает рост кода и релизов без хаотичного усложнения;
- правила rebase/patch/hardening задокументированы и воспроизводимы.
