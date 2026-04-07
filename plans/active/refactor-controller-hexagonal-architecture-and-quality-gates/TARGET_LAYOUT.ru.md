# Target Layout

## 1. Architectural rule

Controller runtime must stay split by responsibility:

- `domain`
- `application`
- `ports`
- `adapters`

Reconciler packages are adapters. They may persist state, but they must not own
lifecycle policy or runtime result interpretation.

## 2. Target package layout

```text
images/controller/internal/
  application/
    publishplan/
    publishobserve/
    deletion/
  domain/
    publishstate/
  ports/
    publishop/
    modelpack/
  controllers/
    catalogstatus/
    catalogcleanup/
  adapters/
    k8s/
      ociregistry/
      ownedresource/
      sourceworker/
      uploadsession/
      workloadpod/
    sourcefetch/
    modelformat/
    modelprofile/
    modelpack/kitops/
  artifactbackend/
  publishedsnapshot/
  support/
    cleanuphandle/
    modelobject/
    resourcenames/
    testkit/
  bootstrap/
```

## 3. Ownership expectations

### `application/publishplan`

Keeps:

- execution-mode selection;
- source-worker planning;
- upload-session issuance policy.

Must not grow into:

- runtime observation;
- backend result decoding;
- public status projection.

### `application/publishobserve`

Keeps:

- publication reconcile entry/skip gate;
- runtime port orchestration through shared publication ports;
- translation from runtime worker/session handles into domain observations;
- status-mutation planning from domain observations into controller persistence
  plans;
- backend termination-result decoding into published snapshot + cleanup handle;
- delete-vs-keep runtime decision after terminal observation.

Must not grow into:

- concrete Pod/Service/Secret rendering;
- controller client reads/writes;
- public status persistence.

### `controllers/catalogstatus`

Keeps only:

- object loading;
- application-use-case calls;
- status / cleanup-handle persistence.

Must lose:

- inline reconcile entry/skip policy;
- inline runtime source-vs-upload branching;
- inline runtime result decoding;
- upload expiry policy branching;
- terminal error-message assembly beyond adapter fallback;
- source-coupled legacy naming in options/wiring.

### `adapters/k8s/sourceworker` and `adapters/k8s/uploadsession`

Keep:

- concrete K8s resource build and CRUD;
- translation from shared runtime ports into concrete Pod/Service/Secret
  execution.

Must not own:

- controller status policy;
- public status projection;
- second copies of shared request/owner/runtime option contracts.

## 4. Thin reconciler rule

A reconciler file may do only:

1. load object(s);
2. call runtime port(s) and one bounded application use case;
3. persist returned status/object mutations;
4. map infra errors to controller-runtime result.

Reconciler must not:

- build Pod/Service/Secret specs inline;
- decode/interpret worker termination payloads inline;
- implement lifecycle transition tables inline;
- invent legacy brand/source-specific naming for shared runtime wiring.
