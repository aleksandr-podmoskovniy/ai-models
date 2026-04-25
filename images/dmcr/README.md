# DMCR

`dmcr` is the ai-models module-owned registry binary for published model
artifacts.

Current implementation keeps the proven registry runtime shape from
`virtualization`, but the executable entrypoint and packaging boundary now live
in this repository:

- binary name: `dmcr`
- source module: `images/dmcr`
- upstream registry engine: `github.com/distribution/distribution/v3`

This keeps the module on an internal, explicit binary seam without dragging the
full upstream dependency tree into this repository. The build follows the usual
Go module and `GOPROXY` flow instead of a checked-in `vendor/` tree.

The same image also carries the repo-owned `dmcr-cleaner` helper. It owns the
productized DMCR cleanup surface:

- `dmcr` serves the internal registry over HTTPS;
- `dmcr-cleaner` runs as an always-on loop in the same Pod during normal
  operation;
- controller delete flow queues internal GC requests after registry artifact
  removal; the always-on cleaner coalesces them before a physical cleanup
  cycle;
- delete-triggered GC requests can also snapshot the deleted owner's current
  unfinished direct-upload session token, so the maintenance cycle may reclaim
  that exact orphan session prefix instead of waiting for the generic
  session-age window;
- model finalizer still does not wait for physical GC to finish before
  disappearing; physical bytes are reclaimed by a later coalesced cleanup
  cycle;
- public `dmcr.gc.schedule` can enqueue periodic stale-sweep requests even
  without a concrete delete event;
- at startup, the scheduled loop performs a report-only stale check, retries
  transient check failures, and queues the normal scheduled request only when
  stale cleanup candidates already exist and no other GC request is active or
  queued;
- operators can run `dmcr-cleaner gc check` for report-only inspection of stale
  repository/source-mirror prefixes plus orphan direct-upload object prefixes
  and open direct-upload multipart uploads, and `dmcr-cleaner gc auto-cleanup`
  for the same sweep followed by registry `garbage-collect`;
- `dmcr-cleaner` coalesces queued requests, activates a cluster-visible
  zero-rollout maintenance gate after the internal debounce window, waits for
  pod-local runtime ack quorum, removes stale
  repository/source-mirror prefixes plus orphan unsealed direct-upload
  object prefixes older than the bounded session window, separately aborts
  stale open direct-upload multipart uploads, and for delete-triggered
  requests may additionally reclaim the exact unfinished direct-upload
  multipart upload snapshotted by controller delete flow as
  `{objectKey, uploadID}`; then it runs registry `garbage-collect` and removes
  processed requests;
- when no live `Model` or `ClusterModel` objects remain in the cluster,
  unprotected direct-upload residue is treated as immediately reclaimable
  instead of waiting for the generic session-age window;
- after registry `garbage-collect`, `dmcr-cleaner` reruns direct-upload orphan
  cleanup for prefixes that were protected before GC and may have become
  orphaned only after canonical blob metadata was removed;
- `dmcr-cleaner gc run` uses internal Kubernetes `Lease` objects so only one
  replica owns scheduled enqueue and active cleanup, while all replicas mirror
  maintenance state and publish pod-scoped runtime acks before destructive GC;
- `dmcr-cleaner` writes repo-owned structured JSON lifecycle logs under the
  `dmcr-garbage-collection` logger; the main `dmcr` process stays on upstream
  logging behavior.

The command package stays intentionally thin. The actual garbage-collection
lifecycle implementation now lives under `images/dmcr/internal/garbagecollection`.

The same image also carries `dmcr-direct-upload`, the repo-owned helper for
policy-controlled internal publication into backing storage:

- it serves the `direct-upload v2` API under `/v2/blob-uploads`;
- it stores multipart uploads as physical objects under
  `_ai_models/direct-upload/objects/<session-id>/data`;
- session tokens are signed and time-bounded via
  `DMCR_DIRECT_UPLOAD_SESSION_TTL`;
- after multipart completion it first asks backing storage for trusted
  `full-object sha256`;
- current default internal policy
  `DMCR_DIRECT_UPLOAD_VERIFICATION_POLICY=trusted-backend-or-client-asserted`
  accepts the controller-declared digest/size without reread when storage does
  not expose that trusted checksum;
- internal strict policy `trusted-backend-or-reread` remains available for a
  later zero-trust pass and still rereads the assembled object;
- if the client did not declare a digest, `dmcr-direct-upload` still has to
  reread the object to obtain the canonical digest-addressed blob key;
- successful publication writes the repository link plus a tiny
  `.dmcr-sealed` sidecar near the canonical digest-addressed blob path; the
  heavy bytes stay in the physical upload object and are resolved by the
  repo-owned `sealeds3` storage driver;
- trusted backend digest/size mismatches and backend-known size mismatches
  clean up the physical upload object; transient reread failures keep it so a
  repeated `complete` can reuse the already assembled bytes instead of forcing
  a full re-upload.
