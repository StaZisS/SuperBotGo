# Межплагинное взаимодействие

## Вызов другого плагина

Вызов метода другого плагина. Параметры сериализуются автоматически, результат возвращается как `[]byte`.

```go
result, err := wasmplugin.CallPlugin(
    "other-plugin",   // ID целевого плагина
    "getData",        // метод
    map[string]interface{}{"key": "value"},
)
// result: []byte (сырой ответ)
```

### Требование

```go
wasmplugin.PluginDep("other-plugin", "Получение данных из другого плагина")
```

::: info Разрешение plugin:\<target\>
Каждый `PluginDep` создаёт отдельное разрешение `plugin:other-plugin`. Хост проверяет его при каждом вызове `CallPlugin`.
:::

## Публикация событий

Публикация событий, на которые могут подписаться другие плагины через [Event-триггеры](/guide/trigger-event):

```go
err := wasmplugin.PublishEvent("orders.created", map[string]interface{}{
    "order_id": 12345,
    "amount":   99.99,
})
```

### Требование

```go
wasmplugin.EventsReq("Публикация событий заказов")
```
