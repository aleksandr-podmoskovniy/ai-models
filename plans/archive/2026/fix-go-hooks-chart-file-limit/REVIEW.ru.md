# REVIEW

## Findings

Критичных замечаний по текущему diff нет.

## Что подтверждено

- найден точный источник ошибки oversized chart file: при наличии `Chart.yaml`
  в external module bundle Deckhouse `helm3lib` идёт в `loader.Load(modulePath)`,
  и Helm начинает читать `hooks/go/ai-models-module-hooks` как обычный chart
  file;
- в fallback path без `Chart.yaml` тот же `helm3lib` синтезирует внутренний chart
  и игнорирует каталоги `hooks/` и `images/`, поэтому Go hooks не попадают под
  Helm per-file limit;
- `ai-models` оставлен на family-pattern `images/hooks -> go-hooks-artifact ->
  /hooks/go`, без возврата к `batchhooks` workaround;
- `.werf/stages/bundle.yaml` больше не включает `Chart.yaml` в external bundle;
- `make verify` и `werf config render --dev --env dev` проходят.

## Residual risks

- cluster-side startup после удаления `Chart.yaml` из external bundle ещё не
  перепроверялся в живом Deckhouse install path;
- решение опирается на текущее поведение `addon-operator/pkg/helm/helm3lib.go`;
  если этот contract upstream изменится, bundle path придётся пересмотреть.
