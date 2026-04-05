# Notes

## Controller drift against agreed patterns

- Текущий `images/controller/internal/modelpublish/reconciler.go` совмещает:
  - lifecycle owner `Model` / `ClusterModel`;
  - orchestration публикации;
  - создание и ожидание worker Job;
  - чтение pod business result;
  - запись cleanup handle.
- Live publish path жёстко связан с `mlflow` и одним worker shape.
- Publish job shell сейчас не отделён от cleanup job shell.
- Public reconciler слишком глубоко знает про pod/job internals.
- `spec.source` drift-нул от agreed source-first direction: сейчас там
  `HuggingFace | Upload | OCIArtifact`, а ожидаемая модель — `HuggingFace |
  HTTP | Upload`.

## Virtualization patterns to imitate

- Пользователь задаёт `dataSource`, а controller публикует итоговый target в
  `status`:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/api/core/v1alpha2/virtual_image.go`
- Reconciler собирается из отдельных handlers, а не из одного fat control loop:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/virtualization-artifact/pkg/controller/cvi/cvi_controller.go`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/virtualization-artifact/pkg/controller/cvi/cvi_reconciler.go`
- Upload path — controller-owned lifecycle со статусом `WaitForUserUpload`,
  upload contract в `status`, cleanup временных ресурсов после завершения:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/virtualization-artifact/pkg/controller/cvi/internal/source/upload.go`
- Auth/access распространяется controller-ом on-demand и потом чистится:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/docs/internal/dvcr_auth.md`
- Deletion owner отделён от source execution owner и живёт через finalizer:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/virtualization-artifact/pkg/controller/cvi/internal/deletion.go`
- Долгие provisioning flows разбиты на шаги (`CreateDataVolumeStep`,
  `WaitForDVStep`, `ReadyStep`), а не кодируются одной функцией reconcile:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/virtualization-artifact/pkg/controller/vd/internal/source/step/create_dv_step.go`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/virtualization-artifact/pkg/controller/vd/internal/source/step/wait_for_dv_step.go`

## Corrective target for ai-models

- `modelpublish` становится lifecycle/status owner для `Model` /
  `ClusterModel`, но не worker orchestrator.
- Появляется отдельный internal publication operation owner в module namespace.
- Worker result идёт в durable internal object, а не напрямую в public
  reconciler из pod status.
- HF остаётся первым live source path, но уже на новой operation structure.
- `Upload` и `HTTP` должны добавляться в ту же operation boundary без смены
  публичного API.
