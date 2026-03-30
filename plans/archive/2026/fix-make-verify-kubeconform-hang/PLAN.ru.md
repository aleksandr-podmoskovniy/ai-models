# План: добить make verify и снять зависание kubeconform

## Current phase
Этап 1. Managed backend inside the module.

## Режим orchestration
`solo`.
Причина: это узкая tooling/debug задача с одним главным риском внутри verify
loop; дополнительная delegation не даст лучшего сигнала быстрее локальной
диагностики.

## Slice 1. Локализовать источник зависания
Цель: понять, висит ли `kubeconform` на schema fetch, на конкретном render file
или на собственном parse/report шаге.

Файлы/каталоги:
- `tools/kubeconform/kubeconform.sh`
- generated renders в `tools/kubeconform/renders/`

Проверки:
- debug run `kubeconform` на отдельных render files;
- локальные preflight checks для schema/network behavior.

Артефакт результата:
- подтверждённый источник hanging.

## Slice 2. Починить verify path
Цель: изменить `kubeconform` loop так, чтобы он завершался детерминированно и
сохранял полезную валидацию.

Файлы/каталоги:
- `tools/kubeconform/kubeconform.sh`
- `Makefile` при необходимости
- `DEVELOPMENT.md` при необходимости

Проверки:
- `make kubeconform`
- `make verify`

Артефакт результата:
- зелёный deterministic verify loop.

## Rollback point
После Slice 1. Причина hanging зафиксирована, но verify behavior ещё не менялся.

## Final validation
- `make lint`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
