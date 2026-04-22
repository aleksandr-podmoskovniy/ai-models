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
- controller delete flow only enqueues internal GC requests after registry
  artifact removal and does not wait for physical GC to finish before removing
  the model finalizer;
- public `dmcr.gc.schedule` can enqueue periodic stale-sweep requests even
  without a concrete delete event;
- operators can run `dmcr-cleaner gc check` for report-only inspection of stale
  repository/source-mirror prefixes and `dmcr-cleaner gc auto-cleanup` for the
  same sweep followed by registry `garbage-collect`;
- `dmcr-cleaner` coalesces queued requests, arms one maintenance/read-only
  cycle after the internal debounce window, removes stale repository/source-
  mirror prefixes, runs registry `garbage-collect`, and removes processed
  requests;
- `dmcr-cleaner` writes repo-owned structured JSON lifecycle logs under the
  `dmcr-garbage-collection` logger; the main `dmcr` process stays on upstream
  logging behavior.

The command package stays intentionally thin. The actual garbage-collection
lifecycle implementation now lives under `images/dmcr/internal/garbagecollection`.

The same image also carries `dmcr-direct-upload`, the repo-owned helper for
trusted internal publication into backing storage:

- it serves the `direct-upload v2` API under `/v2/blob-uploads`;
- it stores multipart uploads as physical objects under
  `_ai_models/direct-upload/objects/<session-id>/data`;
- session tokens are signed and time-bounded via
  `DMCR_DIRECT_UPLOAD_SESSION_TTL`;
- this helper is only the trusted internal controller-owned publication path,
  so after multipart completion it trusts the controller-provided digest and
  size instead of doing a second full storage-side reread;
- successful publication writes the repository link plus a tiny
  `.dmcr-sealed` sidecar near the canonical digest-addressed blob path; the
  heavy bytes stay in the physical upload object and are resolved by the
  repo-owned `sealeds3` storage driver;
- failed finalization cleans up the physical upload object and the sidecar, so
  the registry does not keep half-published blobs.
