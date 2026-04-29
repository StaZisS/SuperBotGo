# Конфигурация платформы

Эта страница описывает конфигурацию самого **SuperBotGo host**: `config.yaml`, переменные окружения `BOT_*`, каналы, БД, Redis, S3, TSU и runtime-параметры.

Конфигурация **WASM-плагинов** описана отдельно: [Конфигурация плагина](/guide/configuration).

## Источники конфигурации

Host загружает параметры в таком порядке:

1. `config.yaml`
2. переменные окружения `BOT_*`

Если один и тот же параметр задан и в YAML, и через env, env-переменная имеет приоритет.

::: info Runtime defaults vs example values
Таблицы ниже показывают значения по умолчанию из кода загрузчика.
:::

## Правило именования env-переменных

Все env-переменные начинаются с префикса `BOT_`.

- переход между секциями (`.`) превращается в один `_`
- символ `_` внутри имени ключа превращается в `__`

Примеры:

| YAML-ключ | Env |
|---|---|
| `database.host` | `BOT_DATABASE_HOST` |
| `user_auth.session_secret` | `BOT_USER__AUTH_SESSION__SECRET` |
| `telegram.webhook_url` | `BOT_TELEGRAM_WEBHOOK__URL` |
| `admin.api_key` | `BOT_ADMIN_API__KEY` |
| `filestore.max_file_size` | `BOT_FILESTORE_MAX__FILE__SIZE` |

::: warning Частая ошибка
Имена вроде `BOT_TELEGRAM_WEBHOOK_URL`, `BOT_ADMIN_API_KEY` и `BOT_DISCORD_SHARD_ID` не совпадают с текущим загрузчиком конфига. Для ключей с `_` внутри имени нужны двойные подчёркивания: `BOT_TELEGRAM_WEBHOOK__URL`, `BOT_ADMIN_API__KEY`, `BOT_DISCORD_SHARD__ID`.
:::

## Минимум для запуска

Минимально рабочий host требует:

- доступный PostgreSQL; `database.host` и `database.dbname` должны быть непустыми
- доступный Redis; host подключается к `redis.addr` на старте
- корректную S3-конфигурацию для `filestore` и `admin.s3`, если вы не используете dev-значения из `config.example.yaml`

Остальные секции условные:

- `telegram`, `discord`, `vk`, `mattermost` нужны только для соответствующих каналов
- `tsu_accounts` и `user_auth` нужны для browser login через TSU
- `spicedb` нужна для полноценной внешней authorization graph интеграции
- `smtp` нужна для email-сценариев админки
- `university_sync` нужна только если включён фоновый sync

## Условные требования и валидация

| Условие | Требование |
|---|---|
| `telegram.mode=webhook` | обязателен `telegram.webhook_url` |
| `vk.mode=callback` | обязателен `vk.callback_url` |
| задан `spicedb.endpoint` или `spicedb.token` | оба поля должны быть заданы вместе |
| задан `tsu_accounts.application_id` или `tsu_accounts.secret_key` | оба поля должны быть заданы вместе |
| задан `admin.s3.access_key` или `admin.s3.secret_key` | оба поля должны быть заданы вместе |
| задан `filestore.s3.access_key` или `filestore.s3.secret_key` | оба поля должны быть заданы вместе |
| задан `mattermost.url` или `mattermost.token` | оба поля должны быть заданы вместе |
| задан `mattermost.actions_url` или `mattermost.actions_secret` | оба поля должны быть заданы вместе |
| задан `smtp.host` или `smtp.from` | оба поля должны быть заданы вместе |
| `discord.shard_id` | должен быть в диапазоне `[0, shard_count)` |
| `vk.callback_path` | должен начинаться с `/` |
| `mattermost.actions_path` | должен начинаться с `/` |
| `admin.blob_store` | сейчас допустимо только значение `s3` |
| `wasm.events_backend` | допустимо только `memory` или `postgres` |

Host также отклоняет явные placeholder-значения вроде `change-me`, `replace-me`, `YOUR_*`, `API_KEY`, `PASSWORD`, `SECRET`, `TOKEN`.

## Справочник параметров

### Core

| YAML-ключ | Env | По умолчанию | Когда нужен | Описание |
|---|---|---|---|---|
| `default_locale` | `BOT_DEFAULT__LOCALE` | `en` | всегда | Локаль по умолчанию |
| `database.host` | `BOT_DATABASE_HOST` | пусто | всегда | Хост PostgreSQL |
| `database.port` | `BOT_DATABASE_PORT` | `5432` | всегда | Порт PostgreSQL |
| `database.user` | `BOT_DATABASE_USER` | пусто | обычно всегда | Пользователь PostgreSQL |
| `database.password` | `BOT_DATABASE_PASSWORD` | пусто | обычно всегда | Пароль PostgreSQL |
| `database.dbname` | `BOT_DATABASE_DBNAME` | пусто | всегда | Имя базы |
| `database.sslmode` | `BOT_DATABASE_SSLMODE` | `prefer` | всегда | Режим SSL для PostgreSQL |
| `redis.addr` | `BOT_REDIS_ADDR` | `localhost:6379` | всегда | Адрес Redis |
| `redis.password` | `BOT_REDIS_PASSWORD` | пусто | если нужен пароль | Пароль Redis |
| `redis.db` | `BOT_REDIS_DB` | `0` | всегда | Номер Redis DB |
| `admin.port` | `BOT_ADMIN_PORT` | `8080` | всегда | Порт Admin API |
| `admin.modules_dir` | `BOT_ADMIN_MODULES__DIR` | `./wasm_modules` | всегда | Директория локальных wasm-модулей |
| `admin.blob_store` | `BOT_ADMIN_BLOB__STORE` | `s3` | всегда | Backend хранилища wasm-бинарей |
| `admin.api_key` | `BOT_ADMIN_API__KEY` | пусто | если нужен Bearer-доступ к Admin API | API-ключ админки |
| `wasm.reconfigure_enabled` | `BOT_WASM_RECONFIGURE__ENABLED` | `true` | опционально | Применение config через `OnReconfigure` без полного reload |
| `wasm.rpc_enabled` | `BOT_WASM_RPC__ENABLED` | `false` | если нужен inter-plugin RPC | Включает вызовы между wasm-плагинами |
| `wasm.events_backend` | `BOT_WASM_EVENTS__BACKEND` | `memory` | опционально | Backend event bus: `memory` или `postgres` |
| `wasm.strict_migrate` | `BOT_WASM_STRICT__MIGRATE` | `true` | опционально | Строгий режим plugin migrations |
| `wasm.http_policy_enabled` | `BOT_WASM_HTTP__POLICY__ENABLED` | `false` | если нужны HTTP policy requirements | Включает enforcement HTTP policy config |

### Channels

| YAML-ключ | Env | По умолчанию | Когда нужен | Описание |
|---|---|---|---|---|
| `telegram.token` | `BOT_TELEGRAM_TOKEN` | пусто | если нужен Telegram | Токен Telegram-бота |
| `telegram.mode` | `BOT_TELEGRAM_MODE` | `polling` | если нужен Telegram | `polling` или `webhook` |
| `telegram.webhook_url` | `BOT_TELEGRAM_WEBHOOK__URL` | пусто | при `telegram.mode=webhook` | Публичный webhook URL |
| `telegram.webhook_secret` | `BOT_TELEGRAM_WEBHOOK__SECRET` | пусто | опционально | Секрет валидации Telegram webhook |
| `telegram.webhook_listen` | `BOT_TELEGRAM_WEBHOOK__LISTEN` | пусто | при webhook при необходимости отдельного bind | Локальный адрес HTTP listener |
| `discord.token` | `BOT_DISCORD_TOKEN` | пусто | если нужен Discord | Токен Discord-бота |
| `discord.shard_id` | `BOT_DISCORD_SHARD__ID` | `0` | при шардинге | Индекс шарда |
| `discord.shard_count` | `BOT_DISCORD_SHARD__COUNT` | `1` | при шардинге | Общее количество шардов |
| `vk.token` | `BOT_VK_TOKEN` | пусто | если нужен VK | Токен VK/community |
| `vk.mode` | `BOT_VK_MODE` | `longpoll` | если нужен VK | `longpoll` или `callback` |
| `vk.callback_url` | `BOT_VK_CALLBACK__URL` | пусто | при `vk.mode=callback` | Публичный callback URL |
| `vk.callback_path` | `BOT_VK_CALLBACK__PATH` | `/vk/callback` | при callback или если нужен кастомный path | Локальный HTTP path |
| `mattermost.url` | `BOT_MATTERMOST_URL` | пусто | если нужен Mattermost | URL Mattermost instance |
| `mattermost.token` | `BOT_MATTERMOST_TOKEN` | пусто | если нужен Mattermost | Bot token Mattermost |
| `mattermost.actions_url` | `BOT_MATTERMOST_ACTIONS__URL` | пусто | для interactive actions | Публичный URL callbacks |
| `mattermost.actions_path` | `BOT_MATTERMOST_ACTIONS__PATH` | `/mattermost/actions` | для interactive actions или кастомного path | Локальный HTTP path |
| `mattermost.actions_secret` | `BOT_MATTERMOST_ACTIONS__SECRET` | пусто | вместе с `actions_url` | Секрет валидации actions |

### Storage

| YAML-ключ | Env | По умолчанию | Когда нужен | Описание |
|---|---|---|---|---|
| `filestore.default_ttl` | `BOT_FILESTORE_DEFAULT__TTL` | `24h` | обычно всегда | TTL пользовательских файлов |
| `filestore.max_file_size` | `BOT_FILESTORE_MAX__FILE__SIZE` | `52428800` | обычно всегда | Максимальный размер файла в байтах |
| `filestore.s3.bucket` | `BOT_FILESTORE_S3_BUCKET` | пусто | всегда | Bucket для пользовательских файлов |
| `filestore.s3.region` | `BOT_FILESTORE_S3_REGION` | пусто | почти всегда | S3 region |
| `filestore.s3.endpoint` | `BOT_FILESTORE_S3_ENDPOINT` | пусто | для MinIO/S3-compatible | Кастомный endpoint |
| `filestore.s3.access_key` | `BOT_FILESTORE_S3_ACCESS__KEY` | пусто | если используются явные static credentials | S3 access key |
| `filestore.s3.secret_key` | `BOT_FILESTORE_S3_SECRET__KEY` | пусто | вместе с `access_key` | S3 secret key |
| `filestore.s3.prefix` | `BOT_FILESTORE_S3_PREFIX` | пусто | опционально | Префикс ключей файлов |
| `admin.s3.bucket` | `BOT_ADMIN_S3_BUCKET` | пусто | всегда | Bucket для wasm-модулей |
| `admin.s3.region` | `BOT_ADMIN_S3_REGION` | пусто | почти всегда | S3 region |
| `admin.s3.endpoint` | `BOT_ADMIN_S3_ENDPOINT` | пусто | для MinIO/S3-compatible | Кастомный endpoint |
| `admin.s3.access_key` | `BOT_ADMIN_S3_ACCESS__KEY` | пусто | если используются явные static credentials | S3 access key |
| `admin.s3.secret_key` | `BOT_ADMIN_S3_SECRET__KEY` | пусто | вместе с `access_key` | S3 secret key |
| `admin.s3.prefix` | `BOT_ADMIN_S3_PREFIX` | пусто | опционально | Префикс ключей wasm-артефактов |

### Auth, Integrations and Background Jobs

| YAML-ключ | Env | По умолчанию | Когда нужен | Описание |
|---|---|---|---|---|
| `user_auth.session_secret` | `BOT_USER__AUTH_SESSION__SECRET` | пусто | для production browser sessions | Секрет cookie-сессий frontend login |
| `tsu_accounts.application_id` | `BOT_TSU__ACCOUNTS_APPLICATION__ID` | пусто | для TSU login | OAuth application ID |
| `tsu_accounts.secret_key` | `BOT_TSU__ACCOUNTS_SECRET__KEY` | пусто | для TSU login | OAuth secret |
| `tsu_accounts.callback_url` | `BOT_TSU__ACCOUNTS_CALLBACK__URL` | пусто | для публичного TSU login flow | Публичный callback URL |
| `tsu_accounts.base_url` | `BOT_TSU__ACCOUNTS_BASE__URL` | `https://accounts.kreosoft.space` | опционально | Base URL сервиса TSU.Accounts |
| `smtp.host` | `BOT_SMTP_HOST` | пусто | для отправки email из админки | SMTP host |
| `smtp.port` | `BOT_SMTP_PORT` | `587` | для SMTP | SMTP port |
| `smtp.username` | `BOT_SMTP_USERNAME` | пусто | если нужен SMTP auth | SMTP username |
| `smtp.password` | `BOT_SMTP_PASSWORD` | пусто | если нужен SMTP auth | SMTP password |
| `smtp.from` | `BOT_SMTP_FROM` | пусто | вместе с `smtp.host` | Адрес отправителя |
| `spicedb.endpoint` | `BOT_SPICEDB_ENDPOINT` | пусто | для внешней authz graph интеграции | gRPC endpoint SpiceDB |
| `spicedb.token` | `BOT_SPICEDB_TOKEN` | пусто | вместе с `endpoint` | Preshared token SpiceDB |
| `spicedb.insecure` | `BOT_SPICEDB_INSECURE` | `false` | если нужен insecure transport | Отключает TLS для dev/local |
| `university_sync.enabled` | `BOT_UNIVERSITY__SYNC_ENABLED` | `false` | для фоновой синхронизации | Включает puller |
| `university_sync.interval` | `BOT_UNIVERSITY__SYNC_INTERVAL` | пусто | если включён sync | Интервал вида `1h`, `30m` |
| `university_sync.base_url` | `BOT_UNIVERSITY__SYNC_BASE__URL` | пусто | если включён sync | Base URL внешней университетской системы |
| `university_sync.token` | `BOT_UNIVERSITY__SYNC_TOKEN` | пусто | если включён sync и нужна auth | Токен внешней системы |

## Пример env-only конфигурации

```bash
BOT_DATABASE_HOST=postgres
BOT_DATABASE_PORT=5432
BOT_DATABASE_USER=superbot
BOT_DATABASE_PASSWORD=superbot_secret
BOT_DATABASE_DBNAME=superbot

BOT_REDIS_ADDR=redis:6379

BOT_FILESTORE_S3_BUCKET=user-files
BOT_FILESTORE_S3_REGION=us-east-1
BOT_FILESTORE_S3_ENDPOINT=http://minio:9000
BOT_FILESTORE_S3_ACCESS__KEY=minioadmin
BOT_FILESTORE_S3_SECRET__KEY=minioadmin

BOT_ADMIN_S3_BUCKET=wasm-plugins
BOT_ADMIN_S3_REGION=us-east-1
BOT_ADMIN_S3_ENDPOINT=http://minio:9000
BOT_ADMIN_S3_ACCESS__KEY=minioadmin
BOT_ADMIN_S3_SECRET__KEY=minioadmin

BOT_ADMIN_API__KEY=super-secret-admin-key
BOT_USER__AUTH_SESSION__SECRET=super-secret-session-key

BOT_TELEGRAM_TOKEN=123:ABC
BOT_TELEGRAM_MODE=polling
```
