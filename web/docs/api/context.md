# EventContext

`EventContext` — единый контекст, передаваемый во все обработчики событий: команды, HTTP, cron и event bus триггеры.

## Общие поля

| Поле | Тип | Описание |
|---|---|---|
| `PluginID` | `string` | ID текущего плагина |
| `TriggerType` | `string` | `"messenger"`, `"http"`, `"cron"`, `"event"` |
| `TriggerName` | `string` | Имя конкретного триггера/команды |
| `Timestamp` | `int64` | Unix timestamp события |

## Данные по типу триггера

В зависимости от типа триггера одно из этих полей не nil:

| Поле | Тип | Не nil когда |
|---|---|---|
| `Messenger` | `*MessengerData` | Команды мессенджера |
| `HTTP` | `*HTTPEventData` | HTTP-триггеры |
| `Cron` | `*CronEventData` | Cron-триггеры |
| `Event` | `*EventBusData` | Event bus-триггеры |

### MessengerData

| Поле | Тип | Описание |
|---|---|---|
| `UserID` | `int64` | ID пользователя |
| `ChannelType` | `string` | Тип канала (telegram, discord, ...) |
| `ChatID` | `string` | ID чата |
| `CommandName` | `string` | Имя вызванной команды |
| `Params` | `map[string]string` | Собранные параметры |
| `Locale` | `string` | Локаль пользователя |

## Методы

### Сообщения

| Метод | Описание |
|---|---|
| `ctx.Reply(text)` | Ответить в текущий чат (мессенджер) |
| `ctx.SendMessage(chatID, text)` | Отправить в произвольный чат |

### HTTP-ответы

| Метод | Описание |
|---|---|
| `ctx.JSON(code, value)` | JSON-ответ (HTTP-триггеры) |
| `ctx.SetHTTPResponse(code, headers, body)` | Произвольный HTTP-ответ |

### Логирование

| Метод | Описание |
|---|---|
| `ctx.Log(msg)` | Info-лог |
| `ctx.LogError(msg)` | Error-лог |

### Доступ к данным

| Метод | Описание |
|---|---|
| `ctx.Config(key, fallback)` | Получить значение конфигурации |
| `ctx.Param(key)` | Получить параметр команды (shortcut для `ctx.Messenger.Params[key]`) |
| `ctx.Locale()` | Локаль пользователя (по умолчанию `"en"`) |

## Определение типа события

```go
Handler: func(ctx *wasmplugin.EventContext) error {
    switch ctx.TriggerType {
    case "messenger":
        // ctx.Messenger != nil
    case wasmplugin.TriggerHTTP:
        // ctx.HTTP != nil
    case wasmplugin.TriggerCron:
        // ctx.Cron != nil
    case wasmplugin.TriggerEvent:
        // ctx.Event != nil
    }
    return nil
}
```
