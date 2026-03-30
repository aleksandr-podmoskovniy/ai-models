# Findings

## Reference patterns

### virtualization
- Object storage metadata (`bucket`, `regionEndpoint`, `region`) остаётся
  user-facing в `config-values`.
- S3 credentials тоже допускаются user-facing и потом рендерятся во внутренний
  Secret модуля.
- Это означает, что bucket и endpoint в DKP module contract не считаются
  секретами сами по себе.

### n8n-d8
- Для чувствительных runtime tokens используется mixed contract:
  inline value для bootstrap плюс `existingSecret` для production.
- Secret reference идёт без попытки читать секрет из чужого namespace через
  Helm; модуль ожидает либо inline value, либо уже доступный Secret.

### deckhouse core modules
- Для global HTTPS `CustomCertificate` данные сертификата сначала попадают во
  `internal.customCertificateData` через hook, а уже потом используются
  templates.
- Это internal-only wiring, а не user-facing contract.

## Conclusion for ai-models
- `bucket`, `pathPrefix`, `endpoint`, `region`, `usePathStyle`, `insecure`
  должны оставаться user-facing как non-secret storage metadata.
- `accessKey` / `secretKey` нельзя делать единственным production path.
- Для phase 1 без hooks/controller нужен mixed contract:
  - `credentialsSecretName` с фиксированными ключами `accessKey` / `secretKey`
    для production;
  - inline `accessKey` / `secretKey` как bootstrap fallback.
- Cross-namespace secret consumption без hooks/controller в текущем module shell
  недоступен; если это понадобится, нужен отдельный platform-side effect.
