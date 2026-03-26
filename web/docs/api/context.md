# EventContext

`EventContext` - единый контекст, передаваемый во все обработчики событий: команды, HTTP, cron и event bus триггеры.

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
| `ChannelType` | `string` | Тип канала (`"telegram"`, `"discord"`, ...) |
| `ChatID` | `string` | ID чата |
| `CommandName` | `string` | Имя вызванной команды |
| `Params` | `map[string]string` | Собранные параметры |
| `Locale` | `string` | Локаль пользователя |

### HTTPEventData

| Поле | Тип | Описание |
|---|---|---|
| `Method` | `string` | HTTP-метод (GET, POST, ...) |
| `Path` | `string` | Путь запроса |
| `Query` | `map[string]string` | Query-параметры |
| `Headers` | `map[string]string` | Заголовки запроса |
| `Body` | `string` | Тело запроса |
| `RemoteAddr` | `string` | IP-адрес клиента |

### CronEventData

| Поле | Тип | Описание |
|---|---|---|
| `ScheduleName` | `string` | Имя расписания (= `Name` триггера) |
| `FireTime` | `int64` | Unix timestamp срабатывания |

### EventBusData

| Поле | Тип | Описание |
|---|---|---|
| `Topic` | `string` | Топик события |
| `Payload` | `[]byte` | Данные события |
| `Source` | `string` | ID плагина-отправителя |

## Методы

### Ответы в чат {#reply}

#### `ctx.Reply(text string)`

Устанавливает текстовый ответ для текущего чата. Работает **только** при `TriggerType == "messenger"` - для cron, HTTP и event-триггеров вызов будет проигнорирован.

```go
ctx.Reply("Готово!")
```

#### `ctx.ReplyLocalized(texts map[string]string)`

Отвечает локализованным сообщением. Хост выбирает текст по локали пользователя. Ключи карты - коды локалей (`"ru"`, `"en"`, ...).

```go
ctx.ReplyLocalized(map[string]string{
    "ru": "Задача выполнена!",
    "en": "Task completed!",
})
```

::: tip Catalog
Для работы с переводами удобнее использовать [Catalog](/api/localization):
```go
ctx.ReplyLocalized(catalog.L("task_done"))
```
:::

### Отправка сообщений {#send}

#### `ctx.SendMessage(chatID string, text string)`

Отправляет сообщение в произвольный чат. Тип канала определяется автоматически по каналу текущего триггера.

```go
ctx.SendMessage("123456789", "Сборка завершена!")
```

#### `ctx.SendLocalizedMessage(chatID string, texts map[string]string)`

Отправляет локализованное сообщение в указанный чат. Хост выбирает текст по локали чата.

```go
ctx.SendLocalizedMessage("123456789", map[string]string{
    "ru": "Новый заказ!",
    "en": "New order!",
})
```

#### `ctx.SendLocalizedToUser(userID int64, texts map[string]string)`

Отправляет локализованное сообщение конкретному пользователю. Хост определяет предпочтительный канал и локаль пользователя.

```go
ctx.SendLocalizedToUser(42, catalog.L("welcome"))
```

#### Поведение доставки

Хост применяет следующие правила при отправке сообщений:

| Правило | Описание |
|---|---|
| **Повтор при ошибках** | Транзиентные ошибки (429, таймаут, сетевые сбои) повторяются до 3 раз с экспоненциальной задержкой (500 мс - 1 с - 2 с) и jitter |
| **Валидация** | Пустые сообщения отклоняются с ошибкой |
| **Обрезка длинных сообщений** | Длинные сообщения обрезаются по лимиту платформы с добавлением `...` |
| **Определение канала** | `SendMessage` без явного канала использует канал текущего триггера |

::: warning Пустые сообщения
Вызов `ctx.Reply("")` или `ctx.SendMessage(chatID, "")` не отправит сообщение. Убедитесь, что текст не пуст.
:::

### Уведомления {#notifications}

| Метод | Описание |
|---|---|
| `ctx.NotifyUser(userID, text, priority)` | Уведомление пользователю с учётом предпочтений |
| `ctx.NotifyChat(channelType, chatID, text, priority)` | Уведомление в конкретный чат |
| `ctx.NotifyProject(projectID, text, priority)` | Уведомление во все чаты проекта |

Подробнее: [Уведомления](/api/notifications)

### HTTP-ответы {#http-response}

#### `ctx.SetHTTPResponse(statusCode int, headers map[string]string, body string)`

Устанавливает произвольный HTTP-ответ с кастомными заголовками. Работает только в HTTP-триггерах.

```go
ctx.SetHTTPResponse(200, map[string]string{
    "Content-Type": "text/plain",
}, "OK")
```

#### `ctx.JSON(statusCode int, v interface{})`

Сериализует значение в JSON и устанавливает его как HTTP-ответ. Заголовок `Content-Type: application/json` добавляется автоматически.

```go
ctx.JSON(200, map[string]string{"status": "ok"})
```

### Логирование {#logging}

| Метод | Описание |
|---|---|
| `ctx.Log(msg)` | Info-лог |
| `ctx.LogError(msg)` | Error-лог |

```go
ctx.Log("обработка завершена")
ctx.LogError("не удалось подключиться к API")
```

### Доступ к данным {#data}

#### `ctx.Config(key string, fallback string) string`

Получает значение конфигурации плагина. Если ключ не установлен, возвращается `fallback`.

```go
apiURL := ctx.Config("api_url", "https://api.example.com")
```

#### `ctx.Param(key string) string`

Получает параметр команды. Shortcut для `ctx.Messenger.Params[key]`.

```go
name := ctx.Param("name")
```

#### `ctx.Locale() string`

Возвращает локаль пользователя (например `"ru"`, `"en"`). По умолчанию `"en"`.

```go
locale := ctx.Locale()
```

## Определение типа события

```go
Handler: func(ctx *wasmplugin.EventContext) error {
    switch ctx.TriggerType {
    case wasmplugin.TriggerMessenger:
        // ctx.Messenger != nil
        ctx.Reply("Привет, " + ctx.Param("name"))

    case wasmplugin.TriggerHTTP:
        // ctx.HTTP != nil
        ctx.JSON(200, map[string]string{"method": ctx.HTTP.Method})

    case wasmplugin.TriggerCron:
        // ctx.Cron != nil
        ctx.Log("cron сработал: " + ctx.Cron.ScheduleName)

    case wasmplugin.TriggerEvent:
        // ctx.Event != nil
        ctx.Log("событие из " + ctx.Event.Source + ": " + ctx.Event.Topic)
    }
    return nil
}
```
