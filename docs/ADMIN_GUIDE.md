---
title: "Admin Guide"
menuTitle: "Administration"
weight: 40
description: "Enable ai-models, configure artifact storage, managed SDS-backed node-cache, RBAC, monitoring, and operations."
---

This guide is for DKP cluster administrators who enable `ai-models`, connect
artifact storage, configure node-local model cache, and verify operational
readiness.

## Core Idea

- The user-facing `ModuleConfig` must stay short.
- `DMCR`, internal object names, source-fetch policy, and GC cadence are
  module-owned.
- Administrators configure only stable decisions: artifact storage, capacity
  limit, and, when node-local cache is needed, cache node/block-device selector
  and size.

## Requirements

- Deckhouse Kubernetes Platform `>= 1.74`.
- Kubernetes `>= 1.30`.
- S3-compatible object storage with a bucket for ai-models.
- Secret in `d8-system` with `accessKey` and `secretKey`.
- For managed node-cache: enabled `sds-node-configurator` and
  `sds-local-volume` modules, plus consumable `BlockDevice` objects on selected
  nodes.

## Minimal Enablement

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ai-models
spec:
  enabled: true
  version: 1
  settings:
    logLevel: Info
    artifacts:
      bucket: ai-models
      endpoint: https://s3.example.com
      region: us-east-1
      credentialsSecretName: ai-models-artifacts
      usePathStyle: true
```

The credentials Secret must live in `d8-system`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-artifacts
  namespace: d8-system
type: Opaque
stringData:
  accessKey: "<access-key>"
  secretKey: "<secret-key>"
```

For custom S3 trust, put `ca.crt` into a separate Secret and set
`artifacts.caSecretName`, or put `ca.crt` into the same credentials Secret.

## Capacity Limit

`artifacts.capacityLimit` sets the total module-owned artifact storage budget.
When enabled, upload sessions must know the payload size before writing data.
The normal `curl -T` path provides that through `Content-Length`; multipart
clients provide it during `/probe`. The module reserves space before upload and
returns a clear failure when capacity is insufficient.

```yaml
spec:
  settings:
    artifacts:
      capacityLimit: 500Gi
```

This does not replace backend object-storage quotas. It is the ai-models
admission and accounting layer that prevents obviously impossible publications.

## Managed Node Cache

Managed node-cache lets workloads receive a read-only model mount from a
per-node shared digest store instead of downloading the model into their own
PVC each time.

Basic setup:

1. Enable `sds-node-configurator` and `sds-local-volume`.
2. Label nodes that may host model cache:

   ```bash
   kubectl label node <node> ai.deckhouse.io/model-cache=true
   ```

3. Label `BlockDevice` objects that may be consumed for cache:

   ```bash
   kubectl label blockdevice <bd-name> ai.deckhouse.io/model-cache=true
   ```

4. Enable node-cache:

   ```yaml
   spec:
     settings:
       nodeCache:
         enabled: true
         size: 200Gi
   ```

By default ai-models selects both `Node` and `BlockDevice` objects by the same
label: `ai.deckhouse.io/model-cache=true`. Override `nodeSelector` and
`blockDeviceSelector` only when the cluster already has a stricter labeling
scheme.

### SDS And Cache Substrate Checks

```bash
kubectl get blockdevices.storage.deckhouse.io -o wide
kubectl get lvmvolumegroupsets.storage.deckhouse.io
kubectl get lvmvolumegroups.storage.deckhouse.io
kubectl get localstorageclasses.storage.deckhouse.io
kubectl -n d8-ai-models get pods,pvc -l app=ai-models-node-cache-runtime -o wide
```

If `sds-node-configurator` does not find a disk, verify that the disk is really
free, has no stale LVM/FS signatures, and appears as `consumable=true`.

## RBAC

The module follows the Deckhouse access-level model:

- `User` can read `Model` / `ClusterModel` and statuses.
- `Editor` can manage namespaced `Model`.
- `ClusterEditor` can manage `ClusterModel`.
- `rbacv2/use` grants namespaced use of `Model`.
- `rbacv2/manage` is the cluster-persona path for `Model`, `ClusterModel`, and
  `ModuleConfig`.

Human-facing roles must not grant `Secret`, `pods/exec`, `pods/attach`,
`pods/portforward`, `status`, `finalizers`, or internal runtime objects.

## Monitoring

Check monitoring resources:

```bash
kubectl -n d8-ai-models get podmonitor,prometheusrule
```

Main metric groups:

- catalog state: phase/ready/conditions/artifact size for `Model` and
  `ClusterModel`;
- storage usage: backend limit/used/reserved;
- runtime health: node-cache runtime Pods/PVC and workload delivery
  mode/reason;
- DMCR GC lifecycle: queued/armed/done request phases.

## Operations

Check components:

```bash
kubectl -n d8-ai-models get pods -o wide
kubectl get models.ai.deckhouse.io -A
kubectl get clustermodels.ai.deckhouse.io
```

Inspect a model:

```bash
kubectl -n <ns> describe model <name>
kubectl get clustermodel <name> -o yaml
```

Publication evidence is in:

- `status.phase`;
- `status.conditions`;
- `status.artifact.uri`;
- `status.resolved.supportedEndpointTypes`;
- `status.resolved.supportedFeatures`.

## Not User-Configurable

- `DMCR` storage prefixes, auth, TLS, and GC schedule;
- source fetch mode as a public toggle;
- names of `LocalStorageClass`, `LVMVolumeGroupSet`, VG, and thin pool;
- publication worker resources, unless a future slice exposes them explicitly.
