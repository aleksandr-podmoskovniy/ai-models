# Plan: ingress TLS parity

## 1. Phase

Phase 1/2 operations hardening. Изменение ограничено публичным upload-gateway Ingress TLS wiring.

## 2. Orchestration

Mode: `solo`.

Reason: user did not explicitly request subagents in this turn; reference pattern is local and change is template-only. Delegation is therefore skipped despite ingress/TLS relevance.

## 3. Active bundle disposition

- `live-e2e-ha-validation` — kept active but paused. It remains the canonical
  e2e runbook after this TLS fix.
- `ingress-tls-parity` — archived after completion; no follow-up slice remains
  inside this focused fix.

## 4. Slices

### Slice 1. Reference check

Status: done.

References:

- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/templates/certificate.yaml`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/templates/custom-certificate.yaml`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse/helm_lib/charts/deckhouse_lib_helm/templates/_module_https.tpl`

### Slice 2. Template fix

Status: done.

Files:

- `templates/upload-gateway/certificate.yaml`
- `templates/upload-gateway/custom-certificate.yaml`
- `templates/upload-gateway/ingress.yaml` if needed
- `tools/helm-tests/validate-renders.py`

Result:

- CertManager mode now renders a dedicated `Certificate/ai-models-upload-gateway`
  with `certificateOwnerRef: false`, `secretName: ingress-tls`, the upload
  public host in `dnsNames` and Deckhouse ClusterIssuer wiring.
- CustomCertificate mode now renders the copied TLS Secret in a separate
  template, matching Deckhouse/virtualization layout instead of hiding it in
  the Ingress template.
- Render validation now fails if upload-gateway Ingress uses a TLS Secret
  without the matching CertManager Certificate or CustomCertificate Secret.

### Slice 3. Verification

Status: done.

Checks:

- `make helm-template` — passed.
- `make kubeconform` — passed.
- `git diff --check` — passed.

## 5. Rollback

Remove Certificate template and render validation additions. Ingress remains as before.
