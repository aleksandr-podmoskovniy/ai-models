# Target Layout

## 1. Architectural rule

Controller runtime must be split by responsibility:

- `domain`
- `application`
- `ports`
- `adapters`

Reconciler packages are adapters. They must not own business workflow assembly.

## 2. Target package layout

```text
images/controller/internal/
  application/
    publication/
      start_publication.go
      plan_source_worker.go
      issue_upload_session.go
    deletion/
      ensure_cleanup_finalizer.go
      finalize_delete.go
  domain/
    publication/
  ports/
    publication/
      operation_contract.go
      ports.go
  controllers/
    catalogstatus/
    publicationops/
    catalogcleanup/
  adapters/
    k8s/
      cleanupjob/
      ociregistry/
      sourceworker/
      uploadsession/
  support/
    cleanuphandle/
    modelobject/
    resourcenames/
    testkit/
  artifactbackend/
  publication/
  app/
```

## 3. What moves out of current packages

### `controllers/catalogstatus`

Should keep only:

- reconcile read/write shell;
- object loading;
- call into application use cases;
- final status patch.

Must lose:

- inline state machine branching;
- direct ConfigMap operation orchestration;
- cleanup handle business decisions;
- lifecycle status assembly.

### `controllers/publicationops`

Should keep only:

- reconcile shell for operation object;
- adapter calls into application use cases.

Must lose:

- worker/session lifecycle business branching;
- artifact result interpretation as domain decision;
- inline error-to-status policy mapping;
- concrete runtime port implementations for source/upload adapters.

### `adapters/k8s/uploadsession`

Must split into:

- domain/session policy
- application/session issuance and observation
- k8s object builders for `Pod` / `Service` / `Secret`

It should not remain a 600+ LOC mixed service.

### `adapters/k8s/sourceworker` / `adapters/k8s/uploadsession`

These packages should own:

- concrete K8s resource build and CRUD;
- translation from shared publication runtime ports into concrete adapter
  execution and handle shapes, without mirroring the shared contract through
  adapter-local `Request` / `OwnerRef` wrappers.

They should not force `controllers/publicationops` to own adapter-specific
runtime wrappers.

## 4. Thin reconciler rule

A reconciler file may do only:

1. load object(s);
2. call one application use case;
3. persist resulting status/object changes;
4. map infra errors to controller-runtime result.

Reconciler must not:

- build Pod/Service/Secret specs inline;
- decode/interpret worker result payloads inline;
- implement lifecycle transition tables inline;
- implement cleanup policy inline.
