## Review gate

### Findings
- Критичных findings по scope нет.

### Scope check
- Bundle соответствует фактическому diff:
  - `ai-models` synced tool/build pins and base image source-of-truth check;
  - `virtualization` synced `deckhouse_lib_helm`, hooks `module-sdk`, live
    `golangci-lint` pins and stale version wording;
  - lock/module artifacts regenerated.

### Validations
- `bash build/base-images/sync-from-deckhouse.sh /Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse`
- `INSTALL_DIR=$(mktemp -d) DMT_VERSION=0.1.69 ./tools/install-dmt.sh && dmt --version`
- `cd /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization && helm dependency update`
  через isolated Helm repo config
- `cd /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/hooks && go mod tidy`
- `cd /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/hooks && go test ./...`
- grep sweep по relevant explicit pins / stale constructors / stale logger API

### Residual risks
- Полный repo-wide lint/test для `virtualization` и `ai-models` не прогонялся:
  подтверждён только узкий hooks/module/tooling slice.
- В `virtualization` уже был unrelated dirty file
  `templates/virtualization-dra/nodegroupconfiguration-usbip.yaml`; он не
  относится к этому slice и не трогался.
