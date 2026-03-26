# Миграции

## Зачем нужны миграции

При обновлении плагина формат хранимых данных может измениться: ключи в KV Store переименовываются, структура таблиц в БД меняется, формат значений обновляется. Миграции позволяют автоматически трансформировать данные при переходе между версиями.

SuperBotGo поддерживает два типа миграций:

- **SQL-миграции** - изменение схемы базы данных
- **KV-миграции** - трансформация данных в KV Store через колбэк `Migrate`

## KV-миграции {#kv}

KV-миграции описываются через колбэк `Migrate` в структуре `Plugin`. Хост вызывает его при загрузке плагина, если версия изменилась.

```go
import wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"

wasmplugin.Plugin{
    ID:      "notes",
    Version: "2.0.0",
    Migrate: func(ctx *wasmplugin.MigrateContext) error {
        if ctx.OldVersion == "1.0.0" {
            // v1 хранил заметки как "note:<id>",
            // v2 использует "notes:user:<uid>:<id>"
            keys, err := ctx.KVList("note:")
            if err != nil {
                return err
            }
            for _, key := range keys {
                val, found, err := ctx.KVGet(key)
                if err != nil {
                    return err
                }
                if found {
                    newKey := "notes:user:default:" + key[5:]
                    if err := ctx.KVSet(newKey, val); err != nil {
                        return err
                    }
                    if err := ctx.KVDelete(key); err != nil {
                        return err
                    }
                }
            }
        }
        return nil
    },
    // ...
}
```

## MigrateContext {#migrate-context}

Колбэк `Migrate` получает `*wasmplugin.MigrateContext` со следующими полями и методами:

| Поле / Метод | Тип | Описание |
|---|---|---|
| `OldVersion` | `string` | Версия, которая была загружена ранее |
| `NewVersion` | `string` | Версия, которая загружается сейчас |
| `KVGet(key)` | `(string, bool, error)` | Чтение значения из KV Store |
| `KVSet(key, value)` | `error` | Запись значения в KV Store |
| `KVDelete(key)` | `error` | Удаление ключа из KV Store |
| `KVList(prefix)` | `([]string, error)` | Список ключей по префиксу |

::: tip
При первой установке плагина `OldVersion` будет пустой строкой. Используйте это для инициализации начальных данных, если нужно.
:::

## SQL-миграции {#sql}

Для плагинов, использующих базу данных, миграции схемы описываются структурой `SQLMigration`:

```go
wasmplugin.Plugin{
    ID:      "analytics",
    Version: "2.0.0",
    Migrations: []wasmplugin.SQLMigration{
        {
            Version:     1,
            Description: "create metrics table",
            Up:   "CREATE TABLE IF NOT EXISTS plugin_metrics (id SERIAL PRIMARY KEY, name TEXT NOT NULL, value DOUBLE PRECISION, created_at TIMESTAMPTZ DEFAULT now());",
            Down: "DROP TABLE IF EXISTS plugin_metrics;",
        },
        {
            Version:     2,
            Description: "add labels column",
            Up:   "ALTER TABLE plugin_metrics ADD COLUMN IF NOT EXISTS labels JSONB DEFAULT '{}';",
            Down: "ALTER TABLE plugin_metrics DROP COLUMN IF EXISTS labels;",
        },
    },
    // ...
}
```

| Поле | Тип | Описание |
|---|---|---|
| `Version` | `int` | Порядковый номер миграции |
| `Description` | `string` | Краткое описание изменения |
| `Up` | `string` | SQL для применения миграции |
| `Down` | `string` | SQL для отката миграции |

## SQL-миграции из файлов {#from-fs}

Для крупных плагинов удобнее хранить миграции в отдельных SQL-файлах. Функция `MigrationsFromFS` читает файлы в формате [goose](https://github.com/pressly/goose) из встроенной файловой системы:

```
migrations/
├── 001_create_metrics.sql
├── 002_add_labels.sql
└── 003_add_index.sql
```

Каждый файл использует goose-формат с маркерами `-- +goose Up` и `-- +goose Down`:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS plugin_metrics (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    value DOUBLE PRECISION,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS plugin_metrics;
```

Подключение через `//go:embed`:

```go
package main

import (
    "embed"

    wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
    wasmplugin.Run(wasmplugin.Plugin{
        ID:         "analytics",
        Version:    "2.0.0",
        Migrations: wasmplugin.MigrationsFromFS(migrationsFS, "migrations"),
        // ...
    })
}
```

`MigrationsFromFS` возвращает `[]SQLMigration`, автоматически извлекая версию из имени файла и разделяя секции Up/Down.

## Порядок выполнения {#order}

При обновлении плагина хост выполняет миграции в строгом порядке:

1. **SQL-миграции** - хост применяет все новые записи из `Migrations` (по возрастанию `Version`). Миграции выполняются на стороне хоста, вне WASM-песочницы.
2. **KV-миграция** - хост вызывает колбэк `Migrate` внутри WASM-экземпляра с заполненным `MigrateContext`.

```
Обновление плагина v1.0.0 → v2.0.0
│
├─ 1. SQL: применить миграции с Version > последней применённой
│   ├─ 001_create_metrics.sql  ✓ (уже применена)
│   ├─ 002_add_labels.sql      ← применяется
│   └─ 003_add_index.sql       ← применяется
│
└─ 2. KV: вызвать Migrate(ctx) с OldVersion="1.0.0", NewVersion="2.0.0"
```

::: info
SQL-миграции управляются хостом и выполняются до запуска WASM. Это позволяет KV-миграции полагаться на уже обновлённую схему БД.
:::

## Идемпотентность {#idempotency}

::: warning
Миграции должны быть идемпотентными. Если миграция завершилась с ошибкой, хост логирует предупреждение, но **продолжает загрузку** плагина. При следующей перезагрузке миграция может быть вызвана повторно.
:::

Рекомендации:

- В SQL используйте `IF NOT EXISTS` / `IF EXISTS` для DDL-операций
- В KV-миграциях проверяйте существование ключей перед трансформацией
- Избегайте операций, которые ломаются при повторном выполнении

```go
// Хорошо: идемпотентно
val, found, _ := ctx.KVGet("old_key")
if found {
    ctx.KVSet("new_key", val)
    ctx.KVDelete("old_key")
}

// Плохо: упадёт при повторном вызове, если old_key уже удалён
val, _, _ := ctx.KVGet("old_key") // val пустой
ctx.KVSet("new_key", val)          // перезаписали новый ключ пустым значением
ctx.KVDelete("old_key")
```

## Пример с несколькими версиями {#multi-version}

При поддержке обновления через несколько версий используйте последовательные проверки:

```go
import wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"

wasmplugin.Plugin{
    ID:      "tasks",
    Version: "3.0.0",
    Migrate: func(ctx *wasmplugin.MigrateContext) error {
        // v1 → v2: переименование ключей
        if ctx.OldVersion < "2.0.0" && ctx.NewVersion >= "2.0.0" {
            keys, _ := ctx.KVList("task:")
            for _, key := range keys {
                val, found, _ := ctx.KVGet(key)
                if found {
                    ctx.KVSet("tasks:active:"+key[5:], val)
                    ctx.KVDelete(key)
                }
            }
        }

        // v2 → v3: добавление метаданных
        if ctx.OldVersion < "3.0.0" && ctx.NewVersion >= "3.0.0" {
            keys, _ := ctx.KVList("tasks:active:")
            for _, key := range keys {
                val, found, _ := ctx.KVGet(key)
                if found {
                    // Оборачиваем значение в JSON с метаданными
                    wrapped := `{"data":` + val + `,"version":3}`
                    ctx.KVSet(key, wrapped)
                }
            }
        }

        return nil
    },
    Migrations: []wasmplugin.SQLMigration{
        {
            Version:     1,
            Description: "create tasks table",
            Up:   "CREATE TABLE IF NOT EXISTS plugin_tasks (id SERIAL PRIMARY KEY, title TEXT NOT NULL);",
            Down: "DROP TABLE IF EXISTS plugin_tasks;",
        },
        {
            Version:     2,
            Description: "add status column",
            Up:   "ALTER TABLE plugin_tasks ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'open';",
            Down: "ALTER TABLE plugin_tasks DROP COLUMN IF EXISTS status;",
        },
        {
            Version:     3,
            Description: "add metadata column",
            Up:   "ALTER TABLE plugin_tasks ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';",
            Down: "ALTER TABLE plugin_tasks DROP COLUMN IF EXISTS metadata;",
        },
    },
    // ...
}
```

Такая структура позволяет корректно обновиться с любой предыдущей версии: плагин, обновляющийся с v1 до v3, последовательно выполнит обе KV-миграции, а хост применит все три SQL-миграции.

::: tip
Предпочитайте аддитивные изменения (новые ключи, новые колонки) вместо деструктивных (переименование, удаление). Если возможно, поддерживайте чтение как старого, так и нового формата в обработчиках в переходный период.
:::

## Что дальше?

- [Сборка и установка](/deploy/build) - компиляция, разрешения и ограничения среды
- [KV Store](/api/kv-store) - работа с KV Store в обработчиках
- [Структура плагина](/guide/plugin-structure) - все поля структуры `Plugin`
