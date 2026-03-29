# AGENTS.md

## Назначение репозитория

`ai-models` — это **модуль Deckhouse Kubernetes Platform**, который добавляет capability платформы: единый каталог опубликованных моделей и внутренний registry backend.

Репозиторий должен оставаться **DKP module root**, а не превращаться в отдельный kubebuilder/operator repo.

## Текущая дорожка разработки

### Этап 1. Внутренний managed backend
Сначала модуль должен уметь:
- поднять внутренний backend как компонент модуля;
- подключить его к PostgreSQL и S3-совместимому хранилищу;
- сделать рабочий UI и базовую эксплуатацию в DKP;
- интегрировать логирование, мониторинг и базовую аутентификацию.

На этом этапе **публичный API каталога ещё не обязателен**. Главная цель — получить работающий, проверяемый и поддерживаемый backend.

### Этап 2. Публичный DKP API каталога моделей
После рабочего backend добавляется:
- `Model`;
- `ClusterModel`;
- контроллер публикации и синхронизации с внутренним backend;
- статусы, conditions и platform UX.

### Этап 3. Упрочнение и переупаковка
После рабочих этапов 1 и 2 добавляются:
- distroless для собственного кода;
- controlled patching/rebasing для внутреннего backend engine;
- supply-chain hardening;
- дополнительные security-проверки и улучшения эксплуатации.

## Постоянные правила

- Не выводить наружу сырые сущности внутреннего backend engine как платформенный контракт.
- Не смешивать код контроллеров, DKP templates, docs и upstream patching в одних и тех же каталогах.
- Любой executable runtime code размещать под `images/*`; top-level `api/` оставлять только для DKP API contract.
- Не делать большие архитектурные изменения без явного плана.
- Не редактировать upstream-артефакты без описанного patch/rebase процесса.
- Не тащить в этап 1 задачи этапа 2 и 3 без явного решения в плане.
- Любая нетривиальная задача сначала превращается в task bundle в `plans/active/<slug>/`.

## Обязательный рабочий цикл

1. Сначала нормализовать задачу в `TASK.ru.md`.
2. Затем сделать `PLAN.ru.md` с этапами, файлами, проверками и rollback point.
3. Выбрать режим orchestration: `solo`, `light` или `full`.
4. Если задача требует delegation, вызвать read-only subagents до первого изменения кода.
5. Зафиксировать выводы subagents в текущем `plans/active/<slug>/PLAN.ru.md` или связанных notes до реализации.
6. Реализовывать только один slice за раз.
7. После каждого slice выполнять узкие проверки.
8. Перед завершением выполнить repo-level проверки.
9. Обновить документацию, если изменились архитектура, процесс, API или эксплуатация.
10. Завершить задачу через `review-gate`, а для substantial tasks с delegation дополнительно через `reviewer`.

## Когда обязательно использовать planning

Planning обязателен, если:
- задача меняет больше одного каталога;
- задача тянет архитектурное решение;
- задача меняет контракт values/OpenAPI/API;
- задача затрагивает внутренний backend, auth, storage или observability;
- задача предполагает patching upstream.

## Режимы orchestration

- `solo` — одна узкая задача без архитектурной неопределённости; сабагенты не нужны.
- `light` — task bundle плюс один read-only subagent по главному риску задачи.
- `full` — task bundle, несколько read-only subagents по разным границам и финальный `reviewer`.

## Когда delegation обязателен

Delegation обязателен, если:
- задача меняет больше одной области репозитория и решение не является чисто механическим;
- задача меняет module layout, build/publish shell, values/OpenAPI/API contract или stage boundary;
- задача затрагивает auth, storage, ingress/TLS, observability, HA или global-vs-local ownership;
- задача связана с upstream patching, rebase или 3p packaging discipline;
- задача проектирует или меняет `Model`, `ClusterModel`, status/conditions или controller boundaries.

## Когда сабагенты не нужны

Сабагенты не нужны, если:
- изменение укладывается в один узкий slice и один основной риск уже понятен;
- это mechanical cleanup, rename, formatting или docs-only wording без архитектурного выбора;
- это локальный bugfix с очевидной причиной и прямой узкой проверкой;
- использование delegation замедлит задачу сильнее, чем добавит сигнала.

## Подбор subagents

### Reusable core

- `task_framer` — переводит обычную человеческую задачу в task bundle.
- `repo_architect` — проверяет границы модуля, layout и anti-patchwork решения.
- `integration_architect` — проверяет runtime/build/integration boundaries, global-vs-local ownership, auth/storage/ingress/HA/observability и 3p packaging discipline.
- `api_designer` — проверяет Kubernetes/DKP API, scope, ownership, spec/status split, immutability и conditions.
- `module_implementer` — выполняет scoped implementation.
- `reviewer` — делает финальный read-only review.

### Project-specific overlays

- `backend_integrator` — используется только для внутреннего backend engine внутри `ai-models`.

## Канонический mapping

- `task-intake-and-slicing` -> `task_framer` только для fuzzy/broad intake; в остальных случаях main agent может оформить bundle сам.
- layout/module shell/repo boundaries -> `repo_architect`
- runtime/build/auth/storage/ingress/observability -> `integration_architect`
- backend-engine-specific runtime details -> `backend_integrator`
- Kubernetes/DKP API/CRD/status/conditions -> `api_designer`
- scoped write delegation после ясных boundaries -> `module_implementer`
- финальная substantial review -> `review-gate`, а при delegation или multi-area task ещё и `reviewer`

## Build / verify

Использовать реальные `make`-команды репозитория:

- `make ensure-tools`
- `make fmt`
- `make lint`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`

Выбирать **самую узкую** проверку, которая подтверждает текущий slice. Перед сдачей делать `make verify`, если это уже реализуемо для текущего состояния репозитория.

## Основные документы

- `docs/development/TZ.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/PHASES.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/TASK_TEMPLATE.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`
- `plans/README.md`
- `.codex/README.md`

## Definition of done

Задача считается завершённой, когда:
- есть task bundle и выполненный план;
- изменение укладывается в текущий этап разработки;
- структура репозитория осталась чище или понятнее, чем была;
- код, templates, values, docs и сборка согласованы между собой;
- пройдены релевантные проверки;
- финальный review не оставил критичных замечаний.
