# KV Store

Key-value хранилище с изоляцией по плагинам. Данные сохраняются между вызовами. У каждого плагина **изолированное пространство ключей** — коллизии между плагинами невозможны.

## Базовые операции

```go
// Запись
err := ctx.KVSet("counter", "42")

// Чтение
value, found, err := ctx.KVGet("counter")
if found {
    fmt.Println(value) // "42"
}

// Удаление
err := ctx.KVDelete("counter")

// Список ключей по префиксу
keys, err := ctx.KVList("user:")
```

## Поддержка TTL

Установка автоматического времени жизни ключа:

```go
err := ctx.KVSetWithTTL("session", data, 30*time.Minute)
```

После истечения TTL ключ автоматически удаляется.

## Справочник API

| Метод | Сигнатура | Разрешение |
|---|---|---|
| `KVGet` | `(key string) (string, bool, error)` | `kv:read` |
| `KVSet` | `(key, value string) error` | `kv:write` |
| `KVSetWithTTL` | `(key, value string, ttl time.Duration) error` | `kv:write` |
| `KVDelete` | `(key string) error` | `kv:write` |
| `KVList` | `(prefix string) ([]string, error)` | `kv:read` |

## KV в миграциях

`MigrateContext` также предоставляет KV-методы для трансформации данных при обновлении версии:

```go
Migrate: func(ctx *wasmplugin.MigrateContext) error {
    val, found, _ := ctx.KVGet("old_key")
    if found {
        ctx.KVSet("new_key", val)
        ctx.KVDelete("old_key")
    }
    return nil
},
```

Подробнее — в разделе [Миграции](/advanced/migrations).

::: warning
Значения KV — строки. Сериализуйте сложные данные в JSON перед сохранением.
:::
