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

The same image also carries the repo-owned `dmcr-cleaner` helper. It is used
only for controller-driven garbage-collection flow:

- `dmcr` serves the internal registry over HTTPS;
- `dmcr-cleaner` runs as an always-on loop in the same Pod during normal
  operation;
- controller delete flow only enqueues internal GC requests after registry
  artifact removal and does not wait for physical GC to finish before removing
  the model finalizer;
- `dmcr-cleaner` coalesces queued requests, arms one maintenance/read-only
  cycle after the internal debounce window, runs registry `garbage-collect`,
  and removes processed requests.
- `dmcr-cleaner` writes repo-owned structured JSON lifecycle logs under the
  `dmcr-garbage-collection` logger; the main `dmcr` process stays on upstream
  logging behavior.

The command package stays intentionally thin. The actual garbage-collection
lifecycle implementation now lives under `images/dmcr/internal/garbagecollection`.
