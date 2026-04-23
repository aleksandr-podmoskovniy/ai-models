# AGENTS.md

## Назначение репозитория

`ai-models` — это **модуль Deckhouse Kubernetes Platform**, который добавляет
capability платформы: единый каталог моделей, controller-owned
publication/runtime path и внутренний registry-backed publication backend.

Репозиторий должен оставаться **DKP module root**, а не превращаться в отдельный kubebuilder/operator repo.

## Текущая дорожка разработки

### Этап 1. Publication/runtime baseline
Сначала модуль должен уметь:
- держать DKP-native `Model` / `ClusterModel` catalog API;
- публиковать канонический OCI `ModelPack` artifact через controller-owned
  runtime во внутренний `DMCR`;
- поддерживать source mirror / upload staging в S3-совместимом хранилище;
- доставлять опубликованный artifact в workload через текущий runtime delivery
  baseline, включая `materialize-artifact` fallback и node-cache substrate /
  runtime shell;
- интегрировать логирование, мониторинг, базовую аутентификацию и
  object-storage wiring.

Главная цель этапа — получить работающий, проверяемый и поддерживаемый
publication/runtime baseline без возврата к backend-first narrative.

### Этап 2. Distribution/runtime expansion
После рабочего baseline добавляются:
- `DMZ`/mirror distribution topology поверх canonical internal artifact;
- optional node-local shared mount/cache service поверх текущего fallback path;
- статусы, conditions, observability и platform UX для новых runtime topologies.

### Этап 3. Упрочнение и переупаковка
После рабочих этапов 1 и 2 добавляются:
- distroless для собственного кода;
- controlled patching/rebasing для внутреннего backend engine;
- supply-chain hardening;
- дополнительные security-проверки и улучшения эксплуатации.

## Постоянные правила

- Не выводить наружу сырые сущности внутреннего publication backend/runtime
  как платформенный контракт.
- Не смешивать код контроллеров, DKP templates, docs и upstream patching в одних и тех же каталогах.
- Любой executable runtime code размещать под `images/*`; top-level `api/` оставлять только для DKP API contract.
- Не делать большие архитектурные изменения без явного плана.
- Не редактировать upstream-артефакты без описанного patch/rebase процесса.
- Не тащить в этап 1 задачи этапа 2 и 3 без явного решения в плане.
- Любая нетривиальная задача сначала превращается в task bundle в `plans/active/<slug>/`.
- Repo-local workflow rules имеют строгий precedence:
  - `AGENTS.md`
  - `.codex/README.md`
  - `.agents/skills/*`
  - `.codex/agents/*.toml`
  Нижний уровень не должен противоречить верхнему.
- Любая задача, которая меняет `AGENTS.md`, `.codex/*`, `.agents/skills/*` или
  `.codex/agents/*`, а также workflow docs
  `docs/development/CODEX_WORKFLOW.ru.md`,
  `docs/development/TASK_TEMPLATE.ru.md`,
  `docs/development/REVIEW_CHECKLIST.ru.md` или `plans/README.md`,
  считается governance task и требует отдельного task bundle, а не incidental
  wording fix inside another workstream.

## Engineering doctrine

### Boundary discipline

- Каждая boundary должна иметь явную причину существования:
  - ownership
  - runtime contract
  - replaceable adapter
  - durable shared helper
- Не создавать оболочки поверх уже живого контракта только ради “удобных имён”.
- Не смешивать в одном месте:
  - policy
  - transport/serialization
  - concrete K8s object shaping
  - product-facing API semantics
- Любой новый package/file должен быть defendable по двум вопросам:
  - почему это отдельная responsibility;
  - почему эта responsibility не живёт лучше в соседней boundary.

### Long-context resilience

- Durable engineering rules должны жить в repo-local docs/skills/references, а
  не только в chat context или в одном giant task bundle.
- Active bundles должны оставаться компактными рабочими поверхностями.
  Если bundle превращается в historical log, его надо архивировать и открывать
  новый continuation bundle.
- Новая работа должна продолжать canonical active bundle для workstream, а не
  создавать sibling source of truth.

### Portable reusable baseline

- Reusable core skills и agents должны описывать переносимую инженерную
  методику и guardrails, а не narrative одного конкретного модуля.
- Project-specific product/runtime/API rules должны жить в явных overlays или
  repo docs, а не протекать в generic core.
- Если этот baseline копируется в другой DKP module repo, переносить нужно
  precedence chain, reusable core, governance inventory и workflow docs, а
  project-specific overlays заменять осознанно.
- Слепое копирование repo-local governance surface в новый модуль запрещено:
  сначала нужен отдельный governance porting slice.
- Этот porting slice обязан явно зафиксировать:
  - source repo baseline;
  - какие overlays заменяются или удаляются;
  - какие repo docs и верхние instruction surfaces переписываются под новый
    модуль до первого product/runtime change.

### Systematic testing

- Тесты считаются частью архитектуры, а не приложением к коду.
- Тестовое дерево должно резаться по decision surface, а не по случайному
  helper reuse.
- Happy-path coverage сама по себе не считается достаточным доказательством.
- Для lifecycle/stateful code обязательны:
  - negative branches
  - idempotency/replay
  - malformed input/result paths
  - owner/deletion/finalizer behavior where relevant
- Helper-only test files не должны становиться скрытым слоем бизнес-логики.

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

## Workflow governance

Если задача меняет repo-local Codex surface:

- `AGENTS.md`
- `.codex/README.md`
- `.agents/skills/*`
- `.codex/agents/*.toml`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/TASK_TEMPLATE.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`
- `plans/README.md`

то обязательно:

1. вынести её в отдельный task bundle;
2. явно перечислить touched instruction surfaces;
3. проверить на противоречия все изменённые уровни, а не только один файл;
4. явно отделить reusable core от project-specific overlays, если меняется
   сам governance baseline;
5. не плодить новые skills или agent roles, если проблему можно решить
   tightening existing boundaries;
6. завершать задачу только после manual consistency review этих surfaces.
7. прогонять `make lint-codex-governance` как machine-checkable guardrail.

## Когда обязательно использовать planning

Planning обязателен, если:
- задача меняет больше одного каталога;
- задача тянет архитектурное решение;
- задача меняет контракт values/OpenAPI/API;
- задача затрагивает publication backend/raw-ingest, auth, storage или
  observability;
- задача предполагает patching upstream.
- задача меняет repo-local workflow/governance surface
  (`AGENTS.md`, `.codex/*`, `.agents/skills/*`, `.codex/agents/*`,
  `docs/development/CODEX_WORKFLOW.ru.md`,
  `docs/development/TASK_TEMPLATE.ru.md`,
  `docs/development/REVIEW_CHECKLIST.ru.md`, `plans/README.md`).

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

Исключение:

- governance/doc-only tasks над repo-local workflow surface могут оставаться в
  `solo`, если цель — tightening and consistency review самих инструкций, а не
  проектирование нового runtime/API behavior.

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

- `backend_integrator` — используется только для внутренних publication
  backend/runtime details внутри `ai-models`.
- `model-catalog-api` — используется только для `ai-models`-specific
  semantics вокруг `Model`, `ClusterModel`, status/conditions и sync с
  internal publication/runtime machinery.

## Канонический mapping

- `task-intake-and-slicing` -> `task_framer` только для fuzzy/broad intake; в остальных случаях main agent может оформить bundle сам.
- layout/module shell/repo boundaries -> `repo_architect`
- runtime/build/auth/storage/ingress/observability -> `integration_architect`
- publication-backend-specific runtime details -> `backend_integrator`
- `Model` / `ClusterModel` overlay semantics -> `model-catalog-api`
- Kubernetes/DKP API/CRD/status/conditions -> `api_designer`
- scoped write delegation после ясных boundaries -> `module_implementer`
- финальная substantial review -> `review-gate`, а при delegation или multi-area task ещё и `reviewer`

## Build / verify

Использовать реальные `make`-команды репозитория:

- `make ensure-tools`
- `make fmt`
- `make lint`
- `make lint-codex-governance`
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
- если менялся workflow surface, между `AGENTS.md`, `.codex/README.md`,
  `.agents/skills/*` и `.codex/agents/*.toml` не осталось явных противоречий.
- если менялся reusable governance baseline, reusable core остался переносимым,
  а module-specific doctrine осталась в явных overlays.
- если менялись architecture/testing/workflow rules, они зафиксированы в
  durable repo-local surfaces и не зависят от chat-only context.
