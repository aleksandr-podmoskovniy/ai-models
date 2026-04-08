# Diff Details

Date : 2026-04-08 00:00:06

Directory /Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller

Total : 56 files,  829 codes, 210 comments, -120 blanks, all 919 lines

[Summary](results.md) / [Details](details.md) / [Diff Summary](diff.md) / Diff Details

## Files
| filename | language | code | comment | blank | total |
| :--- | :--- | ---: | ---: | ---: | ---: |
| [images/controller/README.md](/images/controller/README.md) | Markdown | 22 | 0 | 0 | 22 |
| [images/controller/STRUCTURE.ru.md](/images/controller/STRUCTURE.ru.md) | Markdown | -427 | 0 | -263 | -690 |
| [images/controller/TEST\_EVIDENCE.ru.md](/images/controller/TEST_EVIDENCE.ru.md) | Markdown | 10 | 0 | 2 | 12 |
| [images/controller/cmd/ai-models-artifact-runtime/artifact\_cleanup.go](/images/controller/cmd/ai-models-artifact-runtime/artifact_cleanup.go) | Go | 27 | 15 | 10 | 52 |
| [images/controller/cmd/ai-models-artifact-runtime/dispatch.go](/images/controller/cmd/ai-models-artifact-runtime/dispatch.go) | Go | 26 | 15 | 7 | 48 |
| [images/controller/cmd/ai-models-artifact-runtime/main.go](/images/controller/cmd/ai-models-artifact-runtime/main.go) | Go | 5 | 15 | 4 | 24 |
| [images/controller/cmd/ai-models-artifact-runtime/publish\_worker.go](/images/controller/cmd/ai-models-artifact-runtime/publish_worker.go) | Go | 83 | 15 | 12 | 110 |
| [images/controller/cmd/ai-models-artifact-runtime/upload\_session.go](/images/controller/cmd/ai-models-artifact-runtime/upload_session.go) | Go | 53 | 15 | 11 | 79 |
| [images/controller/cmd/ai-models-controller/artifact\_cleanup.go](/images/controller/cmd/ai-models-controller/artifact_cleanup.go) | Go | -26 | -15 | -10 | -51 |
| [images/controller/cmd/ai-models-controller/common.go](/images/controller/cmd/ai-models-controller/common.go) | Go | -118 | -15 | -23 | -156 |
| [images/controller/cmd/ai-models-controller/dispatch.go](/images/controller/cmd/ai-models-controller/dispatch.go) | Go | -28 | -15 | -6 | -49 |
| [images/controller/cmd/ai-models-controller/publish\_worker.go](/images/controller/cmd/ai-models-controller/publish_worker.go) | Go | -82 | -15 | -12 | -109 |
| [images/controller/cmd/ai-models-controller/run.go](/images/controller/cmd/ai-models-controller/run.go) | Go | 1 | 0 | 0 | 1 |
| [images/controller/cmd/ai-models-controller/upload\_session.go](/images/controller/cmd/ai-models-controller/upload_session.go) | Go | -52 | -15 | -11 | -78 |
| [images/controller/install-kitops.sh](/images/controller/install-kitops.sh) | Shell Script | 73 | 16 | 13 | 102 |
| [images/controller/internal/adapters/k8s/sourceworker/auth\_secret.go](/images/controller/internal/adapters/k8s/sourceworker/auth_secret.go) | Go | -16 | 0 | -2 | -18 |
| [images/controller/internal/adapters/k8s/sourceworker/build.go](/images/controller/internal/adapters/k8s/sourceworker/build.go) | Go | -2 | 0 | 0 | -2 |
| [images/controller/internal/adapters/k8s/sourceworker/service.go](/images/controller/internal/adapters/k8s/sourceworker/service.go) | Go | -26 | 0 | -4 | -30 |
| [images/controller/internal/adapters/k8s/sourceworker/validation.go](/images/controller/internal/adapters/k8s/sourceworker/validation.go) | Go | -26 | 0 | -1 | -27 |
| [images/controller/internal/adapters/k8s/uploadsession/options.go](/images/controller/internal/adapters/k8s/uploadsession/options.go) | Go | -22 | 0 | 0 | -22 |
| [images/controller/internal/adapters/k8s/uploadsession/pod.go](/images/controller/internal/adapters/k8s/uploadsession/pod.go) | Go | 22 | 0 | 2 | 24 |
| [images/controller/internal/adapters/k8s/uploadsession/replay\_test.go](/images/controller/internal/adapters/k8s/uploadsession/replay_test.go) | Go | 3 | 0 | 0 | 3 |
| [images/controller/internal/adapters/k8s/uploadsession/request.go](/images/controller/internal/adapters/k8s/uploadsession/request.go) | Go | -25 | -15 | -5 | -45 |
| [images/controller/internal/adapters/k8s/uploadsession/resources.go](/images/controller/internal/adapters/k8s/uploadsession/resources.go) | Go | -3 | 0 | 0 | -3 |
| [images/controller/internal/adapters/k8s/uploadsession/service.go](/images/controller/internal/adapters/k8s/uploadsession/service.go) | Go | -44 | 0 | -4 | -48 |
| [images/controller/internal/adapters/k8s/uploadsession/service\_roundtrip\_test.go](/images/controller/internal/adapters/k8s/uploadsession/service_roundtrip_test.go) | Go | 3 | 0 | 0 | 3 |
| [images/controller/internal/adapters/k8s/uploadsession/service\_test.go](/images/controller/internal/adapters/k8s/uploadsession/service_test.go) | Go | 3 | 0 | 0 | 3 |
| [images/controller/internal/adapters/k8s/uploadsession/status.go](/images/controller/internal/adapters/k8s/uploadsession/status.go) | Go | -49 | 0 | -9 | -58 |
| [images/controller/internal/adapters/k8s/uploadsession/status\_test.go](/images/controller/internal/adapters/k8s/uploadsession/status_test.go) | Go | -23 | 0 | -4 | -27 |
| [images/controller/internal/adapters/k8s/uploadsession/test\_helpers\_test.go](/images/controller/internal/adapters/k8s/uploadsession/test_helpers_test.go) | Go | 3 | 0 | 0 | 3 |
| [images/controller/internal/adapters/k8s/workloadpod/options.go](/images/controller/internal/adapters/k8s/workloadpod/options.go) | Go | 43 | 15 | 8 | 66 |
| [images/controller/internal/adapters/k8s/workloadpod/options\_test.go](/images/controller/internal/adapters/k8s/workloadpod/options_test.go) | Go | 74 | 15 | 10 | 99 |
| [images/controller/internal/adapters/sourcefetch/http.go](/images/controller/internal/adapters/sourcefetch/http.go) | Go | -16 | 0 | 0 | -16 |
| [images/controller/internal/adapters/sourcefetch/huggingface.go](/images/controller/internal/adapters/sourcefetch/huggingface.go) | Go | -30 | 0 | -2 | -32 |
| [images/controller/internal/adapters/sourcefetch/remote.go](/images/controller/internal/adapters/sourcefetch/remote.go) | Go | 128 | 15 | 20 | 163 |
| [images/controller/internal/adapters/sourcefetch/remote\_test.go](/images/controller/internal/adapters/sourcefetch/remote_test.go) | Go | 36 | 15 | 7 | 58 |
| [images/controller/internal/adapters/sourcefetch/transport.go](/images/controller/internal/adapters/sourcefetch/transport.go) | Go | 63 | 15 | 8 | 86 |
| [images/controller/internal/application/publishobserve/ensure\_runtime.go](/images/controller/internal/application/publishobserve/ensure_runtime.go) | Go | 90 | 15 | 18 | 123 |
| [images/controller/internal/application/publishobserve/ensure\_runtime\_test.go](/images/controller/internal/application/publishobserve/ensure_runtime_test.go) | Go | 205 | 15 | 29 | 249 |
| [images/controller/internal/application/publishobserve/observe\_runtime\_test.go](/images/controller/internal/application/publishobserve/observe_runtime_test.go) | Go | 343 | 15 | 13 | 371 |
| [images/controller/internal/application/publishobserve/observe\_source\_worker.go](/images/controller/internal/application/publishobserve/observe_source_worker.go) | Go | 79 | 15 | 9 | 103 |
| [images/controller/internal/application/publishobserve/observe\_upload\_session.go](/images/controller/internal/application/publishobserve/observe_upload_session.go) | Go | 87 | 15 | 10 | 112 |
| [images/controller/internal/application/publishobserve/reconcile\_gate.go](/images/controller/internal/application/publishobserve/reconcile_gate.go) | Go | 63 | 15 | 12 | 90 |
| [images/controller/internal/application/publishobserve/reconcile\_gate\_test.go](/images/controller/internal/application/publishobserve/reconcile_gate_test.go) | Go | 123 | 15 | 8 | 146 |
| [images/controller/internal/application/publishobserve/runtime\_result.go](/images/controller/internal/application/publishobserve/runtime_result.go) | Go | 29 | 15 | 5 | 49 |
| [images/controller/internal/application/publishobserve/status\_mutation.go](/images/controller/internal/application/publishobserve/status_mutation.go) | Go | 56 | 15 | 9 | 80 |
| [images/controller/internal/application/publishobserve/status\_mutation\_test.go](/images/controller/internal/application/publishobserve/status_mutation_test.go) | Go | 166 | 15 | 11 | 192 |
| [images/controller/internal/cmdsupport/common.go](/images/controller/internal/cmdsupport/common.go) | Go | 122 | 15 | 24 | 161 |
| [images/controller/internal/controllers/catalogstatus/io.go](/images/controller/internal/controllers/catalogstatus/io.go) | Go | -23 | 0 | -3 | -26 |
| [images/controller/internal/controllers/catalogstatus/options.go](/images/controller/internal/controllers/catalogstatus/options.go) | Go | -24 | 0 | 0 | -24 |
| [images/controller/internal/controllers/catalogstatus/policy.go](/images/controller/internal/controllers/catalogstatus/policy.go) | Go | -38 | -15 | -7 | -60 |
| [images/controller/internal/controllers/catalogstatus/reconciler.go](/images/controller/internal/controllers/catalogstatus/reconciler.go) | Go | -70 | 0 | -5 | -75 |
| [images/controller/internal/dataplane/publishworker/run.go](/images/controller/internal/dataplane/publishworker/run.go) | Go | -14 | 0 | 2 | -12 |
| [images/controller/internal/dataplane/publishworker/support.go](/images/controller/internal/dataplane/publishworker/support.go) | Go | -6 | 0 | -1 | -7 |
| [images/controller/tools/install-kitops.sh](/images/controller/tools/install-kitops.sh) | Shell Script | -60 | -16 | -12 | -88 |
| [images/controller/werf.inc.yaml](/images/controller/werf.inc.yaml) | YAML | 38 | 0 | 0 | 38 |

[Summary](results.md) / [Details](details.md) / [Diff Summary](diff.md) / Diff Details