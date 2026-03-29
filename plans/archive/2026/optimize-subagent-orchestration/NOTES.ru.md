# Notes

## Ключевые findings из read-only review

- в старой policy не было negative path для простых задач, поэтому delegation
  выглядел необязательным и легко пропускался;
- mapping между skills и subagents был перечислен, но не был оформлен как
  короткая operational matrix;
- были обязательные триггеры для planning, но не было таких же триггеров для
  delegation;
- cadence вызова subagents не был зафиксирован: до реализации, по slice или
  только в конце;
- `task_framer` и `module_implementer` были описаны как роли, но без ясного
  правила, когда основному агенту работать самому, а когда делегировать;
- final review был размазан между `final review`, `review-gate` и `reviewer`
  без одного канонического контракта.

## Принятое направление

- ввести три режима orchestration: `solo`, `light`, `full`;
- явно зафиксировать случаи, когда delegation обязателен и когда он не нужен;
- оформить skills -> agents -> cadence как краткую matrix;
- считать `review-gate` каноническим финальным check для substantial task, а
  `reviewer` обязательным дополнением для delegation/multi-area work.
