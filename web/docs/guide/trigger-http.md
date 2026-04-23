# HTTP

HTTP-триггер регистрирует endpoint внутри host-системы. Он подходит как для frontend/API-сценариев, так и для server-to-server интеграций.

## Регистрация

```go
wasmplugin.Trigger{
    Name:        "profile",
    Type:        wasmplugin.TriggerHTTP,
    Description: "Профиль пользователя",
    Path:        "/profile",
    Methods:     []string{"GET"},
    Handler: func(ctx *wasmplugin.EventContext) error {
        if ctx.HTTP.Auth == nil || ctx.HTTP.Auth.Kind != "user" {
            ctx.JSON(401, map[string]string{"error": "authentication required"})
            return nil
        }

        ctx.JSON(200, map[string]any{
            "user_id": ctx.HTTP.Auth.UserID,
            "method":  ctx.HTTP.Method,
        })
        return nil
    },
}
```

## URL вызова

HTTP-триггеры вызываются по адресу:

```text
https://<host>/api/triggers/http/<plugin-id><path>
```

Например, для плагина `my-plugin` с `Path: "/profile"`:

```text
https://bot.example.com/api/triggers/http/my-plugin/profile
```

## Авторизация и доступ

Host проверяет доступ **до вызова плагина**.

Для HTTP-триггера доступны два типа principal:

- `user` — пользователь host-системы, пришедший через `user_session` cookie или bearer user-token
- `service` — bearer `service-key` для server-to-server вызовов

Настройки доступа задаются в админке на странице прав trigger'ов:

- `enabled`
- `allow user session`
- `allow service key`
- `policy expression`

### Frontend-сценарий

Для фронтенда рекомендуется использовать browser login через TSU и cookie-сессию host-системы:

1. Браузер уходит на `GET /api/auth/tsu/start?return_to=/app`
2. После callback host ставит `user_session`
3. Frontend вызывает HTTP-trigger endpoint'ы с `credentials: 'include'`

Пример:

```ts
await fetch('/api/triggers/http/my-plugin/profile', {
  method: 'GET',
  credentials: 'include',
})
```

Если пользователь открывает защищённый HTML-trigger обычной навигацией браузера и ещё не вошёл в систему, host делает redirect на:

```text
/api/auth/tsu/start?return_to=<текущий-path-and-query>
```

Для обычных API/fetch-запросов поведение не меняется: host возвращает `401`.

### User bearer token для API-клиентов

Если запросы идут не из браузера, пользователь может сначала получить token из host-сессии:

```http
POST /api/auth/tokens
Content-Type: application/json
Cookie: user_session=...

{"name":"CLI token"}
```

Дальше HTTP-trigger вызывается так:

```http
Authorization: Bearer sbuk_<public>.<secret>
```

Такой bearer token даёт тот же principal `user`, что и cookie-сессия. Дальше host применяет те же `policy expression` и те же права из админки.

### Service-to-service сценарий

Для внешних систем используйте bearer `service-key`:

```http
Authorization: Bearer sbsk_<public>.<secret>
```

Такой ключ проверяется host-системой и должен иметь scope на конкретный `plugin_id + trigger_name`.

## Поля ctx.HTTP

| Поле | Тип | Описание |
|---|---|---|
| `Method` | `string` | HTTP-метод (`GET`, `POST`, `PUT`, ...) |
| `Path` | `string` | Путь запроса |
| `Query` | `map[string]string` | Query-параметры |
| `Headers` | `map[string]string` | Заголовки запроса |
| `Body` | `string` | Тело запроса |
| `RemoteAddr` | `string` | IP-адрес клиента |
| `Auth` | `*HTTPAuthInfo` | Principal, прошедший host-auth |

### ctx.HTTP.Auth

Если запрос аутентифицирован, host передаёт auth-контекст в плагин:

| Поле | Тип | Описание |
|---|---|---|
| `Kind` | `string` | `"user"` или `"service"` |
| `UserID` | `int64` | ID пользователя для запросов с cookie-сессией или user bearer token |
| `ServiceKeyID` | `int64` | ID service-key для server-to-server запросов |

Пример:

```go
switch {
case ctx.HTTP.Auth == nil:
    ctx.JSON(401, map[string]string{"error": "authentication required"})
case ctx.HTTP.Auth.Kind == "user":
    ctx.JSON(200, map[string]any{"user_id": ctx.HTTP.Auth.UserID})
case ctx.HTTP.Auth.Kind == "service":
    ctx.JSON(200, map[string]any{"service_key_id": ctx.HTTP.Auth.ServiceKeyID})
}
```

## Методы ответа

**`ctx.JSON(statusCode, value)`** — сериализует значение в JSON и отправляет с заголовком `Content-Type: application/json`:

```go
ctx.JSON(200, map[string]string{"result": "ok"})
```

**`ctx.SetHTTPResponse(statusCode, headers, body)`** — произвольный ответ с кастомными заголовками:

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
    Path:    "/items",
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
