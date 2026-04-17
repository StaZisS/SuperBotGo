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
| `Files` | `[]FileRef` | Прикреплённые файлы (пусто, если сообщение без файлов) |

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
| `Payload` | `[]byte` | JSON payload события |
| `Source` | `string` | ID плагина-отправителя |

## Методы

### Ответы в чат {#reply}

#### `ctx.Reply(msg Message)`

Устанавливает ответ для текущего чата. Поддерживает rich content и встроенную локализацию. Работает **только** при `TriggerType == "messenger"`.

```go
// Простой текст
ctx.Reply(wasmplugin.NewMessage("Готово!"))

// Локализованное сообщение
ctx.Reply(wasmplugin.NewLocalizedMessage(catalog.L("task_done")))

// Rich content: текст + файл
ctx.Reply(wasmplugin.NewMessage("Вот расписание").File(ref, "schedule.pdf"))
```

### Файлы {#files}

| Метод | Описание |
|---|---|
| `ctx.HasFiles()` | Есть ли прикреплённые файлы |
| `ctx.Files()` | Список `[]FileRef` |
| `ctx.FileMeta(fileID)` | Метаданные файла |
| `ctx.FileReadAll(fileID)` | Прочитать файл целиком |
| `ctx.FileRead(fileID, offset, maxBytes)` | Чтение чанками |
| `ctx.FileStore(name, mime, type, data)` | Сохранить файл |

Подробнее: [Файлы](/api/files)

### Уведомления {#notifications}

| Метод | Описание |
|---|---|
| `ctx.NotifyUser(userID, text, priority)` | Уведомление пользователю с учётом предпочтений |
| `ctx.NotifyChat(channelType, chatID, text, priority)` | Уведомление в конкретный чат |
| `ctx.NotifyStudents().Scope(id).Message(t).Send()` | Уведомление студентам по университетской иерархии (builder) |

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
        ctx.Reply(wasmplugin.NewMessage("Привет, " + ctx.Param("name")))

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
