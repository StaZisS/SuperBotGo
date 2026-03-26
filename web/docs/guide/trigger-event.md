# Event Bus

Плагины обмениваются данными через шину событий (pub/sub). Один плагин публикует событие в топик, другие подписываются и получают данные.

## Подписка на события

```go
wasmplugin.Trigger{
    Name:  "on_order",
    Type:  wasmplugin.TriggerEvent,
    Topic: "orders.created",
    Handler: func(ctx *wasmplugin.EventContext) error {
        topic := ctx.Event.Topic      // "orders.created"
        source := ctx.Event.Source     // ID плагина-отправителя
        payload := ctx.Event.Payload   // []byte с данными

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
| `Payload` | `[]byte` | Данные события (произвольный формат) |
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
        ctx.NotifyProject(1, "Новый заказ!", wasmplugin.PriorityNormal)
        return nil
    },
}
```
