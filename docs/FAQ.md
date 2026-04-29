---
title: "FAQ"
menuTitle: "FAQ"
weight: 80
description: "Common questions about ai-models, node cache, upload, Ollama, ArgoCD, and diagnostics."
---

## Why are there no DMCR settings in ModuleConfig?

`DMCR` is an internal publication backend, not a user-facing registry. The
public module contract is `Model`, `ClusterModel`, `status.artifact`, and
workload annotations. Exposing DMCR prefixes, TLS, auth, or GC cadence would
make users depend on implementation details the module must be able to change.

## Why can't users select sourceFetchMode?

Fetch transport is controller/runtime responsibility, not user responsibility.
The user defines the model source, and the module chooses the safe path:
streaming, temporary object-source, or upload staging. This keeps the
configuration smaller and avoids unsupported publication variants for the same
kind of model.

## Is sds-node-configurator required?

SDS is not required for `Model` / `ClusterModel` publication. Workload
delivery through the target SharedDirect CSI contract requires managed node
cache, so it needs `sds-node-configurator` and `sds-local-volume` or another
explicitly supported substrate for node-local cache.

## Is mixed mode supported: cache where a local disk exists, PVC elsewhere?

The target model is explicit: node-cache runtime is installed only on nodes
selected for local cache, while workload placement stays outside ai-models.
SharedDirect workloads must be scheduled by the workload, ai-inference, or
another scheduler onto nodes where local cache is ready and the model fits. Do
not label nodes without suitable local disks as
`ai.deckhouse.io/model-cache=true`. PVC/materialize fallback in workload
namespaces is no longer a supported path: if a workload lands on an unprepared
node, the CSI mount fails or waits with kubelet events.

## sds-node-configurator does not see a disk. What should I check?

Check that the disk:

- is attached to the VM/node and visible to the OS;
- has no stale LVM/FS signatures;
- is not used by kubelet, Ceph, systemd, or another storage stack;
- appears as a `BlockDevice` and is consumable;
- has the same label selected by `nodeCache.blockDeviceSelector`.

Commands:

```bash
kubectl get blockdevices.storage.deckhouse.io -o wide
kubectl describe blockdevice <bd-name>
kubectl get nodes --show-labels | grep ai.deckhouse.io/model-cache
```

## Why is the upload URL secret?

The upload URL contains a time-bounded credential, matching the direct upload
UX used by virtualization. Treat it as a secret: do not paste it into public
logs or tickets, and prefer `status.upload.inCluster` for in-cluster upload.

## What does ToolCalling mean?

`ToolCalling` means model metadata contains signs of a tool-call capable chat
template. It does not enable MCP by itself. MCP is a capability of the future
`ai-inference` runtime/host layer.

## How does Ollama publication work?

The controller reads the Ollama registry manifest/config/blob path, not the
public HTML page and not a local Ollama daemon. It accepts a single GGUF model
layer, verifies descriptor digests and the GGUF magic header, and publishes the
payload as the module-owned `ModelPack` artifact. Runtime selection remains a
future `ai-inference` decision.

## How do I avoid ArgoCD drift from workload mutation?

Keep only the source object and ai-models model annotation in Git. Do not
commit controller-written CSI volumes, artifact attributes, env vars or mounts.
For CRD operators, render a supported Kubernetes workload with the model
annotation on workload metadata; ai-models does not patch the higher-level CRD
by name.

## Why does a workload not start when a model is not ready?

The delivery controller must fail closed: if `Model` is not `Ready`, the
artifact is not published, node-cache delivery is disabled, or the requested
model set does not fit the configured per-node cache size, the workload
receives a condition/blocking reason instead of starting with an empty model
directory. If the workload is explicitly scheduled onto a node where node-cache
runtime is not ready, kubelet reports the CSI mount failure/wait; ai-models
does not inject node placement.

## Which metrics should I check first?

- `d8_ai_models_model_ready`;
- `d8_ai_models_model_status_phase`;
- `d8_ai_models_model_condition`;
- `d8_ai_models_model_artifact_size_bytes`;
- `d8_ai_models_storage_backend_limit_bytes`;
- `d8_ai_models_node_cache_runtime_pods_ready`;
- `d8_ai_models_workload_delivery_workloads_managed`;
- `d8_ai_models_workload_delivery_pods_managed`;
- `d8_ai_models_workload_delivery_pods_ready`;
- `d8_ai_models_dmcr_gc_requests`.

## What should I do when publication fails with InsufficientStorage?

Check `artifacts.capacityLimit`, current storage usage metrics, and model size.
For upload sessions, use `curl -T <file> <upload-url>` or another client that
sends `Content-Length`; without a known size the module rejects the request
before writing data because it cannot reserve storage safely.

## Can I use status.artifact.uri manually?

For diagnostics, yes. For application workloads, no. Workloads should reference
`Model` / `ClusterModel` through annotations so the controller can manage the
SharedDirect CSI mount, stable runtime environment, retries, and future delivery
topology changes without exposing registry credentials to workload namespaces.
