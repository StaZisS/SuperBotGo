# Структура плагина

Плагин - это один бинарный `.wasm` файл. Вся логика описывается структурой `Plugin` и передаётся в `wasmplugin.Run()`.

## Структура Plugin

```go
import wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"

wasmplugin.Plugin{
    ID:           "my-plugin",              // уникальный идентификатор
    Name:         "My Plugin",              // отображаемое имя
    Version:      "1.0.0",                  // семантическая версия

    Triggers:     []wasmplugin.Trigger{},   // команды, HTTP, cron, события
    Requirements: []wasmplugin.Requirement{}, // запрашиваемые ресурсы
    Config:       configSchema,             // схема конфигурации

    OnConfigure:  func(config []byte) error { ... },   // при установке конфига
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
| `Triggers` | `[]Trigger` | Список триггеров: команды, HTTP-эндпоинты, cron-расписания, подписки на события |
| `Requirements` | `[]Requirement` | Ресурсы, которые плагин запрашивает у хоста |
| `Config` | `ConfigSchema` | Типизированная схема конфигурации |
| `OnConfigure` | `func([]byte) error` | Вызывается при установке или обновлении конфигурации |
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
| `configure` | Установка/обновление конфига | Config JSON | Error JSON (опц.) |
| `handle_event` | Обработка события | Event JSON | EventResponse JSON |
| `step_callback` | Валидация/пагинация шагов | Callback JSON | CallbackResponse JSON |
| `migrate` | Обновление версии | MigrateRequest JSON | MigrateResponse JSON |

- **`meta`** - хост вызывает при загрузке, чтобы получить метаданные: ID, имя, версию, список триггеров, требования, схему конфигурации.
- **`configure`** - вызывается при сохранении конфигурации администратором. Если `OnConfigure` вернёт ошибку, конфигурация отклоняется.
- **`handle_event`** - основной вызов при срабатывании любого триггера.
- **`step_callback`** - вызывается для валидации пользовательского ввода, загрузки динамических опций и пагинации.
- **`migrate`** - вызывается при обновлении версии плагина для выполнения миграций.

## Функция Run()

```go
func main() {
    wasmplugin.Run(myPlugin)
}
```

`Run()` читает `PLUGIN_ACTION` из окружения и вызывает соответствующий внутренний обработчик. Работать с протоколом вручную не нужно - достаточно заполнить структуру `Plugin` и предоставить функции-обработчики.

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

### Зависимость от другого плагина

```go
wasmplugin.PluginDep("auth-plugin", "Проверка токенов авторизации").Build(),
```

### Требование с конфигурацией

Метод `.WithConfig()` позволяет привязать конфигурацию к требованию:

```go
wasmplugin.HTTP("Запросы к платёжной системе").
    WithConfig(wasmplugin.ConfigFields(
        wasmplugin.String("api_url", "URL платёжного API").Required(),
        wasmplugin.String("api_key", "API-ключ").Required().Sensitive(),
    )).
    Build(),
```

### Таблица конструкторов

| Конструктор | Описание |
|---|---|
| `Database(desc)` | Доступ к базе данных |
| `HTTP(desc)` | Исходящие HTTP-запросы |
| `KV(desc)` | Key-Value хранилище |
| `NotifyReq(desc)` | Отправка уведомлений |
| `EventsReq(desc)` | Публикация событий |
| `PluginDep(target, desc)` | Вызов другого плагина |

Каждый конструктор возвращает builder с методами `.WithConfig(cs)` и `.Build()`.
