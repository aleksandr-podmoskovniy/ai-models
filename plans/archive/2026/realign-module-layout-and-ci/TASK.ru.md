# Перестроить module layout и CI shell под DKP-паттерн

## Контекст

Текущий `ai-models` уже собирается и проходит базовые проверки, но layout ещё
слишком плоский и не отражает реальные границы будущего модуля:
- backend templates лежат россыпью в корне `templates/`;
- нет явного repo layout для будущих `api/` и `controllers/`;
- hook boundary не описана, из-за чего неочевидно, когда нужен hook, а когда
  достаточно declarative resources;
- `GitHub Actions` и `.gitlab-ci.yml` выглядят как временная обвязка, а не как
  системный DKP module CI shell.

Пользователь явно просит выровнять структуру по паттернам `virtualization`,
подготовить репозиторий к дальнейшей разработке backend и controller, а для
GitHub Actions отдельно брать за ориентир `gpu-control-plane`.

## Постановка задачи

Нужно аккуратно перестроить layout и CI-shell репозитория так, чтобы:
- `templates/` были разложены по функциональным подпапкам;
- в repo появились нормальные точки входа для будущих `api/` и `controllers/`;
- hook boundary была описана и не смешивалась с declarative DB management;
- GitHub и GitLab CI были названы и структурированы ближе к DKP module pattern,
  при этом GitHub Actions были выровнены именно под паттерн
  `gpu-control-plane`.

## Scope

- разложить templates по подпапкам `module/`, `backend/`, `database/`, `auth/`;
- создать tracked каталоги `api/` и `controllers/` с repo-facing guidance;
- описать repo layout и hook policy в development docs;
- выровнять GitHub workflows под pair `build.yaml` / `deploy.yaml` по образцу
  `gpu-control-plane`;
- выровнять `.gitlab-ci.yml` по stage/job naming и developer ergonomics.

## Non-goals

- не реализовывать реальный controller/runtime API этапа 2;
- не добавлять фейковые imperative hooks для того, что уже покрывается
  `managed-postgres` declarative resources;
- не менять phase-1 runtime semantics backend без необходимости;
- не строить полный CI shell уровня `virtualization` с e2e и release automation.

## Затрагиваемые области

- `plans/active/realign-module-layout-and-ci/`
- `templates/`
- `api/`
- `controllers/`
- `docs/development/`
- `README*.md`
- `AGENTS.md`
- `DEVELOPMENT.md`
- `.github/workflows/`
- `.gitlab-ci.yml`

## Критерии приёмки

- `templates/` разложены по компонентным подпапкам и не выглядят как плоский dump;
- в repo есть зафиксированные границы для `api/` и `controllers/`;
- описано, что managed DB/users создаются declaratively, а hooks нужны только
  для platform-side effects;
- GitHub workflows и GitLab jobs названы и структурированы понятнее и ближе к
  DKP module pattern;
- `make verify` проходит.

## Риски

- можно сломать template checksum/include paths при переносе файлов;
- можно переусложнить CI, если пытаться копировать `virtualization` слишком буквально;
- можно добавить пустые placeholder-каталоги без реальной пользы.
