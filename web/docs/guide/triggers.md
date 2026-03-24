# Триггеры

Триггеры запускают плагин в ответ на внешние события. Плагин может зарегистрировать несколько триггеров разных типов.

## HTTP-триггер

Регистрация HTTP-эндпоинтов, которые хост маршрутизирует в плагин:

```go
wasmplugin.Trigger{
    Name:        "webhook",
    Type:        wasmplugin.TriggerHTTP,
    Description: "Входящий вебхук",
    Path:        "/api/my-plugin/webhook",
    Methods:     []string{"POST"},
    Handler: func(ctx *wasmplugin.EventContext) error {
        body   := ctx.HTTP.Body
        method := ctx.HTTP.Method
        token  := ctx.HTTP.Query["token"]
        auth   := ctx.HTTP.Headers["Authorization"]
        remote := ctx.HTTP.RemoteAddr

        ctx.JSON(200, map[string]string{"status": "ok"})
        return nil
    },
}
```

### Поля `ctx.HTTP`

| Поле | Тип | Описание |
|---|---|---|
| `Method` | `string` | HTTP-метод (GET, POST, ...) |
| `Path` | `string` | Путь запроса |
| `Query` | `map[string]string` | Query-параметры |
| `Headers` | `map[string]string` | Заголовки запроса |
| `Body` | `string` | Тело запроса |
| `RemoteAddr` | `string` | IP-адрес клиента |

### Методы ответа

- **`ctx.JSON(statusCode, value)`** — сериализовать в JSON и отправить
- **`ctx.SetHTTPResponse(statusCode, headers, body)`** — произвольный ответ с кастомными заголовками

## Cron-триггер

Выполнение по расписанию в стандартном cron-синтаксисе:

```go
wasmplugin.Trigger{
    Name:        "daily_report",
    Type:        wasmplugin.TriggerCron,
    Description: "Ежедневный отчёт в 9:00",
    Schedule:    "0 9 * * *",
    Handler: func(ctx *wasmplugin.EventContext) error {
        ctx.Log("cron: daily_report сработал")
        // ctx.Reply() не работает для cron — используйте ctx.SendMessage()
        ctx.SendMessage("CHAT_ID", "Ежедневный отчёт: ...")
        return nil
    },
}
```

### Поля `ctx.Cron`

| Поле | Тип | Описание |
|---|---|---|
| `ScheduleName` | `string` | Имя расписания (= `Name` триггера) |
| `FireTime` | `int64` | Unix timestamp срабатывания |

::: tip ctx.Reply() vs ctx.SendMessage()
`ctx.Reply()` работает **только** в messenger-триггерах — он отвечает в текущий чат пользователя. Для cron и event-триггеров используйте `ctx.SendMessage(chatID, text)` с явным указанием ID чата.
:::

## Event Bus-триггер

Подписка на межплагинные события:

```go
wasmplugin.Trigger{
    Name:  "on_order",
    Type:  wasmplugin.TriggerEvent,
    Topic: "orders.created",
    Handler: func(ctx *wasmplugin.EventContext) error {
        topic   := ctx.Event.Topic     // "orders.created"
        source  := ctx.Event.Source    // ID плагина-отправителя
        payload := ctx.Event.Payload   // []byte с данными
        // ...
        return nil
    },
}
```

### Поля `ctx.Event`

| Поле | Тип | Описание |
|---|---|---|
| `Topic` | `string` | Топик события |
| `Payload` | `[]byte` | Данные события |
| `Source` | `string` | ID плагина-отправителя |

::: info Публикация событий
Используйте `wasmplugin.PublishEvent()` для отправки событий. См. [Host API](/api/host-api#publish-events).
:::

## Сводная таблица типов

| Тип | Константа | Обязательные поля |
|---|---|---|
| HTTP | `wasmplugin.TriggerHTTP` | `Path`, `Methods` |
| Cron | `wasmplugin.TriggerCron` | `Schedule` |
| Event | `wasmplugin.TriggerEvent` | `Topic` |

## Fallback-обработчик

Если у триггера нет своего `Handler`, вызывается `Plugin.OnEvent`:

```go
wasmplugin.Plugin{
    // ...
    OnEvent: func(ctx *wasmplugin.EventContext) error {
        ctx.Log("необработанный триггер: " + ctx.TriggerName)
        return nil
    },
}
```
