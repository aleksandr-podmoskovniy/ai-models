# Security Model

## 1. Что именно мы можем защищать

Для модели как артефакта мы можем проверять:

- integrity;
- provenance;
- signing policy;
- allowed source and package shape;
- registry access and delivery policy;
- runtime unpack policy;
- cleanup discipline.

Мы не можем честно обещать, что этими мерами мы «доказываем безопасность
модели» как ML-системы. Это не заменяет:

- model evaluation;
- jailbreak/red-team checks;
- prompt safety;
- data governance;
- application-level policy.

## 2. Что даёт сам KitOps

### Built-in

- OCI content-addressed packaging;
- digest-based integrity on `pull/unpack`;
- compatibility with Cosign signatures;
- selective unpack;
- standard OCI registry transport.

### Не даёт из коробки

- централизованный policy engine;
- admission control;
- malware scanning;
- CVE/SBOM policy;
- ML-specific safety policy;
- runtime cache lifecycle.

## 3. Mandatory checks before publication

### Source checks

- allowlist/denylist для source types;
- max source size;
- max expanded archive size;
- safe extraction:
  - no path traversal;
  - no symlink/hardlink/special files;
  - allowed file types only;
- auth secret policy;
- optional network allowlist for HTTP/HF.

### Packaging checks

- generated `Kitfile` only from normalized workspace;
- package type/layout policy;
- explicit file classification for what goes into model artifact;
- reject unexpected giant side payloads.

### Publication checks

- push only to module-owned OCI prefix;
- record immutable digest in `status.artifact`;
- store internal cleanup handle separately from public status;
- sign artifact after push;
- optionally attach attestations after publication.

## 4. Mandatory checks before Ready

Before `phase=Ready`, object must have:

- immutable `status.artifact.uri` or equivalent immutable digest reference;
- `status.artifact.digest`;
- resolved technical profile;
- successful publication conditions;
- successful validation conditions.

Optional-but-target checks:

- signature created and registry-visible;
- attestation(s) attached;
- security scan result stored as internal metadata.

Publication should also persist internal evidence for later audit:

- source manifest digest;
- publication timestamp and tool version;
- signature/attestation status;
- policy decision result.

## 5. Mandatory checks before runtime start

### Required

- runtime consumes immutable digest ref, not mutable tag;
- runtime init container verifies integrity during unpack;
- if signing policy is enabled, runtime verifies signature before unpack;
- unpack only into controlled shared path;
- main runtime container gets no registry credentials.

### Strongly recommended

- use pinned upstream `kitops-init` image digest, not `latest`;
- mount Cosign key / keyless verification config through managed secrets;
- use PVC for larger models instead of `emptyDir`;
- use engine-specific unpack filters to reduce exposed surface;
- run init container with least privileges and no writable rootfs where possible.

## 6. Delete safety

Before artifact deletion:

- check for active runtime references / leases;
- check for active local materializations;
- delete published artifact only through controller-owned cleanup path;
- emit explicit deleting/cleanup conditions.

## 7. Implementation order

### V0

- current publication path through `KitOps`/OCI;
- immutable digest in public status;
- upstream `kit init` adapter with pinned version;
- integrity verification on unpack;
- basic delete cleanup.

### V1

- Cosign signing on publish;
- Cosign verification on runtime pull;
- explicit runtime materialization object/lease;
- delete guards against active consumers.

### V2

- module-owned hardened materializer image;
- distroless;
- richer attestations/SBOM/scan policy;
- cache plane and prewarming.

## 8. References

- KitOps init-container docs:
  https://kitops.org/docs/integrations/k8s-init-container/
- KitOps deployment overview:
  https://kitops.org/docs/deploy/
- KitOps security overview:
  https://kitops.org/docs/security/
- KitOps why/overview:
  https://kitops.org/docs/why-kitops/
  https://kitops.org/docs/modelkit/intro/
