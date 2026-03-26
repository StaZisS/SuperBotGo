# Диаграмма компонентов

Общая архитектура SuperBotGo — от пользователя до базы данных.

## Обзор системы

```mermaid
graph LR
    subgraph External["Внешние системы"]
        direction TB
        TG["Telegram"]
        DS["Discord"]
        WH["Webhook-клиенты"]
        Browser["Браузер"]
    end

    subgraph App["SuperBotGo"]
        direction LR

        subgraph Layer1[" "]
            direction TB
            channel["<b>channel</b><br/>ChannelManager<br/>AdapterRegistry<br/>Telegram · Discord"]
            admin["<b>admin</b><br/>SPA (React)<br/>Admin API"]
        end

        subgraph Layer2[" "]
            direction TB
            trigger["<b>trigger</b><br/>TriggerRouter<br/>CronScheduler<br/>HTTP Handler"]
            state["<b>state</b><br/>StateManager<br/>"]
            authz["<b>authz</b><br/>Authorizer"]
            i18n["<b>i18n + locale</b>"]
        end

        subgraph Layer3[" "]
            direction TB
            plugin["<b>plugin</b><br/>PluginManager<br/>SenderAPI"]
            notification["<b>notification</b><br/>NotifyAPI"]
        end

        subgraph Layer4[" "]
            direction TB
            wasm["<b>wasm</b><br/>Runtime (wazero)<br/>Loader<br/>EventBus · Registry"]
            hostapi["<b>hostapi</b><br/>KV · SQL · HTTP<br/>Notify · RPC"]
        end
    end

    subgraph Infra["Хранилища"]
        direction TB
        PG[("PostgreSQL")]
        RD[("Redis")]
        BS["BlobStore<br/>Local FS / S3"]
    end

    %% Внешние → приложение
    TG <--> channel
    DS <--> channel
    WH --> trigger
    Browser --> admin

    %% channel → trigger (все сообщения идут через trigger)
    channel --> trigger
    channel --> i18n

    %% trigger — единая точка: authz + state + маршрутизация
    trigger --> authz
    trigger --> state
    trigger --> plugin


    %% admin
    admin --> plugin

    %% plugin → wasm
    plugin --> wasm
    wasm --> hostapi

    %% hostapi → сервисы
    hostapi --> notification
    hostapi --> plugin
    hostapi --> PG

    %% обратные вызовы
    plugin --> channel
    notification --> channel

    %% → Хранилища
    authz & notification --> PG
    state --> RD
    trigger --> RD
    admin --> BS
    wasm --> BS

    %% Стили
    classDef external fill:#e1f5fe,stroke:#0288d1,color:#01579b
    classDef pkg_channel fill:#f3e5f5,stroke:#7b1fa2,color:#4a148c
    classDef pkg_domain fill:#fff3e0,stroke:#ef6c00,color:#e65100
    classDef pkg_plugin fill:#e8f5e9,stroke:#2e7d32,color:#1b5e20
    classDef pkg_infra fill:#f5f5f5,stroke:#616161,color:#212121
    classDef pkg_admin fill:#e0f2f1,stroke:#00695c,color:#004d40
    classDef db fill:#ffecb3,stroke:#ff8f00,color:#e65100
    classDef invisible fill:none,stroke:none

    class TG,DS,WH,Browser external
    class channel pkg_channel
    class state,authz,i18n pkg_domain
    class plugin,notification,trigger pkg_plugin
    class wasm,hostapi pkg_infra
    class admin pkg_admin
    class PG,RD,BS db
    class Layer1,Layer2,Layer3,Layer4 invisible
```

## Описание компонентов

### `internal/channel`

| Компонент | Файл | Назначение |
|-----------|------|------------|
| **ChannelManager** | `manager.go` | Точка входа для входящих сообщений. Оркестрирует пользователя, состояние, авторизацию, маршрутизацию |
| **AdapterRegistry** | `registry.go` | Реестр каналов; маршрутизация исходящих сообщений по `ChannelType` |
| **Telegram Adapter** | `telegram/` | Приём и отправка сообщений через Telegram Bot API |
| **Discord Adapter** | `discord/` | Приём и отправка сообщений через Discord Gateway |

### `internal/state`

| Компонент | Файл | Назначение |
|-----------|------|------------|
| **StateManager** | `manager.go` | Управление многошаговыми диалогами. Хранит состояние в Redis |

### `internal/authz`

| Компонент | Файл | Назначение |
|-----------|------|------------|
| **Authorizer** | `authorizer.go` | RBAC-авторизация с TTL-кешем и expr-lang для оценки политик. Управление ролями |

### `internal/plugin`

| Компонент | Файл | Назначение |
|-----------|------|------------|
| **UpdateRouter** | `router.go` | Маршрутизация `CommandRequest` в нужный плагин |
| **PluginManager** | `manager.go` | Реестр и жизненный цикл плагинов (native + WASM) |
| **SenderAPI** | `sender.go` | Высокоуровневый API для отправки сообщений из плагинов |

### `internal/wasm`

| Компонент | Файл | Назначение |
|-----------|------|------------|
| **Runtime** | `runtime/` | Движок WebAssembly на базе wazero. Пул инстансов, AOT-кеш, лимиты памяти |
| **Loader** | `adapter/loader.go` | Загрузка `.wasm` модулей, hot-reload, валидация манифеста |
| **Host API** | `hostapi/` | Функции хоста для WASM: HTTP, KV, SQL, уведомления, RPC между плагинами |
| **EventBus** | `eventbus/` | Pub/sub для плагинов. At-least-once доставка, DLQ, retry с backoff |
| **Registry** | `registry/` | Версии, зависимости, подписи плагинов |

### `internal/trigger`

| Компонент | Файл | Назначение |
|-----------|------|------------|
| **TriggerRouter** | `router.go` | Маршрутизация событий (HTTP, Cron, Event) в плагины |
| **CronScheduler** | `cron.go` | Распределённый cron с блокировкой через Redis |
| **HTTP Handler** | `http.go` | Приём webhook-запросов |

### `internal/notification`

| Компонент | Файл | Назначение |
|-----------|------|------------|
| **NotifyAPI** | `notify.go` | Маршрутизация уведомлений с учётом приоритета, рабочих часов, предпочтений канала. Failover между каналами |

### `internal/i18n` + `internal/locale`

| Компонент | Назначение |
|-----------|------------|
| **i18n** | Мультиязычность: резолвинг локали пользователя/чата, TOML-файлы переводов |

### `internal/admin`

| Компонент | Файл | Назначение |
|-----------|------|------------|
| **SPA** | `web/admin/` | React-приложение для управления платформой |
| **Admin API** | `api/` | REST API: загрузка/установка плагинов, управление правами, статусы каналов |

### Внешние хранилища

| Компонент | Назначение |
|-----------|------------|
| **PostgreSQL** | Основная БД: пользователи, чаты, роли, плагины, уведомления, авторизация |
| **Redis** | Состояние диалогов, распределённые блокировки, TTL-кеш |
| **BlobStore** | Хранение `.wasm` файлов (Local FS или S3/MinIO). Используется Admin API и Loader |

## Потоки данных

### Обработка сообщения пользователя

```mermaid
sequenceDiagram
    actor User
    participant Ch as Channel Adapter
    participant CM as ChannelManager
    participant US as UserService
    participant SM as StateManager
    participant AZ as Authorizer
    participant UR as UpdateRouter
    participant P as Plugin
    participant SA as SenderAPI

    User->>Ch: Сообщение
    Ch->>CM: OnUpdate(ctx, Update)
    CM->>US: FindOrCreateUser()
    US-->>CM: GlobalUser
    CM->>SM: StartCommand / ProcessInput
    SM-->>CM: StateResult{Params}
    CM->>AZ: CheckCommand()
    AZ-->>CM: allowed
    CM->>UR: Route(CommandRequest)
    UR->>P: HandleEvent(Event)
    P->>SA: Reply(Message)
    SA->>Ch: SendToChat()
    Ch->>User: Ответ
```

### Загрузка WASM-плагина

```mermaid
sequenceDiagram
    actor Admin
    participant UI as Admin SPA
    participant API as Admin API
    participant BS as BlobStore
    participant PS as PubSub
    participant LDR as Loader
    participant RT as WASM Runtime
    participant PM as PluginManager

    Admin->>UI: Upload .wasm
    UI->>API: POST /plugins
    API->>BS: Store(binary)
    API->>API: Save metadata (PG)
    API->>PS: Publish(PluginInstalled)
    PS-->>LDR: OnPluginInstalled
    LDR->>BS: Fetch(binary)
    LDR->>LDR: Validate manifest
    LDR->>RT: Compile + instantiate
    RT-->>LDR: WasmPlugin
    LDR->>PM: Register(plugin)
```

### Выполнение WASM host-вызова

```mermaid
sequenceDiagram
    participant WP as WASM Plugin
    participant HA as Host API
    participant KV as KV Store
    participant HTTP as HTTP Client
    participant EB as EventBus
    participant NA as NotifyAPI

    WP->>HA: host_kv_get(key)
    HA->>KV: Get(pluginID, key)
    KV-->>HA: value
    HA-->>WP: bytes

    WP->>HA: host_http_request(url)
    HA->>HA: Check permissions + rate limit
    HA->>HTTP: Do(request)
    HTTP-->>HA: response
    HA-->>WP: bytes

    WP->>HA: host_emit_event(topic, data)
    HA->>EB: Publish(topic, data)
    Note over EB: At-least-once delivery<br/>DLQ on failure

    WP->>HA: host_notify(userID, msg)
    HA->>NA: Send(notification)
    Note over NA: Priority routing<br/>Channel failover
```
