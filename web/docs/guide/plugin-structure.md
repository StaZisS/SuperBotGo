# Структура плагина

Плагин — это один бинарный `.wasm` файл. Вся логика описывается структурой `Plugin` и передаётся в `wasmplugin.Run()`.

## Структура Plugin

```go
wasmplugin.Plugin{
    ID:          "my-plugin",         // уникальный идентификатор
    Name:        "My Plugin",         // отображаемое имя
    Version:     "1.0.0",             // семантическая версия
    Config:      configSchema,        // схема конфигурации (опционально)
    Permissions: []Permission{...},   // запрашиваемые разрешения
    Commands:    []Command{...},      // команды мессенджера
    Triggers:    []Trigger{...},      // HTTP / Cron / Event триггеры
    OnEvent:     fallbackHandler,     // обработчик по умолчанию
    OnConfigure: configHandler,       // вызывается при конфигурации
    Migrate:     migrateHandler,      // вызывается при обновлении версии
}
```

## Жизненный цикл

Плагин — это **одноразовый процесс**. Каждый вызов:

1. Хост создаёт **новый экземпляр** WASM-модуля
2. Передаёт `PLUGIN_ACTION` через env, данные через stdin
3. Плагин пишет результат в stdout (JSON)
4. Экземпляр **уничтожается**

::: warning Нет общего состояния между вызовами
Каждое выполнение получает чистое окружение. Используйте [KV Store](/api/kv-store) для хранения данных между вызовами.
:::

## Протокол (actions)

Хост взаимодействует с плагином через переменную окружения `PLUGIN_ACTION`:

| Action | Когда вызывается | Stdin | Stdout |
|---|---|---|---|
| `meta` | Загрузка плагина | — | PluginMeta JSON |
| `configure` | Установка/обновление конфига | Config JSON | Error JSON (опц.) |
| `handle_event` | Обработка события | Event JSON | EventResponse JSON |
| `step_callback` | Валидация/пагинация шагов | Callback JSON | CallbackResponse JSON |
| `migrate` | Обновление версии | MigrateRequest JSON | MigrateResponse JSON |

## Как работает `Run()`

```go
func main() {
    wasmplugin.Run(myPlugin) // диспетчеризация по PLUGIN_ACTION
}
```

`Run()` читает `PLUGIN_ACTION` из окружения и вызывает соответствующий внутренний обработчик. Работать с протоколом вручную не нужно — достаточно заполнить структуру `Plugin` и предоставить функции-обработчики.

## Разрешения

Плагины объявляют необходимые разрешения. Каждый вызов Host API проверяет разрешения в рантайме.

```go
Permissions: []wasmplugin.Permission{
    {Key: "db:read", Description: "Чтение данных пользователей", Required: true},
    {Key: "network:read", Description: "Запросы к внешнему API", Required: false},
},
```

Полная таблица разрешений — в разделе [Сборка и деплой](/advanced/build#permissions).
