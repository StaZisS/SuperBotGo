# HTTP-клиент

Исходящие HTTP-запросы к внешним сервисам.

`HTTPRequest()` использует HTTP requirement с именем `default`. Если у плагина несколько HTTP requirements, используйте `HTTPRequestFor(name, ...)`.

## GET

```go
resp, err := wasmplugin.HTTPGet("https://api.example.com/data")
if err != nil {
    return err
}
fmt.Println(resp.StatusCode, resp.Body)
```

## POST

```go
resp, err := wasmplugin.HTTPPost(
    "https://api.example.com/submit",
    "application/json",
    `{"key": "value"}`,
)
```

## Произвольный запрос

```go
resp, err := wasmplugin.HTTPRequest(
    "PUT",
    "https://api.example.com/item/1",
    map[string]string{"Authorization": "Bearer token"},
    `{"name": "updated"}`,
)
```

## Именованный HTTP requirement

```go
resp, err := wasmplugin.HTTPRequestFor(
    "github",
    "GET",
    "https://api.github.com/repos/owner/repo",
    map[string]string{"Accept": "application/vnd.github+json"},
    "",
)
```

## Структура HTTPResponse

| Поле | Тип | Описание |
|---|---|---|
| `StatusCode` | `int` | HTTP-статус |
| `Headers` | `map[string]string` | Заголовки ответа |
| `Body` | `string` | Тело ответа |

## Требование

```go
wasmplugin.HTTP("Запросы к внешнему API").Build()
```

## Host-side policy enforcement

Для `http` requirement можно задать policy, которую host применяет до выполнения запроса:

```go
wasmplugin.HTTP("Запросы к GitHub API").
    Name("github").
    WithConfig(wasmplugin.HTTPPolicyConfig()).
    Build()
```

Пример config:

```json
{
  "requirements": {
    "http": {
      "github": {
        "allowed_hosts": ["api.github.com"],
        "allowed_methods": ["GET"],
        "max_request_body_bytes": 0,
        "max_response_body_bytes": 1048576
      }
    }
  }
}
```

Поддерживаемые поля:

- `allowed_hosts`
- `allowed_methods`
- `max_request_body_bytes`
- `max_response_body_bytes`

Дополнительно host всегда блокирует loopback, private IP ranges и cloud metadata endpoints для SSRF-защиты.
```
