# Host API (WASM)

Host API — набор функций, доступных WASM-плагинам для взаимодействия с платформой.
Все вызовы проходят через единый конвейер: проверка разрешений, rate limiting, трассировка,
автоочистка ресурсов.

## Диаграмма классов

```mermaid
classDiagram
    direction TB

    class HostAPI {
        -deps Dependencies
        -perms *permissionStore
        -metrics *Metrics
        -kvStore *KVStore
        -sqlStore *SQLHandleStore
        -rateLimits map~string, int~
        +RegisterHostModule(rt)
        +GrantPermissions(pluginID, perms)
        +RevokePermissions(pluginID)
        +RegisterDSN(pluginID, dsn)
        +ContextWithRateLimiter(ctx, pluginID) context
        -registerFunc(name, fn)
    }

    class Dependencies {
        +HTTP HTTPClient
        +Events EventBus
        +PluginRegistry PluginRegistry
        +Notifier Notifier
        +FileStore FileStore
    }

    class HTTPClient {
        <<interface>>
        +Do(req) *Response
    }

    class EventBus {
        <<interface>>
        +Publish(ctx, topic, payload)
    }

    class PluginRegistry {
        <<interface>>
        +CallPlugin(ctx, target, method, params) []byte
    }

    class Notifier {
        <<interface>>
        +NotifyUser(ctx, userID, text, priority)
        +NotifyChat(ctx, channelType, chatID, text, priority)
        +NotifyStudents(ctx, scope, targetID, text, priority)
    }

    class permissionStore {
        -mu RWMutex
        -perms map~string, map~string, bool~~
        +Grant(pluginID, permission)
        +Revoke(pluginID)
        +CheckPermission(pluginID, perm) bool
        +List(pluginID) []string
    }

    class RateLimiter {
        -mu Mutex
        -counts map~string, int~
        -limits map~string, int~
        -pluginID string
        +Allow(funcName) error
    }

    class KVStore {
        -mu RWMutex
        -plugins map~string, *pluginKV~
        +Get(pluginID, key) []byte, bool
        +Set(pluginID, key, value, ttl)
        +Delete(pluginID, key) bool
        +List(pluginID, prefix) []string
        +DropPlugin(pluginID)
    }

    class SQLHandleStore {
        -mu RWMutex
        -plugins map~string, *pluginSQLState~
        +RegisterDSN(pluginID, dsn)
        +UnregisterPlugin(pluginID)
        +Alloc(pluginID, traceID, handle) uint32
        +Get(pluginID, traceID, id) handle
        +Remove(pluginID, traceID, id)
        +CleanupExecution(pluginID, traceID)
    }

    %% Relationships
    HostAPI --> Dependencies
    HostAPI --> permissionStore
    HostAPI --> KVStore
    HostAPI --> SQLHandleStore

    Dependencies --> HTTPClient
    Dependencies --> EventBus
    Dependencies --> PluginRegistry
    Dependencies --> Notifier

    HostAPI ..> RateLimiter : creates per execution
```

## Все host-функции

```mermaid
graph LR
    subgraph WASM["WASM Plugin"]
        Plugin["plugin.wasm"]
    end

    subgraph HostAPI["Host API"]
        direction TB

        subgraph KV["KV Store"]
            kv_get["kv_get"]
            kv_set["kv_set"]
            kv_delete["kv_delete"]
            kv_list["kv_list"]
        end

        subgraph SQL["SQL"]
            sql_open["sql_open"]
            sql_close["sql_close"]
            sql_exec["sql_exec"]
            sql_query["sql_query"]
            sql_next["sql_next"]
            sql_rows_close["sql_rows_close"]
            sql_begin["sql_begin"]
            sql_end["sql_end"]
        end

        subgraph NET["Network"]
            http_request["http_request"]
        end

        subgraph Notify["Notifications"]
            notify_user["notify_user"]
            notify_chat["notify_chat"]
            notify_students["notify_students"]
        end

        subgraph RPC["Inter-plugin"]
            call_plugin["call_plugin"]
            publish_event["publish_event"]
        end

        subgraph Files["Files"]
            file_meta["file_meta"]
            file_read["file_read"]
            file_url["file_url"]
            file_store["file_store"]
        end
    end

    subgraph Infra["Infrastructure"]
        PG[("PostgreSQL")]
        Redis[("Redis / KV")]
        ExtHTTP["External HTTP"]
        MsgCh["Channel Adapters"]
        Plugins["Other Plugins"]
        EB["EventBus"]
        FSt[("FileStore")]
    end

    Plugin --> kv_get & kv_set & kv_delete & kv_list
    Plugin --> sql_open & sql_exec & sql_query
    Plugin --> http_request
    Plugin --> notify_user & notify_chat & notify_students
    Plugin --> call_plugin & publish_event
    Plugin --> file_meta & file_read & file_url & file_store

    kv_get & kv_set --> Redis
    file_meta & file_read & file_url & file_store --> FSt
    sql_open & sql_exec & sql_query --> PG
    http_request --> ExtHTTP
    notify_user & notify_chat --> MsgCh
    call_plugin --> Plugins
    publish_event --> EB

    classDef wasm fill:#e8f5e9,stroke:#2e7d32
    classDef host fill:#fff3e0,stroke:#ef6c00
    classDef infra fill:#e1f5fe,stroke:#0288d1
    class Plugin wasm
    class kv_get,kv_set,kv_delete,kv_list,sql_open,sql_close,sql_exec,sql_query,sql_next,sql_rows_close,sql_begin,sql_end,http_request,notify_user,notify_chat,notify_students,call_plugin,publish_event,file_meta,file_read,file_url,file_store host
    class PG,Redis,ExtHTTP,MsgCh,Plugins,EB,FSt infra
```

## Конвейер вызова

Каждый host-вызов проходит через единый wrapper в `registerFunc`:

```mermaid
flowchart TD
    A["WASM вызывает host-функцию"] --> B["readModMemory (offset, length)"]
    B --> C{RateLimiter.Allow?}
    C -- rate_limited --> ERR1["return error: rate limit exceeded"]
    C -- ok --> D{permissionStore.Check?}
    D -- denied --> ERR2["return error: permission denied"]
    D -- ok --> E["execute host function"]
    E --> F["writeModMemory (result)"]
    F --> G["log + metrics"]
    G --> H["return offset|length to WASM"]

    E -- panic --> R["recover → return error"]
    R --> G

    style ERR1 fill:#ffcdd2,stroke:#c62828
    style ERR2 fill:#ffcdd2,stroke:#c62828
    style R fill:#ffcdd2,stroke:#c62828
```

## Система разрешений

Разрешения назначаются плагину при установке на основе `requirements` из манифеста.

| Requirement | Permission | Что даёт |
|------------|------------|----------|
| `database` | `sql` | Доступ к `sql_*` функциям |
| `http` | `network` | Доступ к `http_request` |
| `kv` | `kv` | Доступ к `kv_*` функциям |
| `notify` | `notify` | Доступ к `notify_*` функциям |
| `events` | `events` | Доступ к `publish_event` |
| `file` | `file` | Доступ к `file_*` функциям |
| `plugin:X` | `plugins:call:X` | Вызов конкретного плагина X |

Проверка — **перед каждым вызовом**. Без разрешения вызов возвращает ошибку,
WASM-модуль не получает доступа к ресурсу.

## Rate Limits

Лимиты **на одно выполнение** (один HandleEvent):

| Функция | Лимит | | Функция | Лимит |
|---------|------:|-|---------|------:|
| `kv_get` | 200 | | `sql_open` | 10 |
| `kv_set` | 200 | | `sql_exec` | 100 |
| `kv_delete` | 100 | | `sql_query` | 100 |
| `kv_list` | 50 | | `sql_next` | 5000 |
| `http_request` | 20 | | `sql_begin` | 20 |
| `call_plugin` | 10 | | `sql_end` | 20 |
| `publish_event` | 50 | | `sql_close` | 10 |

`RateLimiter` создаётся через context hook на каждое выполнение и сбрасывается после.

## Сетевая песочница

`http_request` блокирует обращения к:

| Заблокировано | Причина |
|---------------|---------|
| `localhost`, `127.0.0.1`, `::1` | Loopback |
| `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` | RFC 1918 (приватные сети) |
| `169.254.169.254`, `metadata.google.internal` | Cloud metadata API (SSRF) |
| `169.254.0.0/16` | Link-local |

## SQL: управление ресурсами

```mermaid
flowchart TD
    OPEN["sql_open"] --> |"handle:1 (conn)"| QUERY["sql_query"]
    QUERY --> |"cursor:2 (rows)"| NEXT["sql_next (loop)"]
    NEXT --> CLOSE_ROWS["sql_rows_close"]
    OPEN --> BEGIN["sql_begin"]
    BEGIN --> |"tx:3"| EXEC["sql_exec"]
    EXEC --> END_TX["sql_end(commit=true)"]
    CLOSE_ROWS --> CLOSE["sql_close"]
    END_TX --> CLOSE

    subgraph Cleanup["context.AfterFunc cleanup"]
        direction LR
        C1["close open rows"]
        C2["rollback open tx"]
        C3["release connections"]
        C1 --> C2 --> C3
    end

    CLOSE -.-> Cleanup
    style Cleanup fill:#fff3e0,stroke:#ef6c00
```

- Макс. хэндлов на выполнение: **16** (connections + transactions + result sets)
- Таймаут SQL-операций: **4 секунды**
- `CleanupExecution` вызывается через `context.AfterFunc` — автоочистка при завершении

### Лимиты KV Store

| Параметр | Лимит |
|----------|-------|
| Макс. ключей на плагин | 1 000 |
| Макс. размер значения | 64 KB |
| Макс. объём на плагин | 10 MB |
| TTL | опционально, per key |

## Inter-plugin RPC

```mermaid
sequenceDiagram
    participant A as Plugin A
    participant HA as HostAPI
    participant B as Plugin B

    A ->> HA: call_plugin("pluginB", "method", params)
    HA ->> HA: check permission "plugins:call:pluginB"
    HA ->> HA: check call depth (max 5)
    HA ->> HA: check call cycle (A → B → A)
    HA ->> B: CallPlugin("method", params)
    B -->> HA: result bytes
    HA -->> A: result bytes
```

Защиты:
- **Max call depth**: 5 уровней вложенности
- **Cycle detection**: A → B → A блокируется
- **Permission**: нужен `plugins:call:{target}` для каждого целевого плагина

## Wire Protocol

Все host-функции используют единый формат сериализации:

```
┌────────┬──────────────────────────┐
│ 0x01   │  MessagePack payload     │
│ 1 byte │  variable length         │
└────────┴──────────────────────────┘
```

Параметры передаются через WASM memory:
- **Вызов**: `(offset: i32, length: i32)` → host читает из памяти WASM
- **Возврат**: `i64` = `(offset << 32) | length` → host пишет в память WASM через `alloc`

## Трассировка

Каждое выполнение получает `traceID` (16 случайных hex-байт).
Все host-вызовы логируются с:

```
trace_id, plugin_id, function, duration_ms, status (ok | error | rate_limited)
```

Пример из логов:
```
level=INFO msg="host api call" trace_id=789a8b69 plugin_id=schedule function=sql_query duration_ms=3 status=ok
```
