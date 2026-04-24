# Структура плагина

Плагин - это один бинарный `.wasm` файл. Вся логика описывается структурой `Plugin` и передаётся в `wasmplugin.Run()`.

## Структура Plugin

```go
import wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"

wasmplugin.Plugin{
    ID:           "my-plugin",              // уникальный идентификатор
    Name:         "My Plugin",              // отображаемое имя
    Version:      "1.0.0",                  // семантическая версия
    RPCMethods:   []wasmplugin.RPCMethod{}, // методы для других плагинов

    Triggers:     []wasmplugin.Trigger{},   // команды, HTTP, cron, события
    Requirements: []wasmplugin.Requirement{}, // запрашиваемые ресурсы
    Config:       configSchema,             // схема конфигурации

    OnConfigure:  func(config []byte) error { ... },   // первичная настройка при load/install
    OnReconfigure: func(prev, next []byte) error { ... }, // обновление конфига без смены бинаря
    OnEvent:      func(ctx *wasmplugin.EventContext) error { ... }, // обработчик по умолчанию
    Migrate:      func(ctx *wasmplugin.MigrateContext) error { ... }, // при обновлении версии
    Migrations:   []wasmplugin.SQLMigration{},  // SQL-миграции
}
```

### Описание полей

| Поле | Тип | Описание |
|---|---|---|
| `ID` | `string` | Уникальный идентификатор плагина. Используется в межплагинных вызовах и событиях |
| `Name` | `string` | Человекочитаемое название для отображения в UI |
| `Version` | `string` | Семантическая версия (`1.0.0`). Используется для миграций |
| `RPCMethods` | `[]RPCMethod` | Явно опубликованные RPC-методы, доступные другим плагинам |
| `Triggers` | `[]Trigger` | Список триггеров: команды, HTTP-эндпоинты, cron-расписания, подписки на события |
| `Requirements` | `[]Requirement` | Ресурсы, которые плагин запрашивает у хоста |
| `Config` | `ConfigSchema` | Типизированная схема конфигурации |
| `OnConfigure` | `func([]byte) error` | Вызывается при первичной загрузке/активации плагина |
| `OnReconfigure` | `func(previous, next []byte) error` | Вызывается при обновлении конфигурации без смены бинаря |
| `OnEvent` | `func(*EventContext) error` | Fallback-обработчик для триггеров без собственного `Handler` |
| `Migrate` | `func(*MigrateContext) error` | Вызывается при обновлении версии плагина |
| `Migrations` | `[]SQLMigration` | Декларативные SQL-миграции |

## Жизненный цикл

Плагин - это **одноразовый процесс**. Каждый вызов:

1. Хост создаёт **новый экземпляр** WASM-модуля
2. Передаёт `PLUGIN_ACTION` через переменную окружения, данные через stdin
3. Плагин обрабатывает запрос и пишет результат в stdout (JSON)
4. Экземпляр **уничтожается**

::: warning Нет общего состояния между вызовами
Каждое выполнение получает чистое окружение. Используйте [KV Store](/api/kv-store) для хранения данных между вызовами.
:::

## Протокол (actions)

Хост взаимодействует с плагином через переменную окружения `PLUGIN_ACTION`:

| Action | Когда вызывается | Stdin | Stdout |
|---|---|---|---|
| `meta` | Загрузка плагина | - | PluginMeta JSON |
| `configure` | Первичная настройка при load/install | Config JSON | Error JSON (опц.) |
| `reconfigure` | Обновление конфига без смены wasm-бинаря | ReconfigureRequest JSON | Error JSON (опц.) |
| `handle_event` | Обработка события | Event JSON | EventResponse JSON |
| `handle_rpc` | RPC-вызов от другого плагина | RPCRequest JSON | RPCResponse JSON |
| `step_callback` | Валидация/пагинация шагов | Callback JSON | CallbackResponse JSON |
| `migrate` | Обновление версии | MigrateRequest JSON | MigrateResponse JSON |

- **`meta`** - хост вызывает при загрузке, чтобы получить метаданные: ID, имя, версию, список триггеров, требования, схему конфигурации.
- **`configure`** - вызывается при первичной активации плагина или полном reload.
- **`reconfigure`** - используется при изменении конфигурации, если плагин реализует `OnReconfigure` и в host включён `wasm.reconfigure_enabled`.
- Если `OnReconfigure` не реализован или `reconfigure` отключён конфигом host, новый config применяется через controlled reload того же wasm-бинаря.
- **`handle_event`** - основной вызов при срабатывании любого триггера.
- **`handle_rpc`** - отдельный runtime path для межплагинного RPC.
- **`step_callback`** - вызывается для валидации пользовательского ввода, загрузки динамических опций и пагинации.
- **`migrate`** - вызывается при обновлении версии плагина для выполнения миграций. Если `migrate` завершился ошибкой и strict migrate включён, switch-over на новую версию не происходит.

## Функция Run()

```go
func main() {
    wasmplugin.Run(myPlugin)
}
```

`Run()` читает `PLUGIN_ACTION` из окружения и вызывает соответствующий внутренний обработчик. Работать с протоколом вручную не нужно - достаточно заполнить структуру `Plugin` и предоставить функции-обработчики.

## RPC-методы

Если плагин должен обслуживать запросы от других плагинов, он явно публикует методы в `RPCMethods`:

```go
type ResolveUserRequest struct {
    ExternalID string `msgpack:"external_id"`
}

type ResolveUserResponse struct {
    UserID int64 `msgpack:"user_id"`
}

RPCMethods: []wasmplugin.RPCMethod{
    {
        Name:        "resolve_user",
        Description: "Resolve external user to internal ID",
        Handler: func(ctx *wasmplugin.RPCContext) ([]byte, error) {
            var req ResolveUserRequest
            if err := ctx.Decode(&req); err != nil {
                return nil, err
            }
            return wasmplugin.MarshalRPC(ResolveUserResponse{UserID: 42})
        },
    },
},
```

Методы, которых нет в `RPCMethods`, снаружи недоступны.

## Требования (Requirements) {#requirements}

Плагины явно объявляют, какие ресурсы хоста им нужны. Каждый вызов Host API проверяет требования в рантайме - если ресурс не объявлен, вызов вернёт ошибку.

### Базовые требования

```go
Requirements: []wasmplugin.Requirement{
    wasmplugin.Database("Хранение пользовательских данных").Build(),
    wasmplugin.HTTP("Запросы к внешнему API").Build(),
    wasmplugin.KV("Кеширование результатов").Build(),
    wasmplugin.NotifyReq("Отправка уведомлений").Build(),
    wasmplugin.EventsReq("Публикация событий заказов").Build(),
},
```

### Именованные базы данных

По умолчанию `Database()` создаёт подключение с именем `"default"`. Если плагину нужно несколько БД, используйте `.Name()`:

```go
Requirements: []wasmplugin.Requirement{
    wasmplugin.Database("Основное хранилище").Build(),                     // "default"
    wasmplugin.Database("Аналитика (read replica)").Name("analytics").Build(),
},
```

В коде плагина:

```go
mainDB, _ := sql.Open("superbot", "")            // "default"
analyticsDB, _ := sql.Open("superbot", "analytics")
```

Администратор указывает строки подключения в секции `databases` конфига:

```json
{
  "databases": {
    "default": "postgres://user:pass@host/main",
    "analytics": "postgres://user:pass@host/analytics"
  }
}
```

### Зависимость от другого плагина

```go
wasmplugin.PluginDep("auth-plugin", "Проверка токенов авторизации").Build(),
```

### Требование с конфигурацией

Метод `.WithConfig()` позволяет привязать конфигурацию к требованию:

```go
wasmplugin.HTTP("Запросы к платёжной системе").
    WithConfig(wasmplugin.HTTPPolicyConfig()).
    Build(),
```

Для `http` requirement это не просто UI-форма: хост может реально применять policy из секции `requirements.http.<name>` и ограничивать:

- `allowed_hosts`
- `allowed_methods`
- `max_request_body_bytes`
- `max_response_body_bytes`

### Таблица конструкторов

| Конструктор | Описание |
|---|---|
| `Database(desc)` | Доступ к базе данных (имя `"default"`) |
| `HTTP(desc)` | Исходящие HTTP-запросы |
| `KV(desc)` | Key-Value хранилище |
| `NotifyReq(desc)` | Отправка уведомлений |
| `EventsReq(desc)` | Публикация событий |
| `PluginDep(target, desc)` | Вызов другого плагина |
| `File(desc)` | Файловое хранилище |

Каждый конструктор возвращает builder с методами `.Name(n)`, `.WithConfig(cs)` и `.Build()`.
