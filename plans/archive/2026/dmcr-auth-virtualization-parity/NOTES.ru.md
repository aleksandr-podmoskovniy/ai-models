# Notes

## Virtualization baseline

`virtualization` DVCR auth не генерируется в Helm template:

- hook `generate-secret-for-dvcr` snapshot-ит `Secret/dvcr-secrets`;
- если existing Secret валиден, state переносится в internal values;
- если password/salt/htpasswd отсутствуют или htpasswd не соответствует
  password, hook генерирует новое значение;
- template `templates/dvcr/secret.yaml` только рендерит
  `virtualization.internal.dvcr.{passwordRW,htpasswd,salt}`;
- dockerconfig строится из уже готового internal password.

## Drift в ai-models

DMCR auth в `ai-models` пока делает это в Helm:

- `dmcrWriteAuthPassword` и `dmcrReadAuthPassword` читают live Secrets через
  `lookup`, иначе вызывают `randAlphaNum`;
- `dmcrWriteHTPasswdEntry` и `dmcrReadHTPasswdEntry` вызывают Helm `htpasswd`;
- `dmcrHTTPSalt` вызывает `randAlphaNum`;
- offline render остаётся случайным, а auth state не является явным module
  internal state.
