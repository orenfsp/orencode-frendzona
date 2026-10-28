# Единая цифровая платформа ФСП Оренбургской области

Хакатон «ОренКод 2026». Платформа регионального отделения Федерации спортивного программирования: реестр спортсменов, тренеров, судей и организаций, календарь соревнований, заявки, протоколы, разряды и судейские категории, рейтинг и сборная, отчёты, личные кабинеты с несколькими ролями у одного аккаунта.

## 🌐 Работающий стенд

http://193.232.88.5:8080/

| Что | Адрес |
|---|---|
| Платформа | `<ссылка>/` |
| Состояние API | `<ссылка>/api/health` |
| Навигатор для новичка (без входа) | `<ссылка>/api/public/navigator` |
| ФСП ID: discovery / демо-клиент | `<ссылка>/.well-known/openid-configuration` · `<ссылка>/oidc/demo` |
| Консоль интеграций (роль «Федерация») | `<ссылка>/api/integrations/console` |
| Telegram-бот | [@FSP56OrenburgBot](https://t.me/FSP56OrenburgBot) |

Все данные на стенде вымышлены. Пароль всех демо-аккаунтов — `demo` (список ниже).

---

## 🚀 Запуск локально

### Вариант 1. Docker (рекомендуется, 3 команды)

Нужно: [Docker Desktop](https://www.docker.com/products/docker-desktop/) (или Docker Engine) с Compose v2, свободные порты `8080` и `8081`.

```bash
git clone <адрес-репозитория> fsp-platform
cd fsp-platform
docker compose up --build
```

Первая сборка занимает 3–5 минут. Когда в логах появится строка о запуске API, откройте:

| Что | Адрес |
|---|---|
| Веб-интерфейс | <http://localhost:8080> |
| API напрямую | <http://localhost:8081/api/health> |
| ФСП ID, демо-клиент | <http://localhost:8080/oidc/demo> |
| Консоль интеграций | <http://localhost:8080/api/integrations/console> |

При первом запуске API сам создаёт таблицы (миграции) и наполняет базу демонстрационными данными. Повторные запуски данные не дублируют. База хранится в volume `postgres_data`.

Остановить: `Ctrl+C`, затем `docker compose down`.
Полностью сбросить базу к демо-состоянию: `docker compose down -v && docker compose up --build`.

> **Необязательно.** Без настройки почты и Telegram платформа работает: код входа по email печатается в логе (`docker compose logs api`), уведомления видны в кабинете. Как подключить почту и бота — в разделе «Настройки» ниже.

### Вариант 2. Без Docker (для разработки)

Нужно: Go 1.23+, Node.js 20+, PostgreSQL 15+.

**1. База данных.** Создайте пользователя и базу (значения можно изменить, тогда поправьте `DATABASE_URL`):

```bash
psql -U postgres -c "CREATE USER orenkod WITH PASSWORD 'orenkod_local';"
psql -U postgres -c "CREATE DATABASE orenkod OWNER orenkod;"
```

**2. Бэкенд** (порт `8081`, его ожидает прокси фронтенда):

```bash
cd backend
# bash / macOS / Linux
export PORT=8081
export DATABASE_URL="postgres://orenkod:orenkod_local@localhost:5432/orenkod?sslmode=disable"
go run ./cmd/api
```

```powershell
# PowerShell (Windows)
cd backend
$env:PORT="8081"
$env:DATABASE_URL="postgres://orenkod:orenkod_local@localhost:5432/orenkod?sslmode=disable"
go run ./cmd/api
```

**3. Фронтенд** (в другом терминале):

```bash
cd frontend
npm install
npm run dev
```

Откройте адрес из вывода Vite (обычно <http://localhost:5173>). Запросы `/api`, `/oidc` и `/.well-known` проксируются на бэкенд `localhost:8081`.

### Проверка, что всё работает

1. `http://localhost:8080/api/health` (или `:8081`) возвращает JSON со статусом `ok`.
2. На странице входа войдите как `federation@fsp56.ru` / `demo`: видны дашборд, календарь и реестр с демо-данными.
3. Прогон тестов бэкенда: `cd backend && go test ./... && go vet ./...`.

### Демо-аккаунты (пароль у всех — `demo`)

| Email | Роли |
|---|---|
| `federation@fsp56.ru` | Федерация |
| `morozov@fsp56.ru` | Спортсмен + Тренер (проверка переключения ролей) |
| `sokolova@fsp56.ru` | Спортсмен |
| `krylova@fsp56.ru` | Тренер |
| `volkov@fsp56.ru` | Тренер + Судья |
| `judge@fsp56.ru` | Судья |
| `ministry@fsp56.ru` | Министерство (только чтение) |

Эти учётные данные только для демонстрации. Для публичного развёртывания смените пароли.

---

## ⚙️ Настройки (переменные окружения)

Скопируйте шаблон и заполните нужное: `cp .env.example .env`. Docker Compose подхватывает `.env` из корня проекта. **Не коммитьте `.env`.**

| Переменная | Назначение | По умолчанию |
|---|---|---|
| `PORT` | Порт API | `8080` |
| `DATABASE_URL` | Строка подключения к PostgreSQL | локальная БД из compose |
| `TELEGRAM_BOT_TOKEN` | Токен бота от @BotFather | пусто (бот выключен) |
| `TELEGRAM_DEFAULT_CHAT_ID` | Общий чат для уведомлений (необязательно) | пусто |
| `BREVO_API_KEY` | Отправка кодов входа через HTTP API Brevo (для хостингов, где закрыт SMTP) | пусто |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD` | Отправка кодов через SMTP | пусто / `587` |
| `SMTP_FROM` | Адрес отправителя (для Brevo — подтверждённый отправитель) | пусто |
| `OIDC_ISSUER` | Публичный адрес платформы для ФСП ID, например `https://домен`. Пусто — определяется по запросу | пусто |

После изменения `.env`: `docker compose up -d --force-recreate api`.

### Почта (коды входа)

- **Локально, Gmail:** `SMTP_HOST=smtp.gmail.com`, `SMTP_PORT=587`, `SMTP_USER` и `SMTP_FROM` = ваш адрес, `SMTP_PASSWORD` = [пароль приложения](https://myaccount.google.com/apppasswords) (не обычный пароль).
- **На хостинге:** многие провайдеры (в том числе бесплатный Render) блокируют исходящий SMTP, и письма не уходят. Используйте Brevo: создайте бесплатный аккаунт на brevo.com, подтвердите отправителя, получите API-ключ и задайте `BREVO_API_KEY` и `SMTP_FROM`. В логе API при старте появится строка «Почта: HTTP API Brevo настроен».
- Без настройки код входа печатается в логе: `docker compose logs api`.

### Telegram-бот

Создайте бота у [@BotFather](https://t.me/BotFather), вставьте токен в `TELEGRAM_BOT_TOKEN`, перезапустите `api`. Бот работает через long polling: домен и HTTPS не нужны. Привязка аккаунта: в кабинете раздел «Уведомления» → «Получить код» → отправить боту `/link КОД`. Команды: `/start`, `/events`, `/rating`, `/help`, `/link`, `/chatid`. Подробно: [`docs/TELEGRAM_BOT_GUIDE.md`](docs/TELEGRAM_BOT_GUIDE.md). Бот в мессенджере MAX не подключён: для него нужна лицензия организации.

---

## ☁️ Размещение на хостинге (Render)

Платформа состоит из трёх частей: PostgreSQL, API (Go, `backend/Dockerfile`) и веб-часть (`frontend/Dockerfile`, Nginx проксирует `/api`, `/oidc`, `/.well-known` на API). Docker Compose на Render напрямую не запускается, сервисы создаются по отдельности:

1. **База:** New → PostgreSQL, скопируйте *Internal Database URL*.
2. **API:** New → Web Service → Docker, корень `backend`. Переменные: `DATABASE_URL` (из шага 1), `PORT=8080`, `OIDC_ISSUER=https://<адрес-веб-сервиса>`, а также `BREVO_API_KEY`, `SMTP_FROM`, `TELEGRAM_BOT_TOKEN`.
3. **Веб:** New → Web Service → Docker, корень `frontend`. В `frontend/nginx.conf` в `proxy_pass` укажите внутренний адрес API-сервиса вместо `http://api:8080`.
4. Откройте адрес веб-сервиса, при первом запуске демо-данные создаются автоматически.

Бесплатный тариф «засыпает» после простоя, первый запрос может идти до минуты. Перед демонстрацией откройте стенд заранее.

---

## ✅ Реализованные сценарии и где их проверить

### Обязательные

| № | Сценарий | Как проверить |
|---|---|---|
| С1 | Один аккаунт с несколькими ролями, права по активной роли | Войти как `morozov@fsp56.ru`, переключить роль в шапке (Спортсмен ↔ Тренер); недоступные разделы закрыты, API проверяет роль на сервере |
| С2 | Создание и публикация соревнования (дисциплина, статус, тип из справочника, возрастная группа, календарный план, положение) | Федерация → «Соревнования» → создать → опубликовать |
| С3 | Заявка на соревнование, проверка возраста и дисциплины, модерация | Спортсмен подаёт заявку → Федерация одобряет или отклоняет |
| С4 | Привязка тренера и спортсмена с подтверждением | Тренер → «Мои спортсмены» → запрос; спортсмен подтверждает |
| С5 | Результаты → автоматический протокол → история спортсмена | Федерация вносит результаты; в «Документах» протокол, у спортсмена запись в истории |
| С6 | Судейская коллегия, категории, заявки судей | Федерация назначает судей; судья видит назначения и подаёт заявку на категорию |
| С7 | Заявки на разряд, фиксация, предупреждения о сроках | Тренер или спортсмен подаёт заявку → Федерация присваивает; предупреждения об истечении видны ролям |
| С8 | Реестр и отчёт о динамике численности (.xlsx) | «Документы» → численность занимающихся |
| С9 | Рейтинг по дисциплинам, прозрачный расчёт, сборная и проект приказа | «Рейтинг» (фильтры, расчёт по спортсмену) → «Сборная» |
| С10 | Выгрузка документов (протокол, отчёт по спортсмену) | «Документы» |

### Дополнительные и собственные

| Сценарий | Где смотреть |
|---|---|
| Д1 ФСП ID (OpenID Connect, PKCE) | `/oidc/demo`, регистрация клиентов `/oidc/admin`, discovery `/.well-known/openid-configuration` |
| Д2 Интеграции: токены, вебхуки, импорт результатов, API | `/api/integrations/console`, [`docs/INTEGRATIONS.md`](docs/INTEGRATIONS.md), спецификация `/api/v1/openapi.yaml` |
| Д3, Д5, Д9, Д11, Д12 | [`docs/EXTRAS.md`](docs/EXTRAS.md) |
| Д4, Д6, Д7, Д8, Д10 | [`docs/EXTRAS.md`](docs/EXTRAS.md), раздел «Д4, Д6, Д7, Д8, Д10» |
| **Собственный: «Готовность к старту»** (допуск участников по чек-листу, напоминания) | Страница «Готовность к старту», [`docs/READINESS.md`](docs/READINESS.md) |

Сверка с критериями оценки: [`docs/CRITERIA_MATRIX.md`](docs/CRITERIA_MATRIX.md), чек-лист приёмки: [`docs/ACCEPTANCE_CHECKLIST.md`](docs/ACCEPTANCE_CHECKLIST.md), сценарий демонстрации: [`docs/DEMO_SCENARIO.md`](docs/DEMO_SCENARIO.md).

---

## 🧱 Архитектура

```text
frontend/                 React + TypeScript + Vite (адаптивный интерфейс, светлая/тёмная тема)
backend/cmd/api           точка входа HTTP API (Go)
backend/internal/
  auth, platform          сессии, роли, основные процессы федерации
  competitions, athletes, ratings, dashboard   доменные модули
  readiness               чек-лист допуска («Готовность к старту»)
  oidc                    ФСП ID (OpenID Connect провайдер)
  integrations/extapi     внешний API, токены, вебхуки
  integrations/telegram   бот и уведомления
  emailcode               коды входа по email (Brevo API / SMTP)
  database, httpserver    PostgreSQL, миграции, seed, маршруты
backend/migrations        схема БД (001…008)
docs/                     документация и описание API
docker-compose.yml        web + api + postgres
```

Модель данных описана в миграциях `backend/migrations/*.sql` и разделах `docs/*.md`. Права проверяются на сервере по активной роли сессии. Все действия записываются в журнал. Для несовершеннолетних требуется согласие законного представителя.

### Основные эндпоинты

| Метод | Путь | Назначение |
|---|---|---|
| `GET` | `/api/health` | Проверка сервиса |
| `POST` | `/api/auth/register` · `/api/auth/login` · `/api/auth/logout` | Регистрация, вход, выход |
| `GET` | `/api/auth/me` · `POST /api/auth/switch-role` | Текущий пользователь, смена роли |
| `GET/POST` | `/api/competitions` | Календарь, создание |
| `GET` | `/api/athletes` · `/api/rating` · `/api/team` | Реестр, рейтинг, сборная |
| `GET` | `/api/readiness?competition_id=N` | Готовность участников к старту |
| `GET` | `/api/v1/competitions` и др. | Внешнее API по токену, [`docs/INTEGRATIONS.md`](docs/INTEGRATIONS.md) |

Полный перечень эндпоинтов — [`docs/openapi.yaml`](docs/openapi.yaml) и `docs/*.md`. Для защищённых вызовов: `Authorization: Bearer <token>`.

---

## 🛠️ Решение проблем

| Симптом | Что делать |
|---|---|
| Порт `8080` или `8081` занят | Остановите другое приложение или измените порты в `docker-compose.yml` |
| Письмо с кодом не приходит | Проверьте `docker compose logs api`; на хостинге используйте `BREVO_API_KEY`; без почты код виден в логе |
| В Telegram не приходят уведомления | Проверьте `TELEGRAM_BOT_TOKEN` и что аккаунт привязан командой `/link КОД` |
| Ошибка на странице ФСП ID или интеграций | Для консоли нужна активная роль «Федерация»; на хостинге задайте `OIDC_ISSUER` и проверьте проксирование `/oidc`, `/.well-known` |
| Вебхук не доставляется | Адрес получателя должен быть доступен серверу платформы из интернета (не `localhost`); используйте, например, webhook.site |
| Хочется чистое состояние | `docker compose down -v && docker compose up --build` |

## Тесты

```bash
cd backend
go test ./...
go vet ./...
```

Описание проверок — [`docs/TESTS.md`](docs/TESTS.md).
