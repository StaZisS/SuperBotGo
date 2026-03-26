# KV Store

Key-value хранилище с изоляцией по плагинам. Данные сохраняются между вызовами. У каждого плагина **изолированное пространство ключей** - коллизии между плагинами невозможны.

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
// keys: ["user:1", "user:2", "user:admin"]
```

## Поддержка TTL

Установка автоматического времени жизни ключа:

```go
err := ctx.KVSetWithTTL("session", data, 30*time.Minute)
```

После истечения TTL ключ автоматически удаляется. Повторный `KVGet` вернёт `found == false`.

## Справочник API

| Метод | Сигнатура | Описание |
|---|---|---|
| `KVGet` | `(key string) (string, bool, error)` | Получить значение. `bool` - найден ли ключ |
| `KVSet` | `(key, value string) error` | Установить значение (без TTL) |
| `KVSetWithTTL` | `(key, value string, ttl time.Duration) error` | Установить значение с TTL |
| `KVDelete` | `(key string) error` | Удалить ключ |
| `KVList` | `(prefix string) ([]string, error)` | Список ключей по префиксу |

**Необходимое требование:**

```go
wasmplugin.KV("Кеширование данных")
```

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

## Пример: счётчик вызовов

```go
Handler: func(ctx *wasmplugin.EventContext) error {
    val, found, err := ctx.KVGet("call_count")
    if err != nil {
        return err
    }

    count := 0
    if found {
        count, _ = strconv.Atoi(val)
    }
    count++

    if err := ctx.KVSet("call_count", strconv.Itoa(count)); err != nil {
        return err
    }

    ctx.Reply(fmt.Sprintf("Вызов #%d", count))
    return nil
}
```

::: warning Значения - строки
Значения KV - строки. Сериализуйте сложные данные в JSON перед сохранением.
:::
