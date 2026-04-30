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

## Is sds-node-configurator required?

SDS is not required for `Model` / `ClusterModel` publication. Workload
delivery through the target SharedDirect CSI contract requires managed node
cache, so it needs `sds-node-configurator` and `sds-local-volume` or another
explicitly supported substrate for node-local cache.

## Which model delivery modes are supported?

The current working mode is `SharedDirect`:

- `nodeCache.enabled=true`;
- the selected node has local SDS storage;
- node-cache runtime materializes required digests into node-local cache;
- the workload receives a read-only CSI mount at
  `/data/modelcache/models/<model-name>`.

The target universal mode without local disks is `SharedPVC`:

- `nodeCache.enabled=false`;
- an RWX `StorageClass` is configured, for example CephFS, NFS, or another
  shared filesystem;
- the module creates a controller-owned RWX PVC for the concrete
  workload/service and requested model set;
- all Pods of that workload read models from one network shared volume.

`SharedPVC` is a separate controller-owned mode with its own ownership/auth/GC
path, not an automatic download through a Secret in the workload namespace. If
neither `SharedDirect` nor a safe `SharedPVC` path is ready, the module fails
closed: the workload receives a clear blocking reason instead of an empty
directory or hidden namespace-local download.

The current safe SharedPVC foundation already creates the controller-owned RWX
PVC and keeps the workload behind a scheduling gate until claim/materialization
readiness. Starting workloads through SharedPVC requires the digest-scoped
materializer grant path; the shared DMCR read Secret is not copied into the
workload namespace and will not be used there.

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
