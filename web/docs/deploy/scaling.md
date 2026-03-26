# Горизонтальное масштабирование

SuperBotGo поддерживает запуск нескольких экземпляров для отказоустойчивости и распределения нагрузки. Каждая платформа имеет свой механизм масштабирования.

## Проблема

Если просто запустить N экземпляров с одним токеном бота:

| Платформа | Что произойдёт |
|---|---|
| **Telegram** (long polling) | Telegram вернёт HTTP 409 — только один экземпляр может поллить |
| **Discord** (gateway) | Все N экземпляров получат каждое событие — N-кратная обработка |

SuperBotGo решает это тремя механизмами: **webhook-режим** для Telegram, **шардинг** для Discord и **дедупликация** как страховка для обоих.

## Telegram: Webhook Mode

В webhook-режиме Telegram отправляет обновления на один HTTPS-эндпоинт. Load balancer распределяет запросы между экземплярами — каждый update попадает ровно на один из них.

```mermaid
flowchart LR
    TG["Telegram API"] -->|POST /webhook| LB["Load Balancer"]
    LB --> I1["Instance 1"]
    LB --> I2["Instance 2"]
    LB --> I3["Instance 3"]
    I1 & I2 & I3 --> Redis[("Redis<br/>dedup")]
```

### Конфигурация

```yaml
telegram:
  token: "123:ABC"
  mode: webhook
  webhook_url: "https://bot.example.com/tg/webhook"
  webhook_secret: "random-secret-string"
  webhook_listen: ":8443"
```

Или через переменные окружения:

```bash
BOT_TELEGRAM_MODE=webhook
BOT_TELEGRAM_WEBHOOK_URL=https://bot.example.com/tg/webhook
BOT_TELEGRAM_WEBHOOK_SECRET=random-secret-string
BOT_TELEGRAM_WEBHOOK_LISTEN=:8443
```

| Параметр | Обязателен | Описание |
|---|---|---|
| `mode` | нет | `polling` (по умолчанию) или `webhook` |
| `webhook_url` | при `webhook` | Публичный HTTPS URL, который Telegram будет вызывать |
| `webhook_secret` | нет | Секретный токен для валидации запросов (заголовок `X-Telegram-Bot-Api-Secret-Token`) |
| `webhook_listen` | нет | Локальный адрес HTTP-сервера вебхука, например `:8443` |

::: tip
Для dev-окружения используйте `mode: polling` — не нужен публичный URL. Webhook предназначен для продакшена.
:::

::: warning
При переключении между режимами Telegram запоминает последнюю настройку. Если бот работал в webhook-режиме, а вы переключили на polling — предварительно удалите webhook через Telegram API:
```bash
curl "https://api.telegram.org/bot<TOKEN>/deleteWebhook"
```
:::

## Discord: Шардинг

Discord Gateway отправляет события через WebSocket. Без шардинга каждый экземпляр получает все события. Шардинг распределяет гильдии между экземплярами по формуле `guild_id >> 22 % shard_count == shard_id`.

```mermaid
flowchart LR
    DC["Discord Gateway"] -->|shard 0| I0["Instance 0<br/>guilds 0,3,6..."]
    DC -->|shard 1| I1["Instance 1<br/>guilds 1,4,7..."]
    DC -->|shard 2| I2["Instance 2<br/>guilds 2,5,8..."]
    I0 & I1 & I2 --> Redis[("Redis<br/>dedup")]
```

### Конфигурация

```yaml
discord:
  token: "Bot TOKEN"
  shard_id: 0
  shard_count: 3
```

| Параметр | Обязателен | Описание |
|---|---|---|
| `shard_id` | нет | Индекс шарда, 0-based (по умолчанию `0`) |
| `shard_count` | нет | Общее количество шардов (по умолчанию `1` — без шардинга) |

::: info
Discord **обязывает** использовать шардинг при >2500 серверов. Но включить его можно на любом масштабе для повышения отказоустойчивости.
:::

Каждый экземпляр должен получить уникальный `shard_id` через конфиг или переменную окружения `BOT_DISCORD_SHARD_ID`.

::: warning
При изменении `shard_count` все экземпляры должны быть перезапущены одновременно — Discord требует единого `shard_count` для всех соединений одного бота.
:::

## Дедупликация

Даже с webhook и шардингом возможны дубли: Telegram повторяет webhook при таймауте, Discord переотправляет события при реконнекте. SuperBotGo автоматически дедуплицирует обновления через Redis.

### Как это работает

1. Каждое входящее обновление получает `PlatformUpdateID` — уникальный идентификатор от платформы (например, `tg:123456789` или `dc:msg:1234567890`)
2. Перед обработкой middleware выполняет `SET NX` в Redis с TTL 5 минут
3. Если ключ уже существует — обновление пропускается (другой экземпляр уже обработал)
4. Если Redis недоступен — обновление обрабатывается (fail-open)

```mermaid
flowchart LR
    Update["Входящее<br/>обновление"] --> Check{"Redis<br/>SET NX"}
    Check -->|ключ создан| Process["Обработка"]
    Check -->|ключ существует| Skip["Пропуск<br/>(дубль)"]
    Check -->|ошибка Redis| Process
```

Дедупликация включена по умолчанию и не требует настройки. Нагрузка на Redis минимальна: каждый ключ занимает ~40 байт и живёт 5 минут.

## Справочник конфигурации

Полный список параметров, связанных с масштабированием:

| Параметр | Env | По умолчанию | Описание |
|---|---|---|---|
| `telegram.mode` | `BOT_TELEGRAM_MODE` | `polling` | Режим получения обновлений |
| `telegram.webhook_url` | `BOT_TELEGRAM_WEBHOOK_URL` | — | Публичный URL для webhook |
| `telegram.webhook_secret` | `BOT_TELEGRAM_WEBHOOK_SECRET` | — | Секрет для валидации |
| `telegram.webhook_listen` | `BOT_TELEGRAM_WEBHOOK_LISTEN` | — | Локальный адрес вебхук-сервера |
| `discord.shard_id` | `BOT_DISCORD_SHARD_ID` | `0` | Индекс шарда |
| `discord.shard_count` | `BOT_DISCORD_SHARD_COUNT` | `1` | Общее количество шардов |

## Что дальше?

- [Сборка и установка](/deploy/build) — компиляция WASM-плагинов
- [Компоненты системы](/architecture/components) — общая архитектура
- [Миграции](/deploy/migrations) — управление схемой БД
