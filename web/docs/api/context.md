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
| `ctx.Reply(text)` | Ответить в текущий чат (только для messenger-триггеров) |
| `ctx.SendMessage(chatID, text)` | Отправить в произвольный чат |

#### `ctx.Reply(text)`

Устанавливает текстовый ответ для текущего чата. Работает **только** при `TriggerType == "messenger"` — для cron, HTTP и event-триггеров вызов будет проигнорирован.

```go
ctx.Reply("Готово!")
```

#### `ctx.SendMessage(chatID, text)`

Отправляет сообщение в произвольный чат. Если вызывается из messenger-триггера, тип канала определяется автоматически по каналу триггера. Для отправки из cron/event-триггеров канал также должен быть определён (через контекст вызвавшего триггера).

```go
// Отправить уведомление в конкретный чат
ctx.SendMessage("123456789", "Сборка завершена!")
```

#### Поведение доставки

Хост применяет следующие правила при отправке сообщений:

| Правило | Описание |
|---|---|
| **Повтор при ошибках** | Транзиентные ошибки (429, таймаут, сетевые сбои) повторяются до 3 раз с экспоненциальной задержкой (500 мс → 1 с → 2 с) и jitter |
| **Валидация** | Пустые сообщения отклоняются с ошибкой |
| **Обрезка длинных сообщений** | Telegram: до 4096 символов, Discord: до 2000 символов. Излишек обрезается с добавлением `...` |
| **Определение канала** | `SendMessage` без явного канала использует канал текущего триггера |

::: warning Пустые сообщения
Вызов `ctx.Reply("")` или `ctx.SendMessage(chatID, "")` не отправит сообщение. Убедитесь, что текст не пуст.
:::

### Уведомления

| Метод | Описание |
|---|---|
| `ctx.NotifyUser(userID, text, priority)` | Уведомление пользователю с учётом предпочтений |
| `ctx.NotifyChat(channelType, chatID, text, priority)` | Уведомление в конкретный чат |
| `ctx.NotifyProject(projectID, text, priority)` | Уведомление во все чаты проекта |

Подробнее: [Уведомления](/api/notifications)

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
