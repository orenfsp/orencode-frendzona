package extapi

const openAPISpec = `openapi: 3.0.3
info:
  title: ФСП Оренбургской области — внешний API
  version: 1.0.0
  description: |
    API для подключения соревновательных платформ, сайтов организаций и сервисов партнёров.
    Токен выдаёт Федерация в консоли интеграций (/api/integrations/console); у токена есть набор прав (scope).
    Вебхуки подписываются заголовком X-FSP-Signature: sha256=HMAC_SHA256(secret, X-FSP-Timestamp + "." + тело).
servers:
  - url: /
security:
  - bearer: []
components:
  securitySchemes:
    bearer: { type: http, scheme: bearer }
paths:
  /api/v1/competitions:
    get:
      summary: Календарь опубликованных соревнований (scope competitions:read)
      responses: { "200": { description: OK }, "401": { description: Нет/неверный токен }, "403": { description: Нет права } }
  /api/v1/competitions/{id}/participants:
    get:
      summary: Участники с одобренными заявками (scope participants:read)
      parameters: [{ name: id, in: path, required: true, schema: { type: integer } }]
      responses: { "200": { description: OK } }
  /api/v1/competitions/{id}/protocol:
    get:
      summary: Протокол соревнования (scope protocols:read)
      parameters: [{ name: id, in: path, required: true, schema: { type: integer } }]
      responses: { "200": { description: OK } }
  /api/v1/competitions/{id}/results/import:
    post:
      summary: Импорт результатов из внешней системы, протокол формируется автоматически (scope results:write)
      parameters: [{ name: id, in: path, required: true, schema: { type: integer } }]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [items]
              properties:
                items:
                  type: array
                  items:
                    type: object
                    required: [place]
                    properties:
                      athlete_id: { type: integer }
                      fsp_id: { type: string, description: "Используется, если athlete_id не передан" }
                      place: { type: integer, minimum: 1 }
                      score: { type: integer }
      responses:
        "200": { description: Протокол сформирован }
        "422": { description: Спортсмен не найден или у него нет одобренной заявки }
`
