# Д2. Интеграции с внешними системами

Консоль для Федерации: **`/api/integrations/console`** (ссылка есть в подвале платформы). Войдите под ролью «Федерация».

## Что реализовано
1. **API-токены с ограничением прав.** Права: `competitions:read`, `participants:read`, `protocols:read`, `results:write`. В БД хранится только SHA-256 хеш токена, сам токен показывается один раз; токен можно отозвать; фиксируется время последнего использования.
2. **Вебхуки о событиях платформы.** События: `competition.published`, `application.approved`, `application.rejected`, `protocol.published`, `rank.decided`, `category.decided`, `webhook.test`. Каждый запрос подписан (HMAC-SHA256), 3 попытки доставки (сразу, через 2 с, через 10 с), журнал доставок со статусом, кнопки «Проверить» и «Повторить».
3. **Импорт результатов из внешней системы** — `POST /api/v1/competitions/{id}/results/import`: принимаются только спортсмены с одобренной заявкой, протокол формируется автоматически, пересчитывается рейтинг, результаты попадают в историю, уходит событие `protocol.published`.
4. **Описание API:** `GET /api/v1/openapi.yaml`.

## Формат вебхука
```
POST <url>
Content-Type: application/json
X-FSP-Event: protocol.published
X-FSP-Delivery: 17
X-FSP-Timestamp: 1767000000
X-FSP-Signature: sha256=<hex>      # HMAC_SHA256(secret, timestamp + "." + body)

{"id":"evt_17","event":"protocol.published","created_at":"2026-...Z","data":{"competition_id":3,"title":"...","results":12}}
```
Получатель отвечает любым 2xx. Пример проверки подписи — `docs/examples/webhook_receiver.py`.

## Пример подключения
```bash
TOKEN=fspt_...   # выдан в консоли с правами competitions:read participants:read results:write
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/competitions
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/competitions/3/participants
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"items":[{"fsp_id":"ФСП-56-000012","place":1,"score":950},{"athlete_id":7,"place":2,"score":900}]}' \
  http://localhost:8080/api/v1/competitions/3/results/import
```
Коды ответов: 401 — нет/неверный токен, 403 — у токена нет нужного права, 422 — спортсмен не найден или нет одобренной заявки.

## Как проверить на стенде
1. Запустить `python3 docs/examples/webhook_receiver.py` (секрет — из консоли).
2. В консоли добавить подписку на URL получателя, нажать «Проверить» — в журнале «доставлено».
3. Опубликовать соревнование / одобрить заявку / внести результаты — события приходят получателю.
4. Выдать токен с `results:write` и выполнить импорт командой выше — протокол и история обновились.
