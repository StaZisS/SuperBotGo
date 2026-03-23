# Миграции

При изменении версии плагина во время перезагрузки хост вызывает обработчик `Migrate` для трансформации данных.

## Базовая миграция

```go
Migrate: func(ctx *wasmplugin.MigrateContext) error {
    if ctx.OldVersion == "1.0.0" && ctx.NewVersion == "2.0.0" {
        // Переименование ключей в KV Store
        val, found, err := ctx.KVGet("old_key")
        if err != nil {
            return err
        }
        if found {
            if err := ctx.KVSet("new_key", val); err != nil {
                return err
            }
            if err := ctx.KVDelete("old_key"); err != nil {
                return err
            }
        }
    }
    return nil
},
```

## MigrateContext

| Поле / Метод | Описание |
|---|---|
| `OldVersion` | Ранее загруженная версия |
| `NewVersion` | Загружаемая версия |
| `KVGet(key)` | Чтение из KV Store |
| `KVSet(key, value)` | Запись в KV Store |
| `KVDelete(key)` | Удаление из KV Store |
| `KVList(prefix)` | Список ключей по префиксу |

## Поведение по умолчанию

Если `Migrate` равен `nil`, обновление версии проходит без ошибок (no-op).

::: warning Обработка ошибок
Если миграция возвращает ошибку, хост логирует предупреждение, но **продолжает перезагрузку**. Новая версия всё равно будет загружена. По возможности делайте миграции идемпотентными.
:::

## Миграция между несколькими версиями

```go
Migrate: func(ctx *wasmplugin.MigrateContext) error {
    switch {
    case ctx.OldVersion < "2.0.0" && ctx.NewVersion >= "2.0.0":
        // v1 -> v2: переименование ключей
        migrateV1toV2(ctx)
    case ctx.OldVersion < "3.0.0" && ctx.NewVersion >= "3.0.0":
        // v2 -> v3: реструктуризация данных
        migrateV2toV3(ctx)
    }
    return nil
},
```

::: tip
Старайтесь делать миграции простыми. Предпочитайте аддитивные изменения (новые ключи) вместо деструктивных (переименование/удаление). Если возможно, поддерживайте чтение как старого, так и нового формата ключей в обработчиках в переходный период.
:::
