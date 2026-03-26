# HTTP

HTTP-триггер регистрирует эндпоинт, доступный извне. Используется для интеграций с внешними системами, API и приёма событий.

## Регистрация

```go
wasmplugin.Trigger{
    Name:        "incoming",
    Type:        wasmplugin.TriggerHTTP,
    Description: "Входящие запросы",
    Path:        "/api/my-plugin/incoming",
    Methods:     []string{"POST"},
    Handler: func(ctx *wasmplugin.EventContext) error {
        body := ctx.HTTP.Body
        token := ctx.HTTP.Query["token"]

        if token != ctx.Config("secret_token", "") {
            ctx.JSON(401, map[string]string{"error": "unauthorized"})
            return nil
        }

        // Обработка данных...
        ctx.JSON(200, map[string]string{"status": "ok"})
        return nil
    },
}
```

## URL вызова

Внешние системы обращаются к эндпоинту по адресу:

```
https://<host>/plugins/<plugin-id><path>
```

Например, для плагина `my-plugin` с `Path: "/api/my-plugin/incoming"`:

```
https://bot.example.com/plugins/my-plugin/api/my-plugin/incoming
```

## Поля ctx.HTTP

| Поле | Тип | Описание |
|---|---|---|
| `Method` | `string` | HTTP-метод (`GET`, `POST`, `PUT`, ...) |
| `Path` | `string` | Путь запроса |
| `Query` | `map[string]string` | Query-параметры |
| `Headers` | `map[string]string` | Заголовки запроса |
| `Body` | `string` | Тело запроса |
| `RemoteAddr` | `string` | IP-адрес клиента |

## Методы ответа

**`ctx.JSON(statusCode, value)`** - сериализует значение в JSON и отправляет с заголовком `Content-Type: application/json`:

```go
ctx.JSON(200, map[string]string{"result": "ok"})
```

**`ctx.SetHTTPResponse(statusCode, headers, body)`** - произвольный ответ с кастомными заголовками:

```go
ctx.SetHTTPResponse(200, map[string]string{
    "Content-Type": "text/plain",
    "X-Custom":     "value",
}, "OK")
```

## Несколько методов

Один триггер может обрабатывать несколько HTTP-методов:

```go
wasmplugin.Trigger{
    Name:    "items",
    Type:    wasmplugin.TriggerHTTP,
    Path:    "/api/my-plugin/items",
    Methods: []string{"GET", "POST", "DELETE"},
    Handler: func(ctx *wasmplugin.EventContext) error {
        switch ctx.HTTP.Method {
        case "GET":
            ctx.JSON(200, getItems())
        case "POST":
            createItem(ctx.HTTP.Body)
            ctx.JSON(201, map[string]string{"status": "created"})
        case "DELETE":
            deleteItem(ctx.HTTP.Query["id"])
            ctx.JSON(200, map[string]string{"status": "deleted"})
        }
        return nil
    },
}
```
