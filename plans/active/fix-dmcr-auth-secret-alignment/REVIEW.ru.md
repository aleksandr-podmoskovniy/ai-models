## Review

### Blocking findings

Нет blocking findings по repo-side diff.

### Residual risks

- Repo-side fix теперь покрывает не только first-install drift, но и
  upgrade-safe recovery для уже битых кластеров:
  `ai-models-dmcr-auth` теперь несёт явные
  `write.htpasswd.checksum` / `read.htpasswd.checksum`, а helper reuse'ит
  старый bcrypt только если checksum уже совпадает с текущим password.
- Это означает: после rollout нового module build старый cluster state с
  битым `write.htpasswd` / `read.htpasswd` self-heal'ится без ручной пересборки
  server auth secret.
- Live cluster в момент проверки всё ещё работал на старом module build, поэтому
  репо-фикс нельзя было доказать одним benign reconcile. `Gemma 4` smoke по
  живому кластеру всё ещё подтверждает две вещи:
  - `HF -> source mirror -> native publish` доходит до `DMCR` boundary;
  - для полного recovery нужен rollout модуля с этим diff.

### Validation record

- `python3 tools/helm-tests/validate_renders_test.py`
- `make helm-template`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `make kubeconform`
- `make verify`
- live inspection:
  - `kubectl -n d8-ai-models get secret ai-models-dmcr-auth ai-models-dmcr-auth-write ai-models-dmcr-auth-read -o json`
  - `kubectl -n d8-ai-models describe pod ai-model-publish-1d961707-e5e6-45b9-9598-499f22821665`
  - `htpasswd -vb <tempfile> ai-models <write.password>`

### Notes

- CI failure с `ModuleNotFoundError: No module named 'yaml'` относился к
  pre-fix state verify-path. Текущий repo state уже использует stdlib-only
  parser в `validate-renders.py`, а focused test теперь тоже не требует
  `PyYAML` и проверяет реальный checksum/`htpasswd` path.
