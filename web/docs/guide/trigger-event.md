# Event Bus

Плагины обмениваются данными через шину событий (pub/sub). Один плагин публикует событие в топик, другие подписываются и получают данные.

Payload события в текущем контракте всегда хранится как JSON. `PublishEvent()` маршалит значение в JSON автоматически, а подписчик получает этот JSON в `ctx.Event.Payload`.

## Подписка на события

```go
wasmplugin.Trigger{
    Name:  "on_order",
    Type:  wasmplugin.TriggerEvent,
    Topic: "orders.created",
    Handler: func(ctx *wasmplugin.EventContext) error {
        topic := ctx.Event.Topic       // "orders.created"
        source := ctx.Event.Source     // ID плагина-отправителя

        var payload struct {
            OrderID int64 `json:"order_id"`
        }
        if err := json.Unmarshal(ctx.Event.Payload, &payload); err != nil {
            return err
        }

        ctx.Log("Получен заказ от " + source)
        return nil
    },
}
```

## Публикация событий

Из любого обработчика можно опубликовать событие через Host API:

```go
err := wasmplugin.PublishEvent("orders.created", map[string]interface{}{
    "order_id": 12345,
    "amount":   99.99,
})
```

Если payload уже сериализован, можно использовать:

```go
err := wasmplugin.PublishEventRawJSON("orders.created", rawJSON)
```

Для публикации требуется объявить требование `EventsReq`:

```go
Requirements: []wasmplugin.Requirement{
    wasmplugin.EventsReq("Публикация событий заказов").Build(),
},
```

## Поля ctx.Event

| Поле | Тип | Описание |
|---|---|---|
| `Topic` | `string` | Топик события |
| `Payload` | `[]byte` | JSON payload события |
| `Source` | `string` | ID плагина, опубликовавшего событие |

## Пример: связка двух плагинов

Плагин `orders` публикует событие при создании заказа:

```go
// В обработчике плагина orders
wasmplugin.PublishEvent("orders.created", map[string]interface{}{
    "order_id": orderID,
    "user_id":  userID,
})
```

Плагин `notifications` подписывается и отправляет уведомление:

```go
// В плагине notifications
wasmplugin.Trigger{
    Name:  "on_new_order",
    Type:  wasmplugin.TriggerEvent,
    Topic: "orders.created",
    Handler: func(ctx *wasmplugin.EventContext) error {
        return ctx.NotifyStudents().
            Stream(streamID).
            Message(wasmplugin.NewMessage("Новый заказ!")).
            Send()
    },
}
```

## Гарантии доставки

- Доставка at-least-once. Обработчики должны быть идемпотентными.
- `memory` backend работает внутри одного процесса.
- `postgres` backend даёт cluster-wide durable delivery, retry и DLQ.

Если включён backend PostgreSQL, событие сначала попадает в очередь `wasm_event_queue`, затем доставляется подписчикам worker-процессом.
