# Host API

Плагины вызывают функции хоста через импортированные WASM-функции. Каждый вызов требует соответствующих [разрешений](/advanced/build#permissions).

## База данных

### Запрос

```go
results, err := wasmplugin.DBQuery(map[string]interface{}{
    "collection": "users",
    "filter":     map[string]interface{}{"active": true},
    "limit":      10,
})
// results: []map[string]interface{}
```

### Сохранение

```go
err := wasmplugin.DBSave(map[string]interface{}{
    "collection": "logs",
    "record": map[string]interface{}{
        "event": "login",
        "user":  "alice",
    },
})
```

**Необходимые разрешения:** `db:read`, `db:write`

## HTTP-запросы

### GET

```go
resp, err := wasmplugin.HTTPGet("https://api.example.com/data")
fmt.Println(resp.StatusCode, resp.Body)
```

### POST

```go
resp, err := wasmplugin.HTTPPost(
    "https://api.example.com/submit",
    "application/json",
    `{"key": "value"}`,
)
```

### Произвольный запрос

```go
resp, err := wasmplugin.HTTPRequest(
    "PUT",
    "https://api.example.com/item/1",
    map[string]string{"Authorization": "Bearer token"},
    `{"name": "updated"}`,
)
```

### Структура HTTPResponse

| Поле | Тип | Описание |
|---|---|---|
| `StatusCode` | `int` | HTTP-статус |
| `Headers` | `map[string]string` | Заголовки ответа |
| `Body` | `string` | Тело ответа |

**Необходимое разрешение:** `network:read`

## Вызов другого плагина {#call-plugin}

Вызов метода другого плагина:

```go
result, err := wasmplugin.CallPlugin(
    "other-plugin",   // ID целевого плагина
    "getData",        // метод
    map[string]interface{}{"key": "value"},  // параметры
)
// result: []byte (сырой ответ)
```

**Необходимое разрешение:** `plugins:events`

## Публикация событий {#publish-events}

Публикация событий, на которые могут подписаться другие плагины через [Event-триггеры](/guide/triggers#event-bus-триггер):

```go
err := wasmplugin.PublishEvent("orders.created", map[string]interface{}{
    "order_id": 12345,
    "amount":   99.99,
})
```

**Необходимое разрешение:** `plugins:events`

## Уведомления {#notifications}

Отправка приоритетных уведомлений с учётом предпочтений пользователя (рабочие часы, канал доставки, упоминания).

### Уведомление пользователю

```go
err := ctx.NotifyUser(userID, "Сборка завершена", wasmplugin.PriorityNormal)
```

### Уведомление в чат

```go
err := ctx.NotifyChat("telegram", "123456789", "Новый заказ!", wasmplugin.PriorityHigh)
```

### Уведомление в проект

```go
err := ctx.NotifyProject(42, "Релиз опубликован", wasmplugin.PriorityCritical)
```

**Необходимые разрешения:** `notify:user`, `notify:chat`, `notify:project`

Подробное описание приоритетов и поведения: [Уведомления](/api/notifications).

## Оптимизация MessagePack {#msgpack}

По умолчанию Host API использует JSON. Переключитесь на MessagePack для лучшей производительности на горячих путях:

```go
func init() {
    wasmplugin.UseMessagePack()
}
```

Хост автоматически определяет кодировку по первому байту и отвечает в том же формате. Вернуться к JSON: `wasmplugin.UseJSON()`.

::: tip Когда использовать MessagePack
Используйте для плагинов с большим количеством вызовов хоста за одно выполнение (например, пакетные запросы к БД). Для простых плагинов JSON достаточно.
:::
