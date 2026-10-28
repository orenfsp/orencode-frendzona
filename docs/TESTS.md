# Тесты

Запуск из `backend`: `go test ./...` (подробно: `go test -v ./...`, один пакет: `go test ./internal/readiness`). Тесты не требуют базы данных.

| Пакет | Что проверяет |
|---|---|
| `internal/rules` | возраст (в т.ч. день рождения и високосные годы), возрастные группы, баллы за места по уровню, коэффициент стабильности, шесть дисциплин |
| `internal/readiness` | чек-лист «Готовность к старту»: блокеры и предупреждения, срок приёма заявок, окна напоминаний 7/3/1 |
| `internal/exportutil` | корректность xlsx/docx, экранирование XML |
| `internal/platform` | этапы подготовки, допустимые дисциплины, разбор id |
| `internal/oidc`, `internal/integrations/extapi` | PKCE, redirect_uri (в т.ч. демо-клиент на любом адресе), scope, подпись вебхуков, проверка URL |
| `internal/emailcode` | коды входа: одноразовость, лимиты, повторная отправка после сбоя |
| `internal/httpserver`, `internal/config`, `internal/competitions`, `internal/notifications`, `internal/integrations/telegram` | роутер и CORS, конфигурация, формат ответов, Telegram |

Упал тест — в сообщении указано, какое правило нарушено и что ожидалось. Найденная тестами ошибка: `rules.AgeAt` в високосные годы давал возраст 17 в день 18-летия (исправлено).
