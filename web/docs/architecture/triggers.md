# Система триггеров

Триггеры — единый механизм доставки событий из внешнего мира в плагины.
Четыре типа триггеров покрывают все сценарии: интерактивные команды в мессенджерах,
входящие вебхуки, расписание и межплагинные события.

## Диаграмма классов

```mermaid
classDiagram
    direction TB

    class Router {
        -registry *Registry
        -plugins *plugin.Manager
        +RouteEvent(ctx, event) *EventResponse
    }

    class Registry {
        -mu RWMutex
        -httpRoutes map~string, httpRoute~
        -apiKeys map~string, string~
        -cronScheduler *CronScheduler
        +RegisterTriggers(pluginID, triggers)
        +UnregisterTriggers(pluginID)
        +LookupHTTP(pluginID, path, method) string
        +GetAPIKey(pluginID) string
        +SetCronScheduler(cs)
    }

    class httpRoute {
        +PluginID string
        +TriggerName string
        +Methods map~string, bool~
    }

    class CronScheduler {
        -mu Mutex
        -cron *cron.Cron
        -entries map~string, []cronEntry~
        -router *Router
        -redis *redis.Client
        -running sync.Map
        +AddSchedule(pluginID, triggerName, schedule)
        +RemoveAll(pluginID)
        +Start()
        +Stop()
        +SetRedis(rc)
        -fire(pluginID, triggerName)
        -tryLock(pluginID, triggerName, fireTime) bool
    }

    class cronEntry {
        +EntryID cron.EntryID
        +TriggerName string
        +Schedule string
    }

    class HTTPTriggerHandler {
        -router *Router
        -registry *Registry
        -basePath string
        -metrics *Metrics
        +ServeHTTP(w, r)
        +SetMetrics(m)
    }

    class Event {
        +ID string
        +TriggerType TriggerType
        +TriggerName string
        +PluginID string
        +Timestamp int64
        +Data json.RawMessage
    }

    class EventResponse {
        +Status string
        +Error string
        +Reply string
        +ReplyTexts map~string, string~
        +Data json.RawMessage
        +Logs []LogEntry
    }

    class TriggerType {
        <<enumeration>>
        http
        cron
        event
        messenger
    }

    class MessengerTriggerData {
        +UserID GlobalUserID
        +ChannelType ChannelType
        +ChatID string
        +CommandName string
        +Params OptionMap
        +Locale string
    }

    class HTTPTriggerData {
        +Method string
        +Path string
        +Query map~string, string~
        +Headers map~string, string~
        +Body string
        +RemoteAddr string
    }

    class HTTPResponseData {
        +StatusCode int
        +Headers map~string, string~
        +Body string
    }

    class CronTriggerData {
        +ScheduleName string
        +FireTime int64
    }

    class EventTriggerData {
        +Topic string
        +Payload json.RawMessage
        +Source string
    }

    class TriggerDef {
        +Name string
        +Type string
        +Description string
        +Path string
        +Methods []string
        +Schedule string
        +Topic string
        +Nodes []NodeDef
    }

    class Plugin {
        <<interface>>
        +ID() string
        +HandleEvent(ctx, event) *EventResponse
    }

    %% Relationships
    Router --> Registry
    Router --> Plugin : routes to
    Router ..> Event : accepts
    Router ..> EventResponse : returns

    Registry *-- httpRoute : httpRoutes
    Registry --> CronScheduler : cronScheduler
    CronScheduler --> Router : fire → RouteEvent
    CronScheduler *-- cronEntry : entries

    HTTPTriggerHandler --> Router
    HTTPTriggerHandler --> Registry

    Event --> TriggerType
    Event ..> MessengerTriggerData : Data (messenger)
    Event ..> HTTPTriggerData : Data (http)
    Event ..> CronTriggerData : Data (cron)
    Event ..> EventTriggerData : Data (event)

    HTTPTriggerHandler ..> HTTPResponseData : reads from EventResponse.Data
    Registry ..> TriggerDef : RegisterTriggers input
```

## Потоки обработки по типу триггера

### Messenger (команды в мессенджерах)

Messenger-триггеры **не проходят** через `Registry` / `HTTPTriggerHandler`.
Они обрабатываются в `ChannelManager` через стейт-машину диалогов
(см. [dialog-state.md](dialog-state.md)).

```mermaid
sequenceDiagram
    actor User
    participant Ch as ChannelManager
    participant SM as StateManager
    participant R as Router
    participant P as Plugin

    User ->> Ch: /schedule
    Ch ->> SM: StartCommand("schedule")
    SM -->> Ch: StepMessage (building?)
    Ch -->> User: Выберите корпус

    User ->> Ch: callback: "2"
    Ch ->> SM: ProcessInput("2")
    SM -->> Ch: StepMessage (room?)
    Ch -->> User: Введите аудиторию

    User ->> Ch: "305"
    Ch ->> SM: ProcessInput("305")
    SM -->> Ch: IsComplete=true, Params
    Ch ->> R: RouteEvent(messenger event)
    R ->> P: HandleEvent(event)
    P -->> R: EventResponse{Reply}
    R -->> Ch: response
    Ch -->> User: Расписание корпуса 2, ауд. 305
```

### HTTP (вебхуки)

```
URL:  /api/triggers/http/{pluginID}/{path...}
Auth: X-Trigger-Key (опционально, per plugin)
Body: до 10 MB
```

```mermaid
sequenceDiagram
    participant Client as HTTP-клиент
    participant H as HTTPTriggerHandler
    participant Reg as Registry
    participant R as Router
    participant P as Plugin

    Client ->> H: POST /api/triggers/http/schedule/api/schedule?building=2
    H ->> Reg: LookupHTTP("schedule", "api/schedule", "POST")
    Reg -->> H: triggerName="api"
    H ->> Reg: GetAPIKey("schedule")
    Reg -->> H: "" (no key)
    H ->> H: parse body, query, headers
    H ->> R: RouteEvent(http event)
    R ->> P: HandleEvent(event)
    P -->> R: EventResponse{Data: HTTPResponseData}
    R -->> H: response
    H -->> Client: 200 {"building":"2","classes":[...]}
```

### Cron (расписание)

```mermaid
sequenceDiagram
    participant C as CronScheduler
    participant Redis as Redis
    participant R as Router
    participant P as Plugin

    Note over C: robfig/cron fires "0 7 * * *"
    C ->> C: check running map (overlap guard)
    C ->> Redis: SET NX cron_lock:schedule:daily_reminder:27443520
    Redis -->> C: OK (lock acquired)
    C ->> R: RouteEvent(cron event)
    R ->> P: HandleEvent(event)
    P -->> R: EventResponse
    Note over C: cleanup running map
```

Гарантии:
- **Overlap guard**: `sync.Map` предотвращает параллельный запуск одного триггера
- **Distributed lock**: Redis `SET NX` с гранулярностью 60 сек и TTL 2 мин
- **Fail-open**: если Redis недоступен — триггер выполняется
- **Timeout**: 30 сек на выполнение

### Event (межплагинные события)

```mermaid
sequenceDiagram
    participant P1 as Plugin A (WASM)
    participant HA as HostAPI
    participant EB as EventBus
    participant SUB as EventSubscriber
    participant R as Router
    participant P2 as Plugin B

    P1 ->> HA: host_publish_event("order.created", payload)
    HA ->> HA: check "events" permission
    HA ->> EB: Publish("order.created", payload)
    EB ->> SUB: Handle(topic, payload)
    SUB ->> R: RouteEvent(event trigger)
    R ->> P2: HandleEvent(event trigger)
    Note over EB: memory backend or Postgres queue
    Note over EB: Postgres backend = at-least-once, retry, DLQ
```

## Жизненный цикл триггеров

```mermaid
stateDiagram-v2
    [*] --> Unregistered

    Unregistered --> Registered : RegisterTriggers(pluginID, defs)
    Registered --> Active : Start() / ServeHTTP

    state Active {
        [*] --> Listening
        Listening --> Firing : event arrives
        Firing --> Listening : RouteEvent complete
    }

    Active --> Registered : plugin config update
    Registered --> Unregistered : UnregisterTriggers(pluginID)
    Active --> Unregistered : plugin disabled / deleted

    Unregistered --> [*]
```

## Регистрация

Когда WASM-плагин загружается, `Registry.RegisterTriggers` обрабатывает массив `TriggerDef`
из манифеста плагина:

| Тип | Действие при регистрации | Ключ |
|-----|-------------------------|------|
| `http` | Добавляет запись в `httpRoutes` | `{pluginID}/{path}` |
| `cron` | Вызывает `CronScheduler.AddSchedule()` | cron expression |
| `messenger` | Регистрируется отдельно в `StateManager` | command name |
| `event` | Добавляет subscription по `topic` в registry; delivery идёт через `EventSubscriber` | topic name |

При выгрузке плагина `UnregisterTriggers` удаляет все маршруты, расписания и API-ключи.

## Формат Event ID

Каждое событие получает уникальный ID — 16 случайных байт в hex-формате:
```
a1b2c3d4-e5f6-7890-abcd-ef1234567890
```
