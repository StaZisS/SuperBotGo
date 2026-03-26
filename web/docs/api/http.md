# HTTP-клиент

Исходящие HTTP-запросы к внешним сервисам.

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

## Структура HTTPResponse

| Поле | Тип | Описание |
|---|---|---|
| `StatusCode` | `int` | HTTP-статус |
| `Headers` | `map[string]string` | Заголовки ответа |
| `Body` | `string` | Тело ответа |

## Требование

```go
wasmplugin.HTTP("Запросы к внешнему API")
```
