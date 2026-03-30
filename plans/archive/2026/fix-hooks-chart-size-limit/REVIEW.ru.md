# Review

Критичных замечаний по текущему diff нет.

Проверено:
- изменение укладывается в phase 1 external module shell;
- oversized go-hook artifact удалён из bundle path;
- docs, values и validate согласованы с новым runtime contract;
- `make lint`, `make test` и `make helm-template` проходят.

Остаточные риски:
- `global.modules.https.mode=CustomCertificate` теперь явно не поддерживается;
  если этот сценарий понадобится, его нужно возвращать отдельной задачей с
  другим packaging contract;
- локальный `make verify` по-прежнему может зависать на `kubeconform`, это
  выглядит как старое поведение validate loop, а не как регрессия текущего
  изменения.
