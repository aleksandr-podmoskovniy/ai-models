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
- `dmcr-cleaner` waits idle in the same Pod during normal operation;
- when the controller requests physical blob reclamation, hooks switch DMCR
  into maintenance/read-only mode and the cleaner runs registry
  `garbage-collect`, then marks the request as completed.

The command package stays intentionally thin. The actual garbage-collection
lifecycle implementation now lives under `images/dmcr/internal/garbagecollection`.
