# Deckhouse base images

`build/base-images/deckhouse_images.yml` is the full Deckhouse base image map used
as the source of truth for `werf`.

The repository does not maintain a hand-trimmed list here. Instead,
`.werf/stages/base-images.yaml` scans the current build manifests and instantiates
only the base images that are actually referenced by this module.

Update this file from the local Deckhouse checkout when the baseline changes:

```bash
bash ./build/base-images/sync-from-deckhouse.sh /path/to/deckhouse
```
