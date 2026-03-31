# DECISIONS: как интерпретировать текущий MLflow surface в ai-models

## 1. SSO считать рабочим, но в точном смысле

Считать supported сейчас только:
- ingress-level SSO через Deckhouse `DexAuthenticator`;
- cluster-local Deckhouse auth и/или future cluster-wide `DexProvider`.

Не считать supported сейчас:
- app-native OIDC parity как в `n8n`;
- собственный OIDC client внутри backend.

## 2. Не обещать весь upstream UI как module contract

Считать phase-1 supported:
- experiments / runs
- registered models / logged models
- artifact storage через S3-compatible backend
- базовый UI доступ за SSO
- базовый monitoring/logging

Считать visible-but-not-platformized:
- gateway
- prompts / prompt optimization
- traces
- datasets
- scorers
- другие genai-related surfaces upstream `MLflow 3.10`

## 3. Hugging Face path считать SDK-driven

Для Hugging Face весов и model registry в phase-1 считать нормальным путь:
- logging model artifacts через MLflow SDK;
- затем регистрация модели в registry.

Не обещать:
- специальный DKP-native UI wizard для загрузки HF моделей;
- отдельный module API для этого до phase-2.

## 4. Следующий improvement делать не в breadth, а в ownership

Если расширять surface модуля дальше, делать это не по принципу
"в UI видно, значит поддерживаем", а по принципу ownership:
- выбрать capability;
- определить security/storage/auth/observability contract;
- только потом включать её в supported surface.

Первый кандидат на future hardening:
- либо явное ограничение/скрытие unsupported upstream surfaces;
- либо platformization gateway capability с отдельным secret/crypto contract.
