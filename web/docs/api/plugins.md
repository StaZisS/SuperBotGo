# Межплагинное взаимодействие

## Typed RPC

Межплагинные вызовы идут через отдельный RPC runtime path. Плагин явно публикует методы в `RPCMethods`, а вызывающая сторона использует `CallPlugin`, `CallPluginInto` или `CallPluginRaw`.

## Экспорт RPC-метода

```go
type ResolveRequest struct {
    ExternalID string `msgpack:"external_id"`
}

type ResolveResponse struct {
    UserID int64 `msgpack:"user_id"`
}

RPCMethods: []wasmplugin.RPCMethod{
    {
        Name:        "resolve_user",
        Description: "Resolve external user ID",
        Handler: func(ctx *wasmplugin.RPCContext) ([]byte, error) {
            var req ResolveRequest
            if err := ctx.Decode(&req); err != nil {
                return nil, err
            }
            return wasmplugin.MarshalRPC(ResolveResponse{UserID: 42})
        },
    },
},
```

## Вызов другого плагина

```go
var resp ResolveResponse
err := wasmplugin.CallPluginInto(
    "users-plugin",   // ID целевого плагина
    "resolve_user",   // имя RPC-метода
    ResolveRequest{ExternalID: "ext-123"},
    &resp,
)
```

Если нужен raw msgpack result:

```go
raw, err := wasmplugin.CallPlugin("users-plugin", "resolve_user", ResolveRequest{ExternalID: "ext-123"})
```

### Требование

```go
wasmplugin.PluginDep("users-plugin", "Разрешение внешнего ID").Build()
```

::: info Guardrails
Host проверяет:

- наличие `PluginDep(target, ...)`
- что целевой плагин реально загружен
- что метод опубликован в `RPCMethods`
- защиту от циклов и ограничение глубины вызовов
:::

## Публикация событий

Публикация событий, на которые могут подписаться другие плагины через [Event-триггеры](/guide/trigger-event):

```go
err := wasmplugin.PublishEvent("orders.created", map[string]interface{}{
    "order_id": 12345,
    "amount":   99.99,
})
```

`PublishEvent()` сериализует payload в JSON. Если payload уже сериализован, используйте `PublishEventRawJSON()`.

### Требование

```go
wasmplugin.EventsReq("Публикация событий заказов").Build()
```
